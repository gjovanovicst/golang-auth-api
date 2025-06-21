## Phase 1: Database Model Design and GORM Setup

This phase focuses on defining the database schemas for user management and social authentication, along with setting up GORM for object-relational mapping and database migrations. A well-designed database schema is fundamental for a robust and scalable authentication system. GORM, as a powerful ORM for Go, simplifies database interactions and provides migration capabilities to manage schema changes effectively.

### 1.1 User Model

The `User` model will represent the core user entity in our system. It will store essential user information required for authentication and identification. The following fields are proposed for the `User` model:

| Field Name      | Data Type       | Description                                       | Constraints/Notes                                |
|-----------------|-----------------|---------------------------------------------------|--------------------------------------------------|
| `ID`            | `UUID`          | Unique identifier for the user.                   | Primary Key, Auto-generated, Indexed             |
| `Email`         | `string`        | User's email address.                             | Unique, Indexed, Required                        |
| `PasswordHash`  | `string`        | Hashed password for secure storage.               | Required for traditional login                   |
| `EmailVerified` | `boolean`       | Indicates if the user's email has been verified.  | Default to `false`                               |
| `CreatedAt`     | `time.Time`     | Timestamp of user creation.                       | Auto-generated                                   |
| `UpdatedAt`     | `time.Time`     | Timestamp of last update.                         | Auto-updated                                     |

**GORM Model Definition (GoLang):**

```go
type User struct {
    ID            uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
    Email         string    `gorm:"uniqueIndex;not null" json:"email"`
    PasswordHash  string    `gorm:"not null" json:"-"` // Stored hashed, not exposed via JSON
    EmailVerified bool      `gorm:"default:false" json:"email_verified"`
    CreatedAt     time.Time `gorm:"autoCreateTime" json:"created_at"`
    UpdatedAt     time.Time `gorm:"autoUpdateTime" json:"updated_at"`
    SocialAccounts []SocialAccount `gorm:"foreignKey:UserID" json:"social_accounts"` // One-to-many relationship
}
```

### 1.2 SocialAccount Model

The `SocialAccount` model will store information related to a user's social media logins (e.g., Google, Facebook, GitHub). This model will link social profiles to our internal `User` accounts, allowing users to sign in using their preferred social providers.

| Field Name         | Data Type       | Description                                       | Constraints/Notes                                |
|--------------------|-----------------|---------------------------------------------------|--------------------------------------------------|
| `ID`               | `UUID`          | Unique identifier for the social account.         | Primary Key, Auto-generated, Indexed             |
| `UserID`           | `UUID`          | Foreign key linking to the `User` model.          | Required, Indexed                                |
| `Provider`         | `string`        | Name of the social provider (e.g., "google", "facebook", "github"). | Required, Indexed                                |
| `ProviderUserID`   | `string`        | Unique ID of the user from the social provider.   | Unique per provider, Required, Indexed           |
| `AccessToken`      | `string`        | Access token obtained from the social provider.   | Encrypted storage recommended                    |
| `RefreshToken`     | `string`        | Refresh token for renewing access tokens.         | Encrypted storage recommended, Optional          |
| `ExpiresAt`        | `time.Time`     | Expiration time of the access token.              | Optional                                         |
| `CreatedAt`        | `time.Time`     | Timestamp of social account creation.             | Auto-generated                                   |
| `UpdatedAt`        | `time.Time`     | Timestamp of last update.                         | Auto-updated                                     |

**GORM Model Definition (GoLang):**

```go
type SocialAccount struct {
    ID             uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
    UserID         uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
    Provider       string    `gorm:"not null;index" json:"provider"`
    ProviderUserID string    `gorm:"uniqueIndex:idx_provider_user_id;not null" json:"provider_user_id"` // Composite unique index with Provider
    AccessToken    string    `json:"-"` // Stored encrypted, not exposed via JSON
    RefreshToken   string    `json:"-"` // Stored encrypted, not exposed via JSON
    ExpiresAt      *time.Time `json:"expires_at"`
    CreatedAt      time.Time `gorm:"autoCreateTime" json:"created_at"`
    UpdatedAt      time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}
```

### 1.3 Database Relationships

There will be a one-to-many relationship between the `User` and `SocialAccount` models. A single `User` can have multiple `SocialAccount` entries (e.g., one user can link their Google and Facebook accounts), but each `SocialAccount` belongs to only one `User`.

- `User` has many `SocialAccount`s.
- `SocialAccount` belongs to one `User`.

This relationship is defined in the `User` struct with `SocialAccounts []SocialAccount `gorm:"foreignKey:UserID"` and in the `SocialAccount` struct with `UserID uuid.UUID`.

### 1.4 GORM Setup and Migrations

To manage the database schema, GORM's auto-migration feature will be utilized. This allows for programmatic schema creation and updates based on the defined Go structs. For production environments, more controlled migration tools like `golang-migrate/migrate` are often preferred, but for initial setup and rapid development, GORM's auto-migration is sufficient.

**Database Connection:**

The application will connect to a PostgreSQL database. The connection string will typically be configured via environment variables or a configuration file (e.g., using Viper, which will be discussed in Phase 2).

```go
package database

import (
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func ConnectDatabase() {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Europe/Belgrade",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_PORT"),
	)

	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
		logger.Config{
			SlowThreshold:              time.Second,   // Slow SQL threshold
			LogLevel:                   logger.Info, // Log level
			IgnoreRecordNotFoundError: true,            // Ignore ErrRecordNotFound error for logger
			Colorful:                   true,            // Disable color
		},
	)

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: newLogger,
	})

	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	log.Println("Database connected successfully!")
}

func MigrateDatabase() {
	// AutoMigrate will create tables, missing columns, and missing indexes
	// It will NOT change existing column types or delete unused columns
	err := DB.AutoMigrate(&User{}, &SocialAccount{})
	
	if err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	log.Println("Database migration completed!")
}
```

**Migration Strategy:**

For a more robust migration strategy, especially in production, a dedicated migration tool like `golang-migrate/migrate` is recommended. This tool allows for version-controlled migrations, enabling rollbacks and more granular control over schema changes. However, for the purpose of this plan, GORM's `AutoMigrate` will be used for simplicity during initial development.

**Example `main.go` for Database Initialization:**

```go
package main

import (
	"log"
	"os"

	"github.com/your_username/your_project/database"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on environment variables")
	}

	database.ConnectDatabase()
	database.MigrateDatabase()

	// Your application logic here
}
```

This setup ensures that the database schema is automatically created or updated based on the Go models when the application starts. This completes the first phase of the implementation plan.


## Phase 2: Project Structure and Dependencies

This phase outlines a clear and scalable project structure for the GoLang RESTful API, along with identifying and listing all necessary Go packages. A well-organized project structure is crucial for maintainability, testability, and collaboration, especially as the application grows in complexity. Selecting appropriate dependencies ensures that common functionalities are handled efficiently and securely.

### 2.1 Project Structure

We will adopt a common and recommended project layout for Go applications, which promotes separation of concerns and modularity. This structure is inspired by the standard Go project layout [1].

```
/project-root
├── cmd/
│   └── api/
│       └── main.go               # Main application entry point
├── internal/
│   ├── auth/
│   │   ├── handler.go            # HTTP handlers for authentication
│   │   ├── service.go            # Business logic for authentication
│   │   └── repository.go         # Database interactions for authentication
│   ├── user/
│   │   ├── handler.go            # HTTP handlers for user management
│   │   ├── service.go            # Business logic for user management
│   │   └── repository.go         # Database interactions for user management
│   ├── social/
│   │   ├── handler.go            # HTTP handlers for social auth callbacks
│   │   ├── service.go            # Business logic for social auth
│   │   └── repository.go         # Database interactions for social auth
│   ├── email/
│   │   └── service.go            # Email sending service
│   ├── middleware/
│   │   └── auth.go               # Authentication middleware
│   │   └── rate_limit.go         # Rate limiting middleware
│   ├── config/
│   │   └── config.go             # Application configuration loading
│   ├── database/
│   │   └── db.go                 # Database connection and migration setup
│   └── util/
│       └── helper.go             # Common utility functions (e.g., password hashing)
├── pkg/
│   ├── models/
│   │   ├── user.go               # GORM model for User
│   │   └── social_account.go     # GORM model for SocialAccount
│   ├── dto/
│   │   └── auth.go               # Data Transfer Objects (request/response structs)
│   ├── errors/
│   │   └── errors.go             # Custom error types
│   └── jwt/
│       └── jwt.go                # JWT token handling
├── migrations/
│   └── <timestamp>_create_tables.up.sql  # SQL migration files (if using golang-migrate)
├── vendor/
├── .env                          # Environment variables (for local development)
├── go.mod                        # Go module definition
├── go.sum                        # Go module checksums
├── Dockerfile                    # Docker build instructions
├── README.md                     # Project documentation
```

**Explanation of Directories:**

- **`cmd/`**: Contains the main application entry points. Each subdirectory here represents a distinct executable. `cmd/api/main.go` will be our primary application.
- **`internal/`**: This directory is for private application and library code. It's intended for code that cannot be imported by other applications or libraries outside of this project. This is where our core business logic, handlers, services, and repositories will reside.
    - `auth/`, `user/`, `social/`: These subdirectories represent distinct domains or features within the application, each containing its handlers (HTTP request handling), services (business logic), and repositories (database interactions).
    - `email/`: Service for sending emails.
    - `middleware/`: Custom HTTP middleware functions.
    - `config/`: Handles loading and managing application configurations.
    - `database/`: Database connection and migration logic.
    - `util/`: General utility functions.
- **`pkg/`**: This directory is for library code that's safe to be used by external applications. While our current project might not have external consumers, it's good practice to separate reusable components here.
    - `models/`: GORM struct definitions for database tables.
    - `dto/`: Data Transfer Objects (DTOs) for request and response payloads.
    - `errors/`: Custom error definitions for consistent error handling.
    - `jwt/`: Logic for JWT token creation, parsing, and validation.
- **`migrations/`**: If using a dedicated migration tool like `golang-migrate`, SQL migration files will be stored here.
- **`vendor/`**: Managed by Go modules, this directory stores copies of dependent packages.
- **`.env`**: File for storing environment variables, especially sensitive ones like database credentials and API keys, for local development.
- **`go.mod` and `go.sum`**: These files define the Go module and manage its dependencies.
- **`Dockerfile`**: Instructions for building a Docker image of the application.
- **`README.md`**: Project documentation.

### 2.2 Necessary GoLang Packages

Here's a list of essential Go packages required for building the authentication and authorization API:

| Category           | Package Name           | Description                                                                 | Purpose in Project                                       |
|--------------------|------------------------|-----------------------------------------------------------------------------|----------------------------------------------------------|
| **Web Framework**  | `github.com/gin-gonic/gin` | A high-performance HTTP web framework.                                      | Routing, request handling, middleware management         |
| **ORM**            | `gorm.io/gorm`         | The ORM library for Go.                                                     | Database interactions, model mapping                     |
| **PostgreSQL Driver**| `gorm.io/driver/postgres` | PostgreSQL driver for GORM.                                                 | Connecting to PostgreSQL database                        |
| **UUID Generation**| `github.com/google/uuid` | UUID package for Go.                                                        | Generating unique IDs for users and social accounts      |
| **Password Hashing**| `golang.org/x/crypto/bcrypt` | bcrypt hashing algorithm for secure password storage.                       | Hashing and verifying user passwords                     |
| **JWT**            | `github.com/golang-jwt/jwt/v5` | JSON Web Token (JWT) implementation for Go.                                 | Creating, signing, and validating JWTs                   |
| **Environment Variables**| `github.com/spf13/viper` | A complete configuration solution for Go applications.                      | Loading configuration from `.env`, files, etc.           |
| **Redis Client**   | `github.com/go-redis/redis/v8` | A powerful and feature-rich Redis client for Go.                            | Storing tokens, email verification codes, rate limiting  |
| **OAuth2 Client**  | `golang.org/x/oauth2`  | Go OAuth2 client library.                                                   | Handling OAuth2 flows for Google, Facebook, GitHub       |
| **Google OAuth2**  | `golang.org/x/oauth2/google` | Google-specific OAuth2 endpoints.                                           | Google social login                                      |
| **Facebook OAuth2**| (Custom implementation or a specific library if available) | Facebook-specific OAuth2 endpoints.                                         | Facebook social login                                      |
| **GitHub OAuth2**  | `github.com/google/go-github/github` (for API) | GitHub-specific OAuth2 endpoints. (The `go-github` library is for GitHub API, not directly OAuth2) | GitHub social login                                      |
| **Email Sending**  | `gopkg.in/mail.v2`     | A Go package for sending emails.                                            | Sending verification emails                               |
| **Validation**     | `github.com/go-playground/validator/v10` | Go Struct and Field validation, including Cross Field, Cross Struct, and Field level validations. | Validating request payloads                              |
| **Dotenv**         | `github.com/joho/godotenv` | Loads environment variables from a `.env` file.                             | Convenient local environment setup                       |

### 2.3 Initial Project Setup

To initialize the project, navigate to your desired project directory and execute the following commands:

1.  **Initialize Go Module:**
    ```bash
    go mod init github.com/your_username/your_project_name
    ```
    Replace `github.com/your_username/your_project_name` with your actual module path.

2.  **Create Core Directories:**
    ```bash
    mkdir -p cmd/api internal/auth internal/user internal/social internal/email internal/middleware internal/config internal/database internal/util pkg/models pkg/dto pkg/errors pkg/jwt
    ```

3.  **Install Dependencies:**
    ```bash
    go get github.com/gin-gonic/gin \
           gorm.io/gorm gorm.io/driver/postgres \
           github.com/google/uuid \
           golang.org/x/crypto/bcrypt \
           github.com/golang-jwt/jwt/v5 \
           github.com/spf13/viper \
           github.com/go-redis/redis/v8 \
           golang.org/x/oauth2 golang.org/x/oauth2/google \
           gopkg.in/mail.v2 \
           github.com/go-playground/validator/v10 \
           github.com/joho/godotenv
    ```
    *Note: For Facebook and GitHub OAuth2, specific client libraries might be needed or a direct implementation using `golang.org/x/oauth2` will be used. The `go-github` library is primarily for interacting with the GitHub API, not directly for OAuth2 authentication flow, but can be useful post-authentication.* 

4.  **Create a `.env` file (for local development):**
    ```bash
    touch .env
    ```
    Add placeholder environment variables to `.env`:
    ```
    DB_HOST=localhost
    DB_PORT=5432
    DB_USER=your_db_user
    DB_PASSWORD=your_db_password
    DB_NAME=your_db_name

    JWT_SECRET=supersecretjwtkey
    ACCESS_TOKEN_EXPIRATION_MINUTES=15
    REFRESH_TOKEN_EXPIRATION_HOURS=720 # 30 days

    REDIS_ADDR=localhost:6379
    REDIS_PASSWORD=
    REDIS_DB=0

    GOOGLE_CLIENT_ID=your_google_client_id
    GOOGLE_CLIENT_SECRET=your_google_client_secret
    GOOGLE_REDIRECT_URL=http://localhost:8080/auth/google/callback

    FACEBOOK_CLIENT_ID=your_facebook_client_id
    FACEBOOK_CLIENT_SECRET=your_facebook_client_secret
    FACEBOOK_REDIRECT_URL=http://localhost:8080/auth/facebook/callback

    GITHUB_CLIENT_ID=your_github_client_id
    GITHUB_CLIENT_SECRET=your_github_client_secret
    GITHUB_REDIRECT_URL=http://localhost:8080/auth/github/callback

    EMAIL_HOST=smtp.example.com
    EMAIL_PORT=587
    EMAIL_USERNAME=your_email@example.com
    EMAIL_PASSWORD=your_email_password
    EMAIL_FROM=no-reply@example.com
    ```

This structured approach ensures that the project is set up for efficient development and future scalability. The next phase will delve into the core authentication implementation.

