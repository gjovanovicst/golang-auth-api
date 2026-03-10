package sms

// Sender is the provider-agnostic interface for sending SMS messages.
// Implement this interface to add a new SMS provider.
type Sender interface {
	// Send sends an SMS message to the given E.164 phone number.
	// Returns an error if the message could not be sent.
	Send(to, body string) error
}

// SMSMessage represents an outgoing SMS message.
type SMSMessage struct {
	To   string // E.164 format, e.g. "+14155552671"
	Body string // Message body
}

// Provider constants for supported SMS providers.
const (
	ProviderTwilio   = "twilio"
	ProviderDisabled = "" // Empty string = SMS disabled
)
