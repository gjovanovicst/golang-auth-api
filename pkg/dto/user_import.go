package dto

import "time"

// UserExportItem is one row in a user export (CSV or JSON).
// Fields mirror UserDetail minus secrets, plus social_providers.
type UserExportItem struct {
	ID              string    `json:"id"`
	AppID           string    `json:"app_id"`
	Email           string    `json:"email"`
	Name            string    `json:"name"`
	FirstName       string    `json:"first_name"`
	LastName        string    `json:"last_name"`
	Locale          string    `json:"locale"`
	EmailVerified   bool      `json:"email_verified"`
	IsActive        bool      `json:"is_active"`
	TwoFAEnabled    bool      `json:"two_fa_enabled"`
	TwoFAMethod     string    `json:"two_fa_method"`
	SocialProviders string    `json:"social_providers"` // comma-separated, e.g. "google,github"
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// UserExportResponse wraps the JSON export envelope returned by the REST API.
type UserExportResponse struct {
	Data       []UserExportItem `json:"data"`
	Count      int              `json:"count"`
	Truncated  bool             `json:"truncated"`
	ExportedAt string           `json:"exported_at"`
}

// UserExportRequest is bound from query params for the REST export endpoint.
type UserExportRequest struct {
	Format string `form:"format"`
	AppID  string `form:"app_id"`
	Search string `form:"search"`
}

// UserImportRow is one parsed record from an uploaded CSV or JSON file.
type UserImportRow struct {
	Email     string `json:"email"`
	Name      string `json:"name"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Locale    string `json:"locale"`
}

// UserImportRowError describes a parse or validation failure for a single row/record.
type UserImportRowError struct {
	Row   int    `json:"row"`
	Email string `json:"email"`
	Error string `json:"error"`
}

// UserImportResult is the response DTO after a bulk import operation.
type UserImportResult struct {
	Total    int                  `json:"total"`
	Imported int                  `json:"imported"`
	Skipped  int                  `json:"skipped"`
	Errors   []UserImportRowError `json:"errors,omitempty"`
}
