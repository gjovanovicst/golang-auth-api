package email

import (
	"crypto/tls"
	"fmt"
	"log"

	"github.com/spf13/viper"
	"gopkg.in/mail.v2"
)

// Sender handles the low-level SMTP email sending.
type Sender struct{}

// NewSender creates a new Sender.
func NewSender() *Sender {
	return &Sender{}
}

// Send sends an email using the provided SMTP configuration.
// If htmlBody is provided, it sends a multipart email (HTML + text fallback).
// If only textBody is provided, it sends a plain text email.
func (s *Sender) Send(config SMTPConfig, to, subject, htmlBody, textBody string) error {
	// Check if we're in development mode (no real SMTP configured)
	if config.Host == "" || config.Host == "smtp.example.com" {
		s.logDevEmail(to, config.FromAddress, subject, textBody, htmlBody)
		return nil
	}

	m := mail.NewMessage()

	// Set From header with optional display name
	if config.FromName != "" {
		m.SetAddressHeader("From", config.FromAddress, config.FromName)
	} else {
		m.SetHeader("From", config.FromAddress)
	}

	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)

	// Set body based on available content
	if htmlBody != "" && textBody != "" {
		// Multipart: HTML primary with text fallback
		m.SetBody("text/plain", textBody)
		m.AddAlternative("text/html", htmlBody)
	} else if htmlBody != "" {
		m.SetBody("text/html", htmlBody)
	} else if textBody != "" {
		m.SetBody("text/plain", textBody)
	} else {
		return fmt.Errorf("email must have either HTML or text body")
	}

	d := mail.NewDialer(config.Host, config.Port, config.Username, config.Password)

	if config.UseTLS {
		d.TLSConfig = &tls.Config{
			ServerName: config.Host,
			MinVersion: tls.VersionTLS12,
		}
		// Port 465 uses implicit TLS (SSL), other ports use STARTTLS
		if config.Port == 465 {
			d.SSL = true
		} else {
			d.StartTLSPolicy = mail.MandatoryStartTLS
		}
	}

	if err := d.DialAndSend(m); err != nil {
		log.Printf("Failed to send email to %s via %s:%d: %v", to, config.Host, config.Port, err)
		// Fallback: log the email content for debugging
		s.logDevEmail(to, config.FromAddress, subject, textBody, htmlBody)
		log.Printf("Note: Email delivery failed. Check server logs for the email content above.")
		return nil // Don't fail the operation just because email failed
	}

	log.Printf("Email sent successfully to %s (subject: %s)", to, subject)
	return nil
}

// SendTest sends an email and always returns errors instead of swallowing them.
// This is used for "Send Test Email" so the admin sees exactly what went wrong.
func (s *Sender) SendTest(config SMTPConfig, to, subject, htmlBody, textBody string) error {
	if config.Host == "" || config.Host == "smtp.example.com" {
		return fmt.Errorf("SMTP host is not configured (current value: %q). Please set a valid SMTP host", config.Host)
	}
	if config.FromAddress == "" {
		return fmt.Errorf("from address is not configured. Please set a valid sender email address")
	}
	if config.Port == 0 {
		return fmt.Errorf("SMTP port is not configured. Common ports: 587 (STARTTLS), 465 (SSL), 25 (unencrypted)")
	}

	m := mail.NewMessage()

	if config.FromName != "" {
		m.SetAddressHeader("From", config.FromAddress, config.FromName)
	} else {
		m.SetHeader("From", config.FromAddress)
	}

	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)

	if htmlBody != "" && textBody != "" {
		m.SetBody("text/plain", textBody)
		m.AddAlternative("text/html", htmlBody)
	} else if htmlBody != "" {
		m.SetBody("text/html", htmlBody)
	} else if textBody != "" {
		m.SetBody("text/plain", textBody)
	} else {
		return fmt.Errorf("email must have either HTML or text body")
	}

	d := mail.NewDialer(config.Host, config.Port, config.Username, config.Password)

	if config.UseTLS {
		d.TLSConfig = &tls.Config{
			ServerName: config.Host,
			MinVersion: tls.VersionTLS12,
		}
		if config.Port == 465 {
			d.SSL = true
		} else {
			d.StartTLSPolicy = mail.MandatoryStartTLS
		}
	}

	if err := d.DialAndSend(m); err != nil {
		return fmt.Errorf("SMTP error (%s:%d): %w", config.Host, config.Port, err)
	}

	log.Printf("Test email sent successfully to %s via %s:%d", to, config.Host, config.Port)
	return nil
}

// logDevEmail logs email content to stdout for development/debugging.
func (s *Sender) logDevEmail(to, from, subject, textBody, htmlBody string) {
	log.Printf("=== EMAIL (DEVELOPMENT/FALLBACK MODE) ===")
	log.Printf("To: %s", to)
	log.Printf("From: %s", from)
	log.Printf("Subject: %s", subject)
	if textBody != "" {
		log.Printf("Body (text): %s", textBody)
	}
	if htmlBody != "" {
		log.Printf("Body (html): [HTML content, %d bytes]", len(htmlBody))
	}
	log.Printf("=== EMAIL END ===")
}

// ResolveGlobalSMTPConfig builds an SMTPConfig from global system settings / .env.
func ResolveGlobalSMTPConfig() SMTPConfig {
	return SMTPConfig{
		Host:        viper.GetString("EMAIL_HOST"),
		Port:        viper.GetInt("EMAIL_PORT"),
		Username:    viper.GetString("EMAIL_USERNAME"),
		Password:    viper.GetString("EMAIL_PASSWORD"),
		FromAddress: viper.GetString("EMAIL_FROM"),
		FromName:    viper.GetString("EMAIL_FROM_NAME"),
		UseTLS:      viper.GetBool("EMAIL_USE_TLS"),
	}
}
