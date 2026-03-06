package dto

// ============================================================================
// Webhook Endpoint DTOs
// ============================================================================

// CreateWebhookRequest is the request body for creating a webhook endpoint.
// One endpoint per (AppID, EventType) pair — the event_type must be unique per app.
type CreateWebhookRequest struct {
	EventType string `json:"event_type" binding:"required" example:"user.registered"`
	URL       string `json:"url" binding:"required,url" example:"https://your-app.com/webhooks/auth"`
}

// WebhookEndpointResponse is the standard response for a webhook endpoint.
// The secret is NOT included here — it is only returned once at creation time.
type WebhookEndpointResponse struct {
	ID        string `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	AppID     string `json:"app_id" example:"00000000-0000-0000-0000-000000000001"`
	EventType string `json:"event_type" example:"user.registered"`
	URL       string `json:"url" example:"https://your-app.com/webhooks/auth"`
	IsActive  bool   `json:"is_active" example:"true"`
	CreatedAt string `json:"created_at" example:"2024-01-01T00:00:00Z"`
	UpdatedAt string `json:"updated_at" example:"2024-01-01T00:00:00Z"`
}

// CreateWebhookResponse is returned only once at creation time and includes the plaintext secret.
// The secret must be saved by the caller — it cannot be retrieved again.
type CreateWebhookResponse struct {
	Endpoint WebhookEndpointResponse `json:"endpoint"`
	Secret   string                  `json:"secret" example:"whsec_abc123..."` // #nosec G101 -- plaintext secret, shown once
}

// WebhookEndpointListResponse is the paginated list response for webhook endpoints.
type WebhookEndpointListResponse struct {
	Endpoints []WebhookEndpointResponse `json:"endpoints"`
	Total     int64                     `json:"total" example:"5"`
	Page      int                       `json:"page" example:"1"`
	PageSize  int                       `json:"page_size" example:"20"`
}

// ToggleWebhookRequest enables or disables a webhook endpoint.
type ToggleWebhookRequest struct {
	IsActive bool `json:"is_active" example:"true"`
}

// ============================================================================
// Webhook Delivery DTOs
// ============================================================================

// WebhookDeliveryResponse is the response for a single delivery attempt.
type WebhookDeliveryResponse struct {
	ID           string  `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	EndpointID   string  `json:"endpoint_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	AppID        string  `json:"app_id" example:"00000000-0000-0000-0000-000000000001"`
	EventType    string  `json:"event_type" example:"user.registered"`
	Attempt      int     `json:"attempt" example:"1"`
	StatusCode   int     `json:"status_code" example:"200"`
	ResponseBody string  `json:"response_body" example:"ok"`
	LatencyMs    int64   `json:"latency_ms" example:"120"`
	Success      bool    `json:"success" example:"true"`
	ErrorMessage string  `json:"error_message,omitempty"`
	NextRetryAt  *string `json:"next_retry_at,omitempty" example:"2024-01-01T00:05:00Z"`
	CreatedAt    string  `json:"created_at" example:"2024-01-01T00:00:00Z"`
}

// WebhookDeliveryListResponse is the paginated list response for delivery logs.
type WebhookDeliveryListResponse struct {
	Deliveries []WebhookDeliveryResponse `json:"deliveries"`
	Total      int64                     `json:"total" example:"100"`
	Page       int                       `json:"page" example:"1"`
	PageSize   int                       `json:"page_size" example:"20"`
}
