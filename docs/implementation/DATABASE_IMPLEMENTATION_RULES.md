# Database Implementation Rules & Guidelines

## Project Setup Rules

### Rule 1: Technology Stack Selection
- **MUST** use PostgreSQL as the primary database (version 15+)
- **MUST** use GORM as the ORM library (`gorm.io/gorm`)
- **MUST** use Redis for caching and session management (version 7+)
- **MUST** use UUID for all primary keys (`github.com/google/uuid`)
- **SHOULD** use Docker for containerized development environment

### Rule 2: Required Dependencies
Add these exact dependencies to your `go.mod`:

```go
require (
    gorm.io/driver/postgres v1.5.9
    gorm.io/gorm v1.25.12
    github.com/go-redis/redis/v8 v8.11.5
    github.com/google/uuid v1.6.0
    github.com/spf13/viper v1.20.1
    github.com/joho/godotenv v1.5.1
    golang.org/x/crypto v0.39.0
)
```

## Project Structure Rules

### Rule 3: Directory Organization
**MUST** follow this exact structure:

```
/project-root
├── cmd/api/main.go
├── internal/
│   ├── database/db.go
│   ├── redis/redis.go
│   ├── user/
│   │   ├── repository.go
│   │   ├── service.go
│   │   └── handler.go
│   ├── [domain]/
│   │   ├── repository.go
│   │   ├── service.go
│   │   └── handler.go
│   └── middleware/
├── pkg/
│   └── models/
│       ├── user.go
│       └── [model].go
├── docker-compose.yml
└── .env
```

### Rule 4: File Naming Conventions
- **MUST** use snake_case for database table names
- **MUST** use PascalCase for Go struct names
- **MUST** name repository files as `repository.go`
- **MUST** name model files after the entity (e.g., `user.go`, `social_account.go`)

## Database Configuration Rules

### Rule 5: Environment Variables
**MUST** define these exact environment variables:

```bash
# Database Configuration
DB_HOST=localhost
DB_USER=postgres
DB_PASSWORD=your_password
DB_NAME=your_db_name
DB_PORT=5432

# Redis Configuration
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=
REDIS_DB=0
```

### Rule 6: Database Connection Implementation
**MUST** implement database connection in `internal/database/db.go`:

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
    dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC",
        os.Getenv("DB_HOST"),
        os.Getenv("DB_USER"),
        os.Getenv("DB_PASSWORD"),
        os.Getenv("DB_NAME"),
        os.Getenv("DB_PORT"),
    )

    newLogger := logger.New(
        log.New(os.Stdout, "\r\n", log.LstdFlags),
        logger.Config{
            SlowThreshold:             time.Second,
            LogLevel:                  logger.Info,
            IgnoreRecordNotFoundError: true,
            Colorful:                  true,
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
```

### Rule 7: Redis Connection Implementation
**MUST** implement Redis connection in `internal/redis/redis.go`:

```go
package redis

import (
    "context"
    "log"
    "github.com/go-redis/redis/v8"
    "github.com/spf13/viper"
)

var Rdb *redis.Client
var ctx = context.Background()

func ConnectRedis() {
    Rdb = redis.NewClient(&redis.Options{
        Addr:     viper.GetString("REDIS_ADDR"),
        Password: viper.GetString("REDIS_PASSWORD"),
        DB:       viper.GetInt("REDIS_DB"),
    })

    _, err := Rdb.Ping(ctx).Result()
    if err != nil {
        log.Fatalf("Could not connect to Redis: %v", err)
    }

    log.Println("Connected to Redis!")
}
```

## Model Design Rules

### Rule 8: Base Model Structure
**MUST** include these fields in every model:

```go
type BaseModel struct {
    ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
    CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
    UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}
```

### Rule 9: User Model Template
**MUST** implement user model with these exact security fields:

```go
type User struct {
    ID                 uuid.UUID       `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
    Email              string          `gorm:"uniqueIndex;not null" json:"email"`
    PasswordHash       string          `gorm:"" json:"-"`
    EmailVerified      bool            `gorm:"default:false" json:"email_verified"`
    TwoFAEnabled       bool            `gorm:"default:false" json:"two_fa_enabled"`
    TwoFASecret        string          `gorm:"" json:"-"`
    TwoFARecoveryCodes datatypes.JSON  `gorm:"type:jsonb" json:"-"`
    CreatedAt          time.Time       `gorm:"autoCreateTime" json:"created_at"`
    UpdatedAt          time.Time       `gorm:"autoUpdateTime" json:"updated_at"`
}
```

### Rule 10: Sensitive Data Protection
- **MUST** use `json:"-"` tag for sensitive fields (passwords, tokens, secrets)
- **MUST** use `gorm:"uniqueIndex"` for unique fields like email
- **MUST** use `datatypes.JSON` for PostgreSQL JSONB fields
- **MUST** use `gorm:"default:value"` for fields with default values

### Rule 11: Foreign Key Relationships
**MUST** define relationships using proper GORM tags:

```go
// One-to-many relationship
SocialAccounts []SocialAccount `gorm:"foreignKey:UserID" json:"social_accounts"`

// Foreign key field
UserID uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
```

### Rule 12: Indexing Strategy
**MUST** add indexes on:
- All foreign key fields: `gorm:"index"`
- Frequently queried fields: `gorm:"index"`
- Unique fields: `gorm:"uniqueIndex"`
- Composite unique indexes: `gorm:"uniqueIndex:idx_name"`

## Repository Pattern Rules

### Rule 13: Repository Structure
**MUST** implement repository using this exact pattern:

```go
package [domain]

import (
    "[project]/pkg/models"
    "gorm.io/gorm"
)

type Repository struct {
    DB *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
    return &Repository{DB: db}
}
```

### Rule 14: Repository Method Naming
**MUST** follow these naming conventions:
- Create operations: `Create[Entity](entity *models.Entity) error`
- Read operations: `Get[Entity]By[Field](field string) (*models.Entity, error)`
- Update operations: `Update[Entity][Field](id, field string) error`
- Delete operations: `Delete[Entity](id string) error`
- List operations: `List[Entity]s(filters) ([]models.Entity, error)`

### Rule 15: Repository Error Handling
**MUST** return GORM errors directly for consistent error handling:

```go
func (r *Repository) GetUserByEmail(email string) (*models.User, error) {
    var user models.User
    err := r.DB.Where("email = ?", email).First(&user).Error
    return &user, err // Return GORM error directly
}
```

## Migration Rules

### Rule 16: Migration Implementation
**MUST** implement migration function in `internal/database/db.go`:

```go
func MigrateDatabase() {
    err := DB.AutoMigrate(
        &models.User{},
        &models.SocialAccount{},
        &models.ActivityLog{},
        // Add all your models here
    )

    if err != nil {
        log.Fatalf("Failed to migrate database: %v", err)
    }

    log.Println("Database migration completed!")
}
```

### Rule 17: Migration Order
**MUST** call migrations in this exact order in `main.go`:

```go
func main() {
    // 1. Load environment variables
    godotenv.Load()
    
    // 2. Connect to database
    database.ConnectDatabase()
    
    // 3. Connect to Redis
    redis.ConnectRedis()
    
    // 4. Run migrations
    database.MigrateDatabase()
    
    // 5. Initialize services
    // ... rest of application
}
```

## Security Rules

### Rule 18: Password Security
- **MUST** hash passwords using bcrypt before storing
- **MUST** never store plain text passwords
- **MUST** use `json:"-"` for password fields
- **MUST** validate password strength before hashing

### Rule 19: Token Security
- **MUST** encrypt social account tokens before storing
- **MUST** use `json:"-"` for all token fields
- **MUST** store refresh tokens in Redis with expiration
- **MUST** implement token revocation mechanism

### Rule 20: Database Security
- **MUST** use environment variables for database credentials
- **MUST** configure SSL mode for production (`sslmode=require`)
- **MUST** use connection pooling (GORM handles this automatically)
- **MUST** implement proper database user permissions

## Performance Rules

### Rule 21: Query Optimization
- **MUST** add indexes on frequently queried fields
- **MUST** use `Select()` to limit returned fields when appropriate
- **MUST** implement pagination for list operations
- **MUST** use `Preload()` for eager loading relationships

### Rule 22: Connection Management
- **MUST** use a single global database connection (`var DB *gorm.DB`)
- **MUST** configure slow query logging (threshold: 1 second)
- **MUST** implement health checks for database availability
- **SHOULD** configure connection pool settings for production

## Docker Rules

### Rule 23: Docker Compose Configuration
**MUST** use this exact Docker Compose setup:

```yaml
version: "3.8"

services:
  postgres:
    image: postgres:15-alpine
    container_name: [project]_db
    environment:
      POSTGRES_DB: [project]_db
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: root
    ports:
      - "5433:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 10s
      timeout: 5s
      retries: 5

  redis:
    image: redis:7-alpine
    container_name: [project]_redis
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 5s
      retries: 5

volumes:
  postgres_data:
  redis_data:
```

## Testing Rules

### Rule 24: Test Database Setup
- **MUST** use a separate test database
- **MUST** implement test helpers for database setup/teardown
- **MUST** use transactions for test isolation
- **SHOULD** create repository interfaces for easier mocking

### Rule 25: Test Data Management
- **MUST** clean up test data after each test
- **MUST** use factory patterns for test data creation
- **SHOULD** implement database seeding for development

## Logging & Monitoring Rules

### Rule 26: Database Logging
**MUST** configure GORM logger with these settings:

```go
logger.Config{
    SlowThreshold:             time.Second, // Log slow queries
    LogLevel:                  logger.Info, // Log level
    IgnoreRecordNotFoundError: true,        // Don't log "record not found"
    Colorful:                  true,        // Enable colors in development
}
```

### Rule 27: Activity Logging
- **MUST** implement activity logging for security events
- **MUST** log user authentication events
- **MUST** include IP address and user agent in logs
- **MUST** use JSONB for flexible log details

## Error Handling Rules

### Rule 28: Database Error Handling
- **MUST** handle `gorm.ErrRecordNotFound` explicitly
- **MUST** log database errors with context
- **MUST** return appropriate HTTP status codes
- **SHOULD** implement custom error types for business logic

### Rule 29: Transaction Management
- **MUST** use transactions for multi-table operations
- **MUST** implement proper rollback on errors
- **SHOULD** use GORM's transaction methods

## Production Deployment Rules

### Rule 30: Environment Configuration
- **MUST** use different databases for development/staging/production
- **MUST** enable SSL mode in production
- **MUST** configure proper backup strategies
- **MUST** implement database connection monitoring

### Rule 31: Scaling Considerations
- **SHOULD** implement read replicas for high-traffic applications
- **SHOULD** use connection pooling configuration
- **SHOULD** implement caching strategies with Redis
- **MUST** monitor database performance metrics

## Compliance Rules

### Rule 32: Code Organization
- **MUST** follow the repository pattern consistently
- **MUST** separate database concerns from business logic
- **MUST** use dependency injection for database connections
- **MUST** implement proper error handling

### Rule 33: Documentation
- **MUST** document all model relationships
- **MUST** document environment variables
- **MUST** provide setup instructions in README
- **SHOULD** include database schema diagrams

## Implementation Checklist

Use this checklist when implementing the database layer:

- [ ] Set up required dependencies
- [ ] Create project structure
- [ ] Implement database connection
- [ ] Implement Redis connection
- [ ] Define all models with proper tags
- [ ] Implement repository pattern for each domain
- [ ] Set up migration system
- [ ] Configure Docker environment
- [ ] Implement security measures
- [ ] Add proper indexing
- [ ] Set up logging and monitoring
- [ ] Create test database setup
- [ ] Document environment variables
- [ ] Implement error handling
- [ ] Configure production settings

## Anti-Patterns to Avoid

### What NOT to Do:
- ❌ Don't use auto-incrementing integers as primary keys
- ❌ Don't store sensitive data without encryption
- ❌ Don't skip migration systems
- ❌ Don't put database logic in handlers
- ❌ Don't use SELECT * in production queries
- ❌ Don't ignore database errors
- ❌ Don't use global state without proper initialization
- ❌ Don't mix SQL queries with GORM in the same repository
- ❌ Don't skip indexes on foreign keys
- ❌ Don't use development configurations in production

By following these rules, you'll implement a robust, secure, and scalable database layer that follows industry best practices and maintains consistency across projects. 