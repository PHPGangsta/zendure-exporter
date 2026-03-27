package client

import "fmt"

// ErrUnreachable is returned when a device cannot be contacted at the network
// level — connection refused, DNS failure, or context timeout mid-flight.
type ErrUnreachable struct {
	URL string
	Err error
}

func (e *ErrUnreachable) Error() string {
	return fmt.Sprintf("device unreachable at %s: %v", e.URL, e.Err)
}

// Unwrap allows errors.Is / errors.As to inspect the underlying cause.
func (e *ErrUnreachable) Unwrap() error { return e.Err }

// ErrHTTPError is returned when a device responds with a non-200 HTTP status.
type ErrHTTPError struct {
	URL    string
	Status int
	Body   string
}

func (e *ErrHTTPError) Error() string {
	if e.Body != "" {
		return fmt.Sprintf("HTTP %d from %s: %s", e.Status, e.URL, e.Body)
	}
	return fmt.Sprintf("HTTP %d from %s", e.Status, e.URL)
}

// ErrParse is returned when the device response payload cannot be decoded or
// converted into metrics — malformed JSON or unexpected structure.
type ErrParse struct {
	DeviceID string
	Err      error
}

func (e *ErrParse) Error() string {
	return fmt.Sprintf("device %s: parse error: %v", e.DeviceID, e.Err)
}

// Unwrap allows errors.Is / errors.As to inspect the underlying cause.
func (e *ErrParse) Unwrap() error { return e.Err }
