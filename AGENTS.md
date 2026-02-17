# AGENTS.md - Agentic Coding Guidelines

This guide provides comprehensive instructions for AI coding agents working on the Auth API repository.

## Quick Reference

**Language**: Go 1.23+  
**Framework**: Gin for HTTP handling  
**Database**: PostgreSQL with GORM  
**Caching**: Redis  
**Project Type**: Modular REST API (Clean Architecture)

---

## Build, Lint, Test Commands

### Development

```bash
make dev              # Hot reload development server (uses Air)
make run              # Run application once
make setup            # Install dependencies and Air
```

### Testing

```bash
make test             # Run all tests
make test-totp        # Run TOTP test (requires TEST_TOTP_SECRET env var)
```

### Single Test Execution

```bash
go test -v ./internal/user -run TestRegister              # Specific test
go test -v ./internal/user -run TestRegister -count=1     # No caching
go test -v -race ./internal/user                          # Race detector
```

### Code Quality

```bash
make fmt              # Format code (go fmt)
make lint             # Run golangci-lint
make security         # Run gosec + nancy vulnerability scans
```

### Build & Production

```bash
make build            # Build binary for current OS
make build-prod       # Cross-compile for Linux (CGO_ENABLED=0)
```

### Database

```bash
make migrate-up       # Apply pending migrations
make migrate-down     # Rollback last migration
make migrate-status   # Show current schema
make migrate-backup   # Create database backup
```

---

## Code Style Guidelines

### Imports Organization

```go
// Standard library
import (
    "context"
    "fmt"
    "net/http"
)

// Third-party packages
import (
    "github.com/gin-gonic/gin"
    "gorm.io/gorm"
)

// Internal packages
import (
    "github.com/gjovanovicst/auth_api/internal/user"
    "github.com/gjovanovicst/auth_api/pkg/dto"
)
```

### Formatting & Naming

- **Functions**: CamelCase starting with verb (`NewService`, `GetByID`)
- **Constants**: UPPER_SNAKE_CASE for constants
- **Private functions**: camelCase prefix (e.g., `validateEmail`)
- **Exported types**: PascalCase (e.g., `UserService`)
- **File names**: snake_case (e.g., `handler.go`, `repository.go`)

### Type System

- Use **interfaces for abstraction** in service/repository layers
- Define domain models in `pkg/models/` with GORM tags
- Use **DTOs in `pkg/dto/`** for API request/response contracts
- All struct fields must have JSON tags for API endpoints

```go
type User struct {
    ID        uint      `gorm:"primaryKey" json:"id"`
    Email     string    `gorm:"unique;not null" json:"email"`
    Password  string    `gorm:"not null" json:"-"`  // Hidden from responses
    CreatedAt time.Time `json:"created_at"`
}
```

### Error Handling

**Never expose raw database errors to clients.** Follow the pattern:

```go
if err != nil {
    // Log internal error if needed
    c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
        Error: "An error occurred",  // Generic message
    })
    return
}
```

Use custom error types from `pkg/errors/`:

```go
if err != nil {
    appErr := errors.NewAppError(errors.ErrConflict, "Email already registered")
    c.JSON(appErr.Code, gin.H{"error": appErr.Message})
    return
}
```

### Dependency Injection

All services use constructor functions:

```go
// Repository layer
func NewRepository(db *gorm.DB) Repository {
    return &repository{db: db}
}

// Service layer
func NewService(repo Repository, cache Cache) Service {
    return &service{repo: repo, cache: cache}
}

// Handler layer
func NewHandler(service Service) *Handler {
    return &Handler{Service: service}
}
```

### Validation

Use `go-playground/validator` struct tags:

```go
type LoginRequest struct {
    Email    string `json:"email" validate:"required,email"`
    Password string `json:"password" validate:"required,min=8,max=128"`
}

validate := validator.New()
if err := validate.Struct(req); err != nil {
    c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
    return
}
```

### Database Queries

```go
// Always parameterize queries (GORM handles this automatically)
var user models.User
if err := r.db.Where("email = ?", email).First(&user).Error; err != nil {
    if errors.Is(err, gorm.ErrRecordNotFound) {
        return nil, nil  // Not found, no error
    }
    return nil, err
}
```

### Swagger Documentation

Always document HTTP handlers with Swagger annotations:

```go
// @Summary User login
// @Description Authenticate user with email and password
// @Tags authentication
// @Accept json
// @Produce json
// @Param request body dto.LoginRequest true "Login credentials"
// @Success 200 {object} dto.LoginResponse
// @Failure 400 {object} dto.ErrorResponse
// @Router /login [post]
func (h *Handler) Login(c *gin.Context) {
    // Implementation
}
```

After changes, regenerate docs:

```bash
make swag-init
```

### Logging & Activity Tracking

Use the log service for audit trails:

```go
logService.CreateLog(ctx, "USER_LOGIN", "User logged in successfully", 
    userID, "INFO")
```

---

## Architecture Patterns

### Repository-Service-Handler Layer

- **Repository**: Data access, database queries
- **Service**: Business logic, validation, orchestration
- **Handler**: HTTP transport, request binding, response formatting

### Directory Structure

```
internal/
├── auth/          # Authentication domain
├── user/          # User management domain
├── social/        # OAuth2 providers
├── twofa/         # Two-factor authentication
├── log/           # Activity logging
├── middleware/    # HTTP middleware (auth, CORS, etc.)
├── database/      # Database connection & migrations
├── redis/         # Redis session management
└── config/        # Configuration loading
pkg/
├── models/        # GORM database models
├── dto/           # API request/response DTOs
├── jwt/           # JWT utilities
└── errors/        # Custom error types
```

---

## Security Patterns (from SECURITY.md)

### Authentication

- **JWT tokens**: 15-minute access, 720-hour refresh (configurable)
- **Token blacklisting**: Use Redis for logout functionality
- **Password hashing**: Use bcrypt (never store plaintext)
- **2FA/TOTP**: Support for authenticator apps + recovery codes

### Authorization

- JWT validation middleware in `internal/middleware/auth.go`
- Extract user context: `c.Get("user_id")` after auth middleware
- Admin routes prepared for role-based access control

### Input Validation

- Always validate DTOs with struct tags + validator
- Sanitize database inputs (GORM prevents SQL injection)
- Use parameterized queries only (GORM handles automatically)

### Security Scanning

Before committing:

```bash
make security      # Run gosec + nancy
```

Configuration in `.gosec.json` with Medium severity threshold.

---

## Commit Message Guidelines (from Cursor Rules)

### Format

```
<type>(<scope>): <description>

[optional body]
[optional footer]
```

### Types

- `feat`: New feature
- `fix`: Bug fix
- `security`: Security patch (especially important for auth APIs)
- `docs`: Documentation
- `refactor`: Code restructuring (no logic changes)
- `test`: Adding/updating tests
- `chore`: Build, dependencies

### Scopes

`auth`, `user`, `social`, `twofa`, `email`, `middleware`, `database`, `redis`, `log`, `api`, `models`, `dto`, `jwt`

### Examples

```
feat(auth): add JWT token blacklisting for logout
fix(middleware): prevent auth bypass with malformed tokens
security(password): enforce bcrypt cost >= 12
test(user): add registration validation tests
docs(api): update Swagger for 2FA endpoints
```

---

## Development Workflow

### Before Starting

1. Check `.env` is configured with database/Redis credentials
2. Run `make docker-dev` to start PostgreSQL and Redis (optional, or use local services)
3. Run `make setup` to install Air and dependencies

### During Development

- Use `make dev` for hot-reload server
- Write tests alongside implementation
- Run `make fmt` and `make lint` before committing
- Document HTTP endpoints with Swagger tags

### Before Committing

1. `make test` - Ensure all tests pass
2. `make fmt` - Format code
3. `make lint` - Check linting rules
4. `make security` - Run security scans
5. `make swag-init` - Update Swagger docs (if API changed)
6. Follow commit message format above

---

## Key Files Reference

- **Entry point**: `cmd/api/main.go` (dependency injection, route setup)
- **Database models**: `pkg/models/`
- **DTOs**: `pkg/dto/`
- **Error types**: `pkg/errors/errors.go`
- **JWT utilities**: `pkg/jwt/jwt.go`
- **Configuration**: Viper-based in `cmd/api/main.go` with `.env` file
- **Swagger docs**: Auto-generated in `docs/` directory

