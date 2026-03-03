package dto

// SessionResponse represents a single active session for the authenticated user.
type SessionResponse struct {
	ID         string `json:"id"`
	IPAddress  string `json:"ip_address"`
	UserAgent  string `json:"user_agent"`
	CreatedAt  string `json:"created_at"`
	LastActive string `json:"last_active"`
	IsCurrent  bool   `json:"is_current"`
}

// SessionListResponse wraps the list of active sessions.
type SessionListResponse struct {
	Sessions []SessionResponse `json:"sessions"`
}
