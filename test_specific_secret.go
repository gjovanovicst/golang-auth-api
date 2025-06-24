package main

import (
	"fmt"
	"time"

	"github.com/pquerna/otp/totp"
)

func main() {
	// Use the secret from the previous API call
	secret := "RZCH2POUGIOAIDZJ2R2M4E62AIACDYVLF6WLDXG3KHWBCLZQL2ZA===="
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
