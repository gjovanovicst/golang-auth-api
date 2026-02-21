package email

import (
	"fmt"
	"log"

	"github.com/spf13/viper"
	"gopkg.in/mail.v2"
)

type Service struct {
	// Configuration for SMTP server
}

func NewService() *Service {
	return &Service{}
}

func (s *Service) SendVerificationEmail(toEmail, token string) error {
	from := viper.GetString("EMAIL_FROM")
	subject := "Verify Your Email Address"
	body := fmt.Sprintf("Please verify your email address by clicking on the link: http://localhost:8080/verify-email?token=%s", token)

	// Check if we're in development mode (no real SMTP configured)
	emailHost := viper.GetString("EMAIL_HOST")

	if emailHost == "" || emailHost == "smtp.example.com" {
		// Development mode - log email instead of sending
		log.Printf("=== EMAIL VERIFICATION (DEVELOPMENT MODE) ===")
		log.Printf("To: %s", toEmail)
		log.Printf("From: %s", from)
		log.Printf("Subject: %s", subject)
		log.Printf("Body: %s", body)
		log.Printf("=== Verification link: http://localhost:8080/verify-email?token=%s ===", token)
		log.Printf("=== EMAIL END ===")
		return nil
	}

	// Production mode - send actual email
	m := mail.NewMessage()
	m.SetHeader("From", from)
	m.SetHeader("To", toEmail)
	m.SetHeader("Subject", subject)
	m.SetBody("text/plain", body)

	d := mail.NewDialer(
		emailHost,
		viper.GetInt("EMAIL_PORT"),
		viper.GetString("EMAIL_USERNAME"),
		viper.GetString("EMAIL_PASSWORD"),
	)

	if err := d.DialAndSend(m); err != nil {
		log.Printf("Failed to send verification email to %s: %v", toEmail, err)
		// Fallback to development mode - log the email instead of failing
		log.Printf("=== EMAIL VERIFICATION (FALLBACK - SMTP FAILED) ===")
		log.Printf("To: %s", toEmail)
		log.Printf("From: %s", from)
		log.Printf("Subject: %s", subject)
		log.Printf("Body: %s", body)
		log.Printf("=== Verification link: http://localhost:8080/verify-email?token=%s ===", token)
		log.Printf("=== EMAIL END ===")
		log.Printf("Note: Check server logs for the verification link above since email delivery failed")
		return nil // Don't fail registration just because email failed
	}
	log.Printf("Verification email sent to %s", toEmail)
	return nil
}

func (s *Service) SendPasswordResetEmail(toEmail, resetLink string) error {
	from := viper.GetString("EMAIL_FROM")
	subject := "Password Reset Request"
	body := fmt.Sprintf("You requested a password reset. Please click on the link to reset your password: %s", resetLink)

	// Check if we're in development mode (no real SMTP configured)
	emailHost := viper.GetString("EMAIL_HOST")
	if emailHost == "" || emailHost == "smtp.example.com" {
		// Development mode - log email instead of sending
		log.Printf("=== PASSWORD RESET EMAIL (DEVELOPMENT MODE) ===")
		log.Printf("To: %s", toEmail)
		log.Printf("From: %s", from)
		log.Printf("Subject: %s", subject)
		log.Printf("Body: %s", body)
		log.Printf("=== Reset link: %s ===", resetLink)
		log.Printf("=== EMAIL END ===")
		return nil
	}

	// Production mode - send actual email
	m := mail.NewMessage()
	m.SetHeader("From", from)
	m.SetHeader("To", toEmail)
	m.SetHeader("Subject", subject)
	m.SetBody("text/plain", body)

	d := mail.NewDialer(
		emailHost,
		viper.GetInt("EMAIL_PORT"),
		viper.GetString("EMAIL_USERNAME"),
		viper.GetString("EMAIL_PASSWORD"),
	)

	if err := d.DialAndSend(m); err != nil {
		log.Printf("Failed to send password reset email to %s: %v", toEmail, err)
		// Fallback to development mode - log the email instead of failing
		log.Printf("=== PASSWORD RESET EMAIL (FALLBACK - SMTP FAILED) ===")
		log.Printf("To: %s", toEmail)
		log.Printf("From: %s", from)
		log.Printf("Subject: %s", subject)
		log.Printf("Body: %s", body)
		log.Printf("=== Reset link: %s ===", resetLink)
		log.Printf("=== EMAIL END ===")
		log.Printf("Note: Check server logs for the reset link above since email delivery failed")
		return nil // Don't fail the operation just because email failed
	}
	log.Printf("Password reset email sent to %s", toEmail)
	return nil
}
