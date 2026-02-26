package email

import (
	"encoding/json"
	"log"

	"github.com/gjovanovicst/auth_api/pkg/models"
	"github.com/google/uuid"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

// VariableResolver handles the multi-source resolution of email template variables.
// Resolution priority (highest wins):
//  1. Explicit variables passed by the caller
//  2. User profile fields (when userID is provided)
//  3. App/system settings (app_name, frontend_url, etc.)
//  4. Static default values defined on the email type's variable declarations
type VariableResolver struct {
	db *gorm.DB
}

// NewVariableResolver creates a new resolver with database access for user lookups.
func NewVariableResolver(db *gorm.DB) *VariableResolver {
	return &VariableResolver{db: db}
}

// ResolveVariables builds the final variable map by merging values from all sources.
// The resolution pipeline applies values in order of increasing priority:
// static defaults -> settings -> user fields -> explicit vars.
func (r *VariableResolver) ResolveVariables(
	appID uuid.UUID,
	emailTypeCode string,
	toEmail string,
	userID *uuid.UUID,
	explicitVars map[string]string,
) map[string]string {
	resolved := make(map[string]string)

	// Layer 1 (lowest priority): Static default values from email type variable declarations
	r.applyStaticDefaults(resolved, emailTypeCode)

	// Layer 2: App/system settings
	r.applySettingsVars(resolved, appID)

	// Layer 3: User profile fields (if userID provided)
	if userID != nil {
		r.applyUserVars(resolved, *userID)
	}

	// Always ensure user_email is set (from the toEmail parameter)
	if _, ok := resolved[VarUserEmail]; !ok {
		resolved[VarUserEmail] = toEmail
	}

	// Layer 4 (highest priority): Explicit caller-passed variables
	for k, v := range explicitVars {
		if v != "" {
			resolved[k] = v
		}
	}

	return resolved
}

// applyStaticDefaults reads the email type's variable definitions from the database
// and applies any non-empty DefaultValue as the lowest-priority fallback.
func (r *VariableResolver) applyStaticDefaults(vars map[string]string, emailTypeCode string) {
	if r.db == nil {
		return
	}

	var emailType models.EmailType
	if err := r.db.Where("code = ?", emailTypeCode).First(&emailType).Error; err != nil {
		// Email type not found in DB, skip static defaults
		return
	}

	if len(emailType.Variables) == 0 {
		return
	}

	var typVars []models.EmailTypeVariable
	if err := json.Unmarshal(emailType.Variables, &typVars); err != nil {
		log.Printf("Warning: failed to parse variables for email type %s: %v", emailTypeCode, err)
		return
	}

	for _, v := range typVars {
		if v.DefaultValue != "" {
			vars[v.Name] = v.DefaultValue
		}
	}
}

// applySettingsVars populates variables that come from app/system settings.
// These are resolved using the same pattern as the existing resolveAppName.
func (r *VariableResolver) applySettingsVars(vars map[string]string, appID uuid.UUID) {
	// app_name: application name from DB -> env -> default
	vars[VarAppName] = r.resolveAppName(appID)

	// frontend_url: from env/config
	frontendURL := viper.GetString("FRONTEND_URL")
	if frontendURL != "" {
		vars[VarFrontendURL] = frontendURL
	}
}

// applyUserVars loads the user by ID and populates user-sourced variables.
func (r *VariableResolver) applyUserVars(vars map[string]string, userID uuid.UUID) {
	if r.db == nil {
		return
	}

	var user models.User
	err := r.db.Select("email, name, first_name, last_name, locale, profile_picture").
		First(&user, "id = ?", userID).Error
	if err != nil {
		log.Printf("Warning: failed to load user %s for email variable resolution: %v", userID, err)
		return
	}

	// Only set non-empty fields so we don't overwrite static defaults with blanks
	if user.Email != "" {
		vars[VarUserEmail] = user.Email
	}
	if user.Name != "" {
		vars[VarUserName] = user.Name
	}
	if user.FirstName != "" {
		vars[VarFirstName] = user.FirstName
	}
	if user.LastName != "" {
		vars[VarLastName] = user.LastName
	}
	if user.Locale != "" {
		vars[VarLocale] = user.Locale
	}
	if user.ProfilePicture != "" {
		vars[VarProfilePicture] = user.ProfilePicture
	}
}

// resolveAppName determines the application name for use in email templates.
// Resolution order: DB application record -> APP_NAME env/config -> "Auth API" default.
func (r *VariableResolver) resolveAppName(appID uuid.UUID) string {
	if r.db != nil {
		var app models.Application
		if err := r.db.Select("name").First(&app, "id = ?", appID).Error; err == nil && app.Name != "" {
			return app.Name
		}
	}
	appName := viper.GetString("APP_NAME")
	if appName == "" {
		appName = "Auth API"
	}
	return appName
}
