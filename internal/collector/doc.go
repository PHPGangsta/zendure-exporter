// Package collector implements a Prometheus collector for Zendure device metrics.
//
// [Collector] implements [prometheus.Collector] and orchestrates parallel metric
// collection from one or more Zendure devices on every Prometheus scrape. It
// depends on the [deviceFetcher] interface (satisfied by *client.Client) for
// HTTP interaction, which keeps the two packages loosely coupled and the
// collector independently testable.
//
// On each [Collector.Collect] call the collector:
//
//  1. Checks per-device circuit-breaker state; skips devices whose backoff
//     window has not yet expired after repeated failures.
//  2. Fans out a goroutine per enabled device and awaits all of them.
//  3. Emits device metrics, per-channel metrics, and battery-pack metrics.
//  4. Emits self-monitoring metrics (scrape duration, success flags, error
//     counters, fetch latency, circuit-breaker state).
//
// A scrape_id attribute is attached to the scoped logger used by every device
// goroutine so that log lines from a single scrape cycle can be correlated.
//
// [Collector.Ready] returns true after at least one successful device scrape
// and is used by the /ready HTTP endpoint exposed by the main package.
package collector
