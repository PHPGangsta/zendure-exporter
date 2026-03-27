package collector

import "time"

// Circuit-breaker constants. A device is skipped for circuitBreakerBackoff after
// circuitBreakerThreshold consecutive fetch failures. The counter resets after a
// successful fetch or when the backoff window expires.
const (
	circuitBreakerThreshold = 5
	circuitBreakerBackoff   = 60 * time.Second
)

// isCircuitOpen reports whether the circuit breaker for deviceID is open.
// If the backoff window has expired, it resets the consecutive-failure counter
// and closes the circuit so the next call gets a fresh attempt.
// Must be called with c.mu held.
func (c *Collector) isCircuitOpen(deviceID string) bool {
	until, ok := c.circuitOpenUntil[deviceID]
	if !ok || until.IsZero() {
		return false
	}
	if time.Now().Before(until) {
		return true
	}
	// Backoff expired: close circuit and give device a fresh consecutive-failure count.
	c.consecutiveFailures[deviceID] = 0
	delete(c.circuitOpenUntil, deviceID)
	return false
}
