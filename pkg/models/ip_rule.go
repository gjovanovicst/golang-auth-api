package models

import (
	"time"

	"github.com/google/uuid"
)

// IP rule type constants
const (
	IPRuleTypeAllow = "allow"
	IPRuleTypeBlock = "block"
)

// IP rule match type constants
const (
	IPMatchTypeIP      = "ip"      // Exact IP match (e.g., "192.168.1.1")
	IPMatchTypeCIDR    = "cidr"    // CIDR range match (e.g., "10.0.0.0/8")
	IPMatchTypeCountry = "country" // Country code match (e.g., "US") - requires GeoIP
)

// IPRule defines an IP-based access rule for an application.
// Rules can allow or block access based on IP address, CIDR range, or country code.
//
// Evaluation logic:
//   - If any allowlist rules exist for an app, only matching IPs are permitted (whitelist mode).
//   - If only blocklist rules exist, matching IPs are blocked and all others are allowed.
//   - If both exist: allowlist is checked first; if no allowlist match, access is denied.
type IPRule struct {
	ID          uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	AppID       uuid.UUID `gorm:"type:uuid;not null;index:idx_ip_rule_app" json:"app_id"`
	RuleType    string    `gorm:"type:varchar(10);not null" json:"rule_type"`  // "allow" or "block"
	MatchType   string    `gorm:"type:varchar(10);not null" json:"match_type"` // "ip", "cidr", "country"
	Value       string    `gorm:"not null" json:"value"`                       // IP address, CIDR notation, or ISO country code
	Description string    `json:"description"`
	IsActive    bool      `gorm:"default:true" json:"is_active"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName specifies the table name for IPRule
func (IPRule) TableName() string {
	return "ip_rules"
}
