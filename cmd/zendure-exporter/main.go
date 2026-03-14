package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"zendure-exporter/internal/collector"
	"zendure-exporter/internal/config"
)

func main() {
	configPath := flag.String("config", "config.yml", "Path to configuration file")
	checkConfig := flag.Bool("check-config", false, "Validate config and exit")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	if *checkConfig {
		fmt.Println("OK")
		os.Exit(0)
	}

	logger := setupLogger(cfg.Debug)
	logger.Info("starting zendure-exporter",
		"listen_addr", cfg.ListenAddr,
		"listen_port", cfg.ListenPort,
		"devices", len(cfg.Devices),
		"discovery_mode", cfg.DiscoveryMode,
		"debug", cfg.Debug,
		"device_request_timeout_seconds", cfg.DeviceRequestTimeoutSeconds,
	)

	// Log per-device config summary (without secrets).
	for i, dev := range cfg.Devices {
		logger.Info("configured device",
			"index", i,
			"device_id", dev.ID,
			"model", dev.Model,
			"base_url", dev.BaseURL,
			"enabled", dev.Enabled,
		)
	}

	registry := prometheus.NewRegistry()
	registry.MustRegister(prometheus.NewGoCollector())
	registry.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))

	zendureCollector := collector.New(cfg, logger)
	registry.MustRegister(zendureCollector)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "OK")
	})

	addr := fmt.Sprintf("%s:%d", cfg.ListenAddr, cfg.ListenPort)
	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("listening", "addr", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down gracefully")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", "err", err)
		os.Exit(1)
	}

	logger.Info("shutdown complete")
}

func setupLogger(debug bool) *slog.Logger {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	}))
}
