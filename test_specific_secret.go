package main

import (
	"fmt"
	"os"
	"time"

	"github.com/pquerna/otp/totp"
)

func main() {
	// Get the secret from environment variable or use a test default
	secret := os.Getenv("TEST_TOTP_SECRET")
	if secret == "" {
		fmt.Println("Error: TEST_TOTP_SECRET environment variable is required")
		fmt.Println("Please set it with: export TEST_TOTP_SECRET=your_secret_here")
		return
	}

	fmt.Printf("Testing with secret: %s\n", secret)
	fmt.Printf("Secret length: %d\n", len(secret))

	// Generate current TOTP code
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		fmt.Printf("Error generating code: %v\n", err)
		return
	}

	fmt.Printf("Current TOTP code for this secret: %s\n", code)

	// Test validation with the code you tried earlier
	userCode := "779254"
	valid := totp.Validate(userCode, secret)
	fmt.Printf("Validation of user code %s: %t\n", userCode, valid)

	// Test validation with current generated code
	valid = totp.Validate(code, secret)
	fmt.Printf("Validation of current code %s: %t\n", code, valid)
}
