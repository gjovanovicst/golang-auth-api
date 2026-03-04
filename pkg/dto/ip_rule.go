package dto

import "time"

// --- IP Rule DTOs ---

// IPRuleCreateRequest is the request body for creating a new IP rule
type IPRuleCreateRequest struct {
	RuleType    string `json:"rule_type" validate:"required,oneof=allow block" example:"block"`
	MatchType   string `json:"match_type" validate:"required,oneof=ip cidr country" example:"ip"`
	Value       string `json:"value" validate:"required" example:"192.168.1.1"`
	Description string `json:"description" example:"Block suspicious IP"`
	IsActive    bool   `json:"is_active" example:"true"`
}

// IPRuleUpdateRequest is the request body for updating an IP rule
type IPRuleUpdateRequest struct {
	RuleType    *string `json:"rule_type,omitempty" validate:"omitempty,oneof=allow block" example:"block"`
	MatchType   *string `json:"match_type,omitempty" validate:"omitempty,oneof=ip cidr country" example:"cidr"`
	Value       *string `json:"value,omitempty" example:"10.0.0.0/8"`
	Description *string `json:"description,omitempty" example:"Block entire subnet"`
	IsActive    *bool   `json:"is_active,omitempty" example:"true"`
}

// IPRuleResponse is the response body for an IP rule
type IPRuleResponse struct {
	ID          string    `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	AppID       string    `json:"app_id" example:"00000000-0000-0000-0000-000000000001"`
	RuleType    string    `json:"rule_type" example:"block"`
	MatchType   string    `json:"match_type" example:"ip"`
	Value       string    `json:"value" example:"192.168.1.1"`
	Description string    `json:"description" example:"Block suspicious IP"`
	IsActive    bool      `json:"is_active" example:"true"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// IPRuleListResponse wraps a list of IP rules
type IPRuleListResponse struct {
	Rules []IPRuleResponse `json:"rules"`
	Total int              `json:"total"`
}

// IPAccessCheckRequest is the request for checking IP access
type IPAccessCheckRequest struct {
	IPAddress string `json:"ip_address" validate:"required" example:"203.0.113.50"`
}

// IPAccessCheckResponse is the response for an IP access check
type IPAccessCheckResponse struct {
	Allowed     bool   `json:"allowed" example:"true"`
	Reason      string `json:"reason" example:"not_in_blocklist"`
	Country     string `json:"country,omitempty" example:"US"`
	CountryName string `json:"country_name,omitempty" example:"United States"`
	City        string `json:"city,omitempty" example:"San Francisco"`
}
