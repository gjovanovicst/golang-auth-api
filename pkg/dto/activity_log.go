package dto

// ActivityLogResponse represents a single activity log entry in API responses
type ActivityLogResponse struct {
	ID        string      `json:"id"`
	UserID    string      `json:"user_id"`
	EventType string      `json:"event_type"`
	Timestamp string      `json:"timestamp"`
	IPAddress string      `json:"ip_address"`
	UserAgent string      `json:"user_agent"`
	Details   interface{} `json:"details" swaggertype:"object"`
}

// ActivityLogListRequest represents query parameters for listing activity logs
type ActivityLogListRequest struct {
	Page      int    `form:"page" binding:"omitempty,min=1"`
	Limit     int    `form:"limit" binding:"omitempty,min=1,max=100"`
	EventType string `form:"event_type" binding:"omitempty"`
	StartDate string `form:"start_date" binding:"omitempty"` // Format: 2006-01-02
	EndDate   string `form:"end_date" binding:"omitempty"`   // Format: 2006-01-02
}

// ActivityLogListResponse represents the paginated response for activity logs
type ActivityLogListResponse struct {
	Data       []ActivityLogResponse `json:"data"`
	Pagination PaginationResponse    `json:"pagination"`
}

// PaginationResponse represents pagination metadata
type PaginationResponse struct {
	Page         int   `json:"page"`
	Limit        int   `json:"limit"`
	TotalRecords int64 `json:"total_records"`
	TotalPages   int   `json:"total_pages"`
	HasNext      bool  `json:"has_next"`
	HasPrevious  bool  `json:"has_previous"`
}
