package sms

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// TwilioSender implements Sender using the Twilio REST API.
type TwilioSender struct {
	accountSID string
	authToken  string
	fromNumber string // E.164 format, e.g. "+15005550006"
}

// NewTwilioSender creates a new Twilio SMS sender.
func NewTwilioSender(accountSID, authToken, fromNumber string) *TwilioSender {
	return &TwilioSender{
		accountSID: accountSID,
		authToken:  authToken,
		fromNumber: fromNumber,
	}
}

// Send sends an SMS message via the Twilio Messages API.
func (t *TwilioSender) Send(to, body string) error {
	apiURL := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", t.accountSID)

	formData := url.Values{}
	formData.Set("To", to)
	formData.Set("From", t.fromNumber)
	formData.Set("Body", body)

	req, err := http.NewRequest(http.MethodPost, apiURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return fmt.Errorf("twilio: failed to create request: %w", err)
	}

	req.SetBasicAuth(t.accountSID, t.authToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("twilio: HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		var twilioErr struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}
		if jsonErr := json.Unmarshal(respBody, &twilioErr); jsonErr == nil && twilioErr.Message != "" {
			return fmt.Errorf("twilio: API error %d: %s", twilioErr.Code, twilioErr.Message)
		}
		return fmt.Errorf("twilio: unexpected HTTP status %d", resp.StatusCode)
	}

	return nil
}
