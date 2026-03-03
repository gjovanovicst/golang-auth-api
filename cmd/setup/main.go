package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"syscall"
	"unicode"

	"github.com/gjovanovicst/auth_api/internal/admin"
	"github.com/gjovanovicst/auth_api/internal/database"
	"github.com/gjovanovicst/auth_api/pkg/models"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"
)

const (
	bcryptCost     = 12
	minPasswordLen = 12
	maxPasswordLen = 128
)

func main() {
	// Parse command-line flags for non-interactive mode
	username := flag.String("username", "", "Admin username")
	password := flag.String("password", "", "Admin password")
	emailFlag := flag.String("email", "", "Admin email (optional)")
	flag.Parse()

	fmt.Println("===========================================")
	fmt.Println("  Auth API - Admin Account Setup")
	fmt.Println("===========================================")
	fmt.Println()

	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on environment variables")
	}

	// Connect to database
	database.ConnectDatabase()
	database.MigrateDatabase()

	// Initialize repository
	repo := admin.NewAccountRepository(database.DB)

	// Check existing admin count
	count, err := repo.Count()
	if err != nil {
		log.Fatalf("Failed to check existing admin accounts: %v", err)
	}

	if count > 0 {
		accounts, err := repo.ListAll()
		if err != nil {
			log.Fatalf("Failed to list existing admin accounts: %v", err)
		}
		fmt.Printf("Found %d existing admin account(s):\n", count)
		for _, acc := range accounts {
			lastLogin := "never"
			if acc.LastLoginAt != nil {
				lastLogin = acc.LastLoginAt.Format("2006-01-02 15:04:05")
			}
			fmt.Printf("  - %s (created: %s, last login: %s)\n",
				acc.Username,
				acc.CreatedAt.Format("2006-01-02 15:04:05"),
				lastLogin,
			)
		}
		fmt.Println()

		if !confirmAction("Do you want to create an additional admin account?") {
			fmt.Println("Setup cancelled.")
			return
		}
	}

	// Get credentials
	var adminUsername, adminPassword, adminEmail string

	if *username != "" && *password != "" {
		// Non-interactive mode
		adminUsername = *username
		adminPassword = *password
		adminEmail = *emailFlag
	} else {
		// Interactive mode
		adminUsername = promptUsername()
		adminPassword = promptPassword()
		adminEmail = promptEmail()
	}

	// Validate email if provided
	if adminEmail != "" {
		if err := validateEmail(adminEmail); err != nil {
			log.Fatalf("Invalid email: %v", err)
		}
	}

	// Validate username
	if err := validateUsername(adminUsername); err != nil {
		log.Fatalf("Invalid username: %v", err)
	}

	// Check if username already exists
	existing, _ := repo.GetByUsername(adminUsername)
	if existing != nil {
		if *username != "" {
			// Non-interactive mode: fail
			log.Fatalf("Username '%s' already exists. Choose a different username.", adminUsername)
		}
		// Interactive mode: ask to overwrite
		if !confirmAction(fmt.Sprintf("Username '%s' already exists. Overwrite the password?", adminUsername)) {
			fmt.Println("Setup cancelled.")
			return
		}
		// Update existing account password
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(adminPassword), bcryptCost)
		if err != nil {
			log.Fatalf("Failed to hash password: %v", err)
		}
		updates := map[string]interface{}{
			"password_hash": string(hashedPassword),
		}
		if adminEmail != "" {
			updates["email"] = adminEmail
		}
		if err := database.DB.Model(existing).Updates(updates).Error; err != nil {
			log.Fatalf("Failed to update admin account: %v", err)
		}
		fmt.Println()
		fmt.Println("===========================================")
		fmt.Printf("  Admin account '%s' updated!\n", adminUsername)
		if adminEmail != "" {
			fmt.Printf("  Email: %s\n", adminEmail)
		}
		fmt.Println("===========================================")
		return
	}

	// Validate password
	if err := validatePassword(adminPassword); err != nil {
		log.Fatalf("Invalid password: %v", err)
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(adminPassword), bcryptCost)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}

	// Create admin account
	account := &models.AdminAccount{
		Username:     adminUsername,
		Email:        adminEmail,
		PasswordHash: string(hashedPassword),
	}

	if err := repo.Create(account); err != nil {
		log.Fatalf("Failed to create admin account: %v", err)
	}

	fmt.Println()
	fmt.Println("===========================================")
	fmt.Printf("  Admin account '%s' created successfully!\n", adminUsername)
	if adminEmail != "" {
		fmt.Printf("  Email: %s\n", adminEmail)
	}
	fmt.Println("  You can now log in at /gui/login")
	fmt.Println("===========================================")
}

// promptUsername asks for a username interactively
func promptUsername() string {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Enter admin username: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			log.Fatalf("Failed to read input: %v", err)
		}
		username := strings.TrimSpace(input)
		if err := validateUsername(username); err != nil {
			fmt.Printf("  Error: %v\n", err)
			continue
		}
		return username
	}
}

// promptPassword asks for a password interactively with masked input
func promptPassword() string {
	for {
		fmt.Print("Enter admin password: ")
		password, err := readPassword()
		if err != nil {
			log.Fatalf("Failed to read password: %v", err)
		}
		fmt.Println() // newline after masked input

		if err := validatePassword(password); err != nil {
			fmt.Printf("  Error: %v\n", err)
			continue
		}

		fmt.Print("Confirm admin password: ")
		confirm, err := readPassword()
		if err != nil {
			log.Fatalf("Failed to read password confirmation: %v", err)
		}
		fmt.Println() // newline after masked input

		if password != confirm {
			fmt.Println("  Error: passwords do not match")
			continue
		}

		return password
	}
}

// readPassword reads a password from the terminal with masked input
func readPassword() (string, error) {
	fd := int(syscall.Stdin) // #nosec G115 -- syscall.Stdin is always a valid fd
	password, err := term.ReadPassword(fd)
	if err != nil {
		return "", fmt.Errorf("failed to read password: %w", err)
	}
	return string(password), nil
}

// confirmAction prompts the user for a yes/no confirmation
func confirmAction(prompt string) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s (y/N): ", prompt)
	input, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	answer := strings.TrimSpace(strings.ToLower(input))
	return answer == "y" || answer == "yes"
}

// validateUsername validates the admin username
func validateUsername(username string) error {
	if len(username) < 3 {
		return fmt.Errorf("username must be at least 3 characters")
	}
	if len(username) > 50 {
		return fmt.Errorf("username must be at most 50 characters")
	}
	for _, c := range username {
		if !unicode.IsLetter(c) && !unicode.IsDigit(c) && c != '_' && c != '-' && c != '.' {
			return fmt.Errorf("username can only contain letters, digits, underscores, hyphens, and dots")
		}
	}
	return nil
}

// validatePassword validates the admin password against security requirements
func validatePassword(password string) error {
	if len(password) < minPasswordLen {
		return fmt.Errorf("password must be at least %d characters", minPasswordLen)
	}
	if len(password) > maxPasswordLen {
		return fmt.Errorf("password must be at most %d characters", maxPasswordLen)
	}

	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, c := range password {
		switch {
		case unicode.IsUpper(c):
			hasUpper = true
		case unicode.IsLower(c):
			hasLower = true
		case unicode.IsDigit(c):
			hasDigit = true
		case unicode.IsPunct(c) || unicode.IsSymbol(c):
			hasSpecial = true
		}
	}

	var missing []string
	if !hasUpper {
		missing = append(missing, "uppercase letter")
	}
	if !hasLower {
		missing = append(missing, "lowercase letter")
	}
	if !hasDigit {
		missing = append(missing, "digit")
	}
	if !hasSpecial {
		missing = append(missing, "special character")
	}

	if len(missing) > 0 {
		return fmt.Errorf("password must contain at least one: %s", strings.Join(missing, ", "))
	}

	return nil
}

// promptEmail asks for an optional email address interactively
func promptEmail() string {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Enter admin email (optional, press Enter to skip): ")
		input, err := reader.ReadString('\n')
		if err != nil {
			log.Fatalf("Failed to read input: %v", err)
		}
		email := strings.TrimSpace(input)
		if email == "" {
			return ""
		}
		if err := validateEmail(email); err != nil {
			fmt.Printf("  Error: %v\n", err)
			continue
		}
		return email
	}
}

// validateEmail validates a basic email format
func validateEmail(email string) error {
	if len(email) < 3 {
		return fmt.Errorf("email must be at least 3 characters")
	}
	if len(email) > 254 {
		return fmt.Errorf("email must be at most 254 characters")
	}
	atIdx := strings.Index(email, "@")
	if atIdx < 1 {
		return fmt.Errorf("email must contain '@' with a local part before it")
	}
	domain := email[atIdx+1:]
	if len(domain) < 3 || !strings.Contains(domain, ".") {
		return fmt.Errorf("email must have a valid domain (e.g. user@example.com)")
	}
	return nil
}
