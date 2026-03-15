package bruteforce

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const recaptchaVerifyURL = "https://www.google.com/recaptcha/api/siteverify"

// recaptchaResponse represents the JSON response from Google reCAPTCHA verification API.
type recaptchaResponse struct {
	Success     bool      `json:"success"`
	ChallengeTS time.Time `json:"challenge_ts"`
	Hostname    string    `json:"hostname"`
	ErrorCodes  []string  `json:"error-codes"`
}

// VerifyCaptcha validates a reCAPTCHA token against the Google verification API.
// It uses the secret key from the provided BruteForceConfig (which may be per-app or global).
// The remoteIP parameter is optional and helps Google improve risk analysis.
// Returns nil if verification succeeds, an error otherwise.
func VerifyCaptcha(token, remoteIP string, cfg BruteForceConfig) error {
	if !cfg.CaptchaEnabled {
		return nil // CAPTCHA not enabled, skip verification
	}

	if token == "" {
		return fmt.Errorf("CAPTCHA token is required")
	}

	secretKey := cfg.CaptchaSecretKey
	if secretKey == "" {
		return fmt.Errorf("CAPTCHA secret key not configured")
	}

	// POST to Google reCAPTCHA verification endpoint
	formValues := url.Values{
		"secret":   {secretKey},
		"response": {token},
	}
	if remoteIP != "" {
		formValues.Set("remoteip", remoteIP)
	}

	resp, err := http.PostForm(recaptchaVerifyURL, formValues)
	if err != nil {
		return fmt.Errorf("failed to verify CAPTCHA: %w", err)
	}
	defer resp.Body.Close()

	var result recaptchaResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode CAPTCHA response: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("CAPTCHA verification failed: %v", result.ErrorCodes)
	}

	return nil
}
