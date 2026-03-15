package sms

import (
	"log"

	"github.com/spf13/viper"
)

// NewSenderFromConfig creates the appropriate SMS sender based on the SMS_PROVIDER
// environment variable. Returns nil if SMS is disabled (empty provider).
//
// Supported values for SMS_PROVIDER:
//   - "twilio" — uses Twilio REST API (requires SMS_TWILIO_ACCOUNT_SID,
//     SMS_TWILIO_AUTH_TOKEN, SMS_TWILIO_FROM_NUMBER)
//   - "" (empty) — SMS is disabled; all Send calls will be no-ops
func NewSenderFromConfig() Sender {
	provider := viper.GetString("SMS_PROVIDER")

	switch provider {
	case ProviderTwilio:
		sid := viper.GetString("SMS_TWILIO_ACCOUNT_SID")
		token := viper.GetString("SMS_TWILIO_AUTH_TOKEN")
		from := viper.GetString("SMS_TWILIO_FROM_NUMBER")
		if sid == "" || token == "" || from == "" {
			log.Println("Warning: SMS_PROVIDER=twilio but Twilio credentials are incomplete. SMS will be disabled.")
			return nil
		}
		log.Printf("SMS provider: Twilio (from: %s)", from)
		return NewTwilioSender(sid, token, from)
	case ProviderDisabled:
		log.Println("SMS provider: disabled (SMS_PROVIDER not set)")
		return nil
	default:
		log.Printf("Warning: unknown SMS_PROVIDER=%q. SMS will be disabled.", provider)
		return nil
	}
}
