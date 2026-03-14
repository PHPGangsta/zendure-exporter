#!/usr/bin/env bash
# Smoke test for zendure-exporter.
# Starts the exporter locally with a test config, scrapes /metrics, validates output.
#
# Prerequisites: The binary must be built first:
#   cd zendure-exporter && go build -o zendure-exporter ./cmd/zendure-exporter
#
# Usage:
#   cd zendure-exporter && bash scripts/smoke-test.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

BINARY="$PROJECT_DIR/zendure-exporter"
PORT=19854  # Use a non-standard port to avoid conflicts.
METRICS_URL="http://127.0.0.1:${PORT}/metrics"
HEALTH_URL="http://127.0.0.1:${PORT}/health"

# --- Helpers ---

cleanup() {
    if [ -n "${EXPORTER_PID:-}" ]; then
        kill "$EXPORTER_PID" 2>/dev/null || true
        wait "$EXPORTER_PID" 2>/dev/null || true
    fi
    rm -f "$CONFIG_FILE"
}
trap cleanup EXIT

fail() {
    echo "FAIL: $1" >&2
    exit 1
}

pass() {
    echo "PASS: $1"
}

# --- Preconditions ---

if [ ! -x "$BINARY" ]; then
    echo "Binary not found at $BINARY"
    echo "Build it first: go build -o zendure-exporter ./cmd/zendure-exporter"
    exit 1
fi

# --- Create a test config ---
# Note: The device URL points to a non-existing address on purpose.
# The exporter should still start and serve /metrics (with scrape_success=0).

CONFIG_FILE="$(mktemp)"
cat > "$CONFIG_FILE" <<EOF
listen_addr: 127.0.0.1
listen_port: ${PORT}
device_request_timeout_seconds: 1
devices:
  - id: smoke-test-device
    model: SolarFlow800
    base_url: http://127.0.0.1:1
    enabled: true
EOF

# --- Test 1: --check-config ---

echo "=== Test: --check-config ==="
OUTPUT=$("$BINARY" --config "$CONFIG_FILE" --check-config 2>&1)
if echo "$OUTPUT" | grep -q "OK"; then
    pass "--check-config returned OK"
else
    fail "--check-config did not return OK: $OUTPUT"
fi

# --- Test 2: Start exporter and check health ---

echo "=== Test: Start exporter ==="
"$BINARY" --config "$CONFIG_FILE" &
EXPORTER_PID=$!

# Wait for the exporter to be ready.
for i in $(seq 1 20); do
    if curl -sf "$HEALTH_URL" >/dev/null 2>&1; then
        break
    fi
    sleep 0.25
done

if ! curl -sf "$HEALTH_URL" >/dev/null 2>&1; then
    fail "Exporter health endpoint not reachable after 5s"
fi
pass "Exporter started and /health is OK"

# --- Test 3: Scrape /metrics ---

echo "=== Test: Scrape /metrics ==="
METRICS=$(curl -sf "$METRICS_URL")
if [ -z "$METRICS" ]; then
    fail "/metrics returned empty response"
fi
pass "/metrics returned non-empty response"

# --- Test 4: Validate expected metric names in output ---

echo "=== Test: Validate metric names ==="
EXPECTED_METRICS=(
    "zendure_exporter_scrape_duration_seconds"
    "zendure_exporter_scrape_success"
    "zendure_exporter_upstream_request_errors_total"
)

for metric in "${EXPECTED_METRICS[@]}"; do
    if echo "$METRICS" | grep -q "$metric"; then
        pass "Found metric: $metric"
    else
        fail "Missing metric: $metric"
    fi
done

# --- Test 5: Validate device label ---

echo "=== Test: Validate device labels ==="
if echo "$METRICS" | grep -q 'device_id="smoke-test-device"'; then
    pass "Found device_id label"
else
    fail "Missing device_id label"
fi

# --- Test 6: Validate scrape_success=0 (device unreachable) ---

echo "=== Test: Validate scrape_success=0 for unreachable device ==="
if echo "$METRICS" | grep -q 'zendure_exporter_scrape_success.*0'; then
    pass "scrape_success=0 for unreachable device"
else
    fail "Expected scrape_success=0 for unreachable device"
fi

echo ""
echo "=== All smoke tests passed ==="
