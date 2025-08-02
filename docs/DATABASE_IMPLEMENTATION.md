# Database Implementation Documentation

## Overview

This Auth API application uses **PostgreSQL** as the primary database with GORM as the Object-Relational Mapping (ORM) library. The system also employs Redis for caching and session management. This document provides a comprehensive overview of the database architecture, configuration, models, and implementation patterns.

## Database Architecture

### Technology Stack
- **Primary Database**: PostgreSQL 15
- **ORM**: GORM (Go Object-Relational Mapping)
- **Cache Layer**: Redis 7
- **Database Driver**: `gorm.io/driver/postgres`
- **Connection Management**: Environment-based configuration
- **Migration Strategy**: GORM AutoMigrate

## Database Configuration

### Connection Setup

The database connection is established in `internal/database/db.go`:

```go
// Global database instance
var DB *gorm.DB

// ConnectDatabase establishes connection to PostgreSQL database
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
            SlowThreshold:             time.Second, // Slow SQL threshold
            LogLevel:                  logger.Info, // Log level
            IgnoreRecordNotFoundError: true,        // Ignore ErrRecordNotFound error for logger
            Colorful:                  true,        // Enable color
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

### Environment Variables

The following environment variables are required for database connection:

| Variable | Description | Example |
|----------|-------------|---------|
| `DB_HOST` | PostgreSQL server host | `localhost` |
| `DB_USER` | Database username | `postgres` |
| `DB_PASSWORD` | Database password | `root` |
| `DB_NAME` | Database name | `auth_db` |
| `DB_PORT` | Database port | `5432` |

### Docker Configuration

The database is containerized using Docker Compose (`docker-compose.yml`):

```yaml
services:
  postgres:
    image: postgres:15-alpine
    container_name: auth_db
    environment:
      POSTGRES_DB: auth_db
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
```

## Database Models

### 1. User Model (`pkg/models/user.go`)

The core user entity with authentication and security features:

```go
type User struct {
    ID                 uuid.UUID       `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
    Email              string          `gorm:"uniqueIndex;not null" json:"email"`
    PasswordHash       string          `gorm:"" json:"-"` // Stored hashed, not exposed via JSON
    EmailVerified      bool            `gorm:"default:false" json:"email_verified"`
    TwoFAEnabled       bool            `gorm:"default:false" json:"two_fa_enabled"`
    TwoFASecret        string          `gorm:"" json:"-"`           // Stored encrypted, not exposed
    TwoFARecoveryCodes datatypes.JSON  `gorm:"type:jsonb" json:"-"` // Stored encrypted, not exposed
    CreatedAt          time.Time       `gorm:"autoCreateTime" json:"created_at"`
    UpdatedAt          time.Time       `gorm:"autoUpdateTime" json:"updated_at"`
    SocialAccounts     []SocialAccount `gorm:"foreignKey:UserID" json:"social_accounts"`
}
```

**Key Features:**
- UUID primary key with automatic generation
- Email uniqueness constraint
- Password hashing (not stored in plain text)
- Two-factor authentication support
- JSONB storage for recovery codes
- Relationship with social accounts

### 2. SocialAccount Model (`pkg/models/social_account.go`)

Handles OAuth and social media authentication:

```go
type SocialAccount struct {
    ID             uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
    UserID         uuid.UUID  `gorm:"type:uuid;not null;index" json:"user_id"`
    Provider       string     `gorm:"not null;index;uniqueIndex:idx_provider_user_id" json:"provider"`
    ProviderUserID string     `gorm:"not null;uniqueIndex:idx_provider_user_id" json:"provider_user_id"`
    AccessToken    string     `json:"-"` // Stored encrypted, not exposed
    RefreshToken   string     `json:"-"` // Stored encrypted, not exposed
    ExpiresAt      *time.Time `json:"expires_at"`
    CreatedAt      time.Time  `gorm:"autoCreateTime" json:"created_at"`
    UpdatedAt      time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}
```

**Key Features:**
- Supports multiple OAuth providers (Google, Facebook, GitHub)
- Composite unique index on provider and provider_user_id
- Encrypted token storage
- Foreign key relationship with User

### 3. ActivityLog Model (`pkg/models/activity_log.go`)

Tracks user activities and security events:

```go
type ActivityLog struct {
    ID        uuid.UUID       `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
    UserID    uuid.UUID       `gorm:"index" json:"user_id"`
    EventType string          `gorm:"index;not null" json:"event_type"`
    Timestamp time.Time       `gorm:"index;not null" json:"timestamp"`
    IPAddress string          `json:"ip_address"`
    UserAgent string          `json:"user_agent"`
    Details   json.RawMessage `gorm:"type:jsonb" json:"details"`
}
```

**Key Features:**
- Flexible JSONB details field
- Indexed for efficient querying
- Tracks security-relevant events
- Performance-optimized (no foreign key constraint for high volume)

## GORM Implementation Patterns

### Repository Pattern

The application uses the Repository pattern for database operations. Each domain has its own repository:

#### User Repository (`internal/user/repository.go`)

```go
type Repository struct {
    DB *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
    return &Repository{DB: db}
}

// Key methods:
func (r *Repository) CreateUser(user *models.User) error
func (r *Repository) GetUserByEmail(email string) (*models.User, error)
func (r *Repository) GetUserByID(id string) (*models.User, error)
func (r *Repository) UpdateUserPassword(userID, hashedPassword string) error
func (r *Repository) Enable2FA(userID, secret, recoveryCodes string) error
```

#### Social Account Repository (`internal/social/repository.go`)

```go
// Key methods:
func (r *Repository) CreateSocialAccount(socialAccount *models.SocialAccount) error
func (r *Repository) GetSocialAccountByProviderAndUserID(provider, providerUserID string) (*models.SocialAccount, error)
func (r *Repository) UpdateSocialAccountTokens(id string, accessToken, refreshToken string) error
```

#### Activity Log Repository (`internal/log/repository.go`)

```go
// Advanced querying with pagination and filtering:
func (r *Repository) ListUserActivityLogs(userID uuid.UUID, page, limit int, eventType string, startDate, endDate *time.Time) ([]models.ActivityLog, int64, error)
```

### GORM Features Used

1. **Auto Migration**: Automatic schema creation and updates
2. **Associations**: One-to-many relationships between users and social accounts
3. **Indexes**: Performance optimization on frequently queried fields
4. **JSONB Support**: Flexible data storage for PostgreSQL
5. **Soft Deletes**: Available but not currently implemented
6. **Query Building**: Dynamic filtering and pagination

## Database Migrations

### GORM AutoMigrate

The application uses GORM's AutoMigrate feature for schema management:

```go
func MigrateDatabase() {
    // AutoMigrate will create tables, missing columns, and missing indexes
    // It will NOT change existing column types or delete unused columns
    err := DB.AutoMigrate(&models.User{}, &models.SocialAccount{}, &models.ActivityLog{})

    if err != nil {
        log.Fatalf("Failed to migrate database: %v", err)
    }

    log.Println("Database migration completed!")
}
```

**Migration Features:**
- Creates tables if they don't exist
- Adds missing columns
- Creates missing indexes
- **Does NOT** change existing column types
- **Does NOT** delete unused columns

### Production Considerations

For production environments, consider using dedicated migration tools like:
- `golang-migrate/migrate`
- SQL-based migrations for better control
- Version-controlled schema changes

## Redis Integration

### Configuration

Redis is used for caching and session management (`internal/redis/redis.go`):

```go
var Rdb *redis.Client

func ConnectRedis() {
    Rdb = redis.NewClient(&redis.Options{
        Addr:     viper.GetString("REDIS_ADDR"),
        Password: viper.GetString("REDIS_PASSWORD"),
        DB:       viper.GetInt("REDIS_DB"),
    })
}
```

### Use Cases

1. **Refresh Token Storage**: JWT refresh tokens with expiration
2. **Session Management**: User session data
3. **Rate Limiting**: API rate limiting (future implementation)
4. **Caching**: Frequently accessed data

### Redis Operations

```go
// Refresh token management
func SetRefreshToken(userID, token string) error
func GetRefreshToken(userID string) (string, error)
func RevokeRefreshToken(userID, token string) error
```

## Database Schema

### Tables Created

1. **users**
   - Primary key: `id` (UUID)
   - Unique index: `email`
   - Indexes: Standard GORM indexes on timestamps

2. **social_accounts**
   - Primary key: `id` (UUID)
   - Foreign key: `user_id` → `users.id`
   - Composite unique index: `provider` + `provider_user_id`
   - Index: `user_id`

3. **activity_logs**
   - Primary key: `id` (UUID)
   - Indexes: `user_id`, `event_type`, `timestamp`
   - JSONB field: `details`

## Performance Considerations

### Indexing Strategy

1. **Primary Keys**: All tables use UUID primary keys
2. **Email Lookup**: Unique index on `users.email`
3. **Social Authentication**: Composite index on provider fields
4. **Activity Logging**: Indexes on `user_id`, `event_type`, and `timestamp`
5. **Foreign Keys**: Indexed for join performance

### Query Optimization

1. **Pagination**: Implemented in activity log queries
2. **Filtering**: Dynamic where clauses based on parameters
3. **Selective Loading**: JSON fields excluded from sensitive responses
4. **Connection Pooling**: GORM handles connection pooling automatically

### Monitoring

1. **Slow Query Logging**: Configurable threshold (1 second)
2. **Colorized Logging**: Development-friendly output
3. **Health Checks**: Docker health checks for database availability

## Security Considerations

### Data Protection

1. **Password Storage**: Bcrypt hashed passwords
2. **Token Encryption**: Social account tokens stored encrypted
3. **Sensitive Data**: Excluded from JSON responses using `json:"-"`
4. **Two-Factor Auth**: Encrypted secret and recovery codes

### Database Security

1. **Environment Variables**: Database credentials via environment
2. **SSL Mode**: Configurable (currently disabled for development)
3. **Connection Timeouts**: Health check configurations
4. **Access Control**: Database user permissions

## Development Workflow

### Local Development

1. **Docker Compose**: Complete development environment
2. **Auto Migration**: Schema updates on application start
3. **Seed Data**: Can be added to migration function
4. **Database Reset**: Docker volume management

### Testing

1. **Test Database**: Separate database for testing
2. **Transaction Rollback**: Test isolation
3. **Mock Repositories**: Interface-based testing

## Troubleshooting

### Common Issues

1. **Connection Failures**: Check environment variables and Docker status
2. **Migration Errors**: Review model definitions and constraints
3. **Performance**: Monitor slow query logs
4. **Data Integrity**: Verify unique constraints and relationships

### Debug Tools

1. **GORM Logger**: Detailed SQL query logging
2. **PostgreSQL Logs**: Database-level debugging
3. **Redis Commander**: Web UI for Redis inspection (port 8081)

## Dependencies

### Core Database Dependencies

```go
require (
    gorm.io/driver/postgres v1.5.9
    gorm.io/gorm v1.25.12
    github.com/go-redis/redis/v8 v8.11.5
    github.com/google/uuid v1.6.0
)
```

### Supporting Libraries

- `github.com/spf13/viper`: Configuration management
- `github.com/joho/godotenv`: Environment variable loading
- `golang.org/x/crypto`: Password hashing

## Conclusion

This database implementation provides a robust, scalable foundation for the authentication API with:

- **Type Safety**: GORM models with Go structs
- **Performance**: Proper indexing and query optimization
- **Security**: Encrypted sensitive data and secure practices
- **Maintainability**: Repository pattern and clean architecture
- **Scalability**: PostgreSQL with Redis caching layer
- **Development Efficiency**: Auto-migration and Docker setup

The system is designed to handle authentication, social login, two-factor authentication, and comprehensive activity logging while maintaining security and performance standards. 