package dto

import (
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

// ---------------------------------------------------------------------------
// RegisterRequest tests
// ---------------------------------------------------------------------------

func TestRegisterRequest_Valid(t *testing.T) {
	req := RegisterRequest{
		Email:    "user@example.com",
		Password: "validpass8",
	}
	if err := validate.Struct(req); err != nil {
		t.Errorf("expected valid, got error: %v", err)
	}
}

func TestRegisterRequest_MissingEmail(t *testing.T) {
	req := RegisterRequest{Password: "validpass8"}
	if err := validate.Struct(req); err == nil {
		t.Error("expected validation error for missing email")
	}
}

func TestRegisterRequest_InvalidEmail(t *testing.T) {
	req := RegisterRequest{Email: "not-an-email", Password: "validpass8"}
	if err := validate.Struct(req); err == nil {
		t.Error("expected validation error for invalid email")
	}
}

func TestRegisterRequest_PasswordTooShort(t *testing.T) {
	req := RegisterRequest{Email: "user@example.com", Password: "short"}
	if err := validate.Struct(req); err == nil {
		t.Error("expected validation error for short password")
	}
}

func TestRegisterRequest_PasswordTooLong(t *testing.T) {
	req := RegisterRequest{
		Email:    "user@example.com",
		Password: strings.Repeat("a", 129), // 129 > max=128
	}
	if err := validate.Struct(req); err == nil {
		t.Error("expected validation error for password > 128 chars")
	}
}

func TestRegisterRequest_PasswordExactly128(t *testing.T) {
	req := RegisterRequest{
		Email:    "user@example.com",
		Password: strings.Repeat("a", 128),
	}
	if err := validate.Struct(req); err != nil {
		t.Errorf("password of exactly 128 chars should be valid, got error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// LoginRequest tests
// ---------------------------------------------------------------------------

func TestLoginRequest_Valid(t *testing.T) {
	req := LoginRequest{Email: "user@example.com", Password: "pass"}
	if err := validate.Struct(req); err != nil {
		t.Errorf("expected valid, got error: %v", err)
	}
}

func TestLoginRequest_PasswordTooLong(t *testing.T) {
	req := LoginRequest{
		Email:    "user@example.com",
		Password: strings.Repeat("b", 129),
	}
	if err := validate.Struct(req); err == nil {
		t.Error("expected validation error for login password > 128 chars")
	}
}

// ---------------------------------------------------------------------------
// ResetPasswordRequest tests
// ---------------------------------------------------------------------------

func TestResetPasswordRequest_Valid(t *testing.T) {
	req := ResetPasswordRequest{Token: "some-token", NewPassword: "newpass88"}
	if err := validate.Struct(req); err != nil {
		t.Errorf("expected valid, got error: %v", err)
	}
}

func TestResetPasswordRequest_NewPasswordTooShort(t *testing.T) {
	req := ResetPasswordRequest{Token: "tok", NewPassword: "short"}
	if err := validate.Struct(req); err == nil {
		t.Error("expected validation error for short new_password")
	}
}

func TestResetPasswordRequest_NewPasswordTooLong(t *testing.T) {
	req := ResetPasswordRequest{
		Token:       "tok",
		NewPassword: strings.Repeat("c", 129),
	}
	if err := validate.Struct(req); err == nil {
		t.Error("expected validation error for new_password > 128 chars")
	}
}

// ---------------------------------------------------------------------------
// UpdateEmailRequest tests
// ---------------------------------------------------------------------------

func TestUpdateEmailRequest_PasswordTooLong(t *testing.T) {
	req := UpdateEmailRequest{
		Email:    "new@example.com",
		Password: strings.Repeat("d", 129),
	}
	if err := validate.Struct(req); err == nil {
		t.Error("expected validation error for update email password > 128 chars")
	}
}

// ---------------------------------------------------------------------------
// UpdatePasswordRequest tests
// ---------------------------------------------------------------------------

func TestUpdatePasswordRequest_Valid(t *testing.T) {
	req := UpdatePasswordRequest{
		CurrentPassword: "oldpass123",
		NewPassword:     "newpass88",
	}
	if err := validate.Struct(req); err != nil {
		t.Errorf("expected valid, got error: %v", err)
	}
}

func TestUpdatePasswordRequest_CurrentTooLong(t *testing.T) {
	req := UpdatePasswordRequest{
		CurrentPassword: strings.Repeat("e", 129),
		NewPassword:     "newpass88",
	}
	if err := validate.Struct(req); err == nil {
		t.Error("expected validation error for current_password > 128 chars")
	}
}

func TestUpdatePasswordRequest_NewTooLong(t *testing.T) {
	req := UpdatePasswordRequest{
		CurrentPassword: "oldpass123",
		NewPassword:     strings.Repeat("f", 129),
	}
	if err := validate.Struct(req); err == nil {
		t.Error("expected validation error for new_password > 128 chars")
	}
}

// ---------------------------------------------------------------------------
// DeleteAccountRequest tests
// ---------------------------------------------------------------------------

func TestDeleteAccountRequest_PasswordTooLong(t *testing.T) {
	req := DeleteAccountRequest{
		Password:        strings.Repeat("g", 129),
		ConfirmDeletion: true,
	}
	if err := validate.Struct(req); err == nil {
		t.Error("expected validation error for delete password > 128 chars")
	}
}

func TestDeleteAccountRequest_Valid(t *testing.T) {
	req := DeleteAccountRequest{
		Password:        "validpass",
		ConfirmDeletion: true,
	}
	if err := validate.Struct(req); err != nil {
		t.Errorf("expected valid, got error: %v", err)
	}
}
