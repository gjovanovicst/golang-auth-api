package dto

// ComponentStatus represents the health status of a single infrastructure component.
type ComponentStatus struct {
	Status    string `json:"status"`               // "up", "down", or "unconfigured"
	LatencyMs int64  `json:"latency_ms,omitempty"` // Round-trip time in milliseconds (0 when unconfigured)
	Host      string `json:"host,omitempty"`       // Remote address (SMTP only)
	Error     string `json:"error,omitempty"`      // Error message when status is "down"
}

// HealthResponse is returned by GET /health.
type HealthResponse struct {
	Status    string                     `json:"status"` // "healthy", "degraded", or "unhealthy"
	Timestamp string                     `json:"timestamp"`
	Checks    map[string]ComponentStatus `json:"checks"`
}
