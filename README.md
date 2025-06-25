# Authentication API

A modern, production-ready Go REST API for authentication and authorization, featuring social login, email verification, JWT, Two-Factor Authentication (2FA), and Redis integration.

---

## 🚀 Features
- Secure user registration & login (JWT access/refresh tokens)
- **Two-Factor Authentication (2FA) with TOTP and recovery codes**
- Social login: Google, Facebook, GitHub
- Email verification & password reset
- Role-based access control (middleware)
- Redis for token/session management
- Dockerized for local development & deployment
- Unit & integration tests
- **Interactive Swagger API documentation**
- Development and production Docker configurations

## 🗂️ Project Structure
```
cmd/api/main.go         # Entry point
internal/               # Core logic
├── auth/              # Authentication handlers
├── user/              # User management
├── social/            # Social authentication (OAuth2)
├── twofa/             # Two-Factor Authentication
├── email/             # Email verification & password reset
├── middleware/        # JWT auth middleware
├── database/          # Database connection & migrations
├── redis/             # Redis connection & session management
├── config/            # Configuration management
└── util/              # Utility functions
pkg/                   # Shared packages
├── models/            # Database models
├── dto/               # Data Transfer Objects
├── errors/            # Custom error types
└── jwt/               # JWT utilities
docs/                  # API documentation
├── swagger.json       # Generated Swagger spec
├── swagger.yaml       # Generated Swagger spec
├── docs.go            # Generated Swagger docs
├── README.md          # Documentation overview
├── ARCHITECTURE.md    # System architecture
└── API.md             # API reference
.env                   # Environment variables
.github/               # GitHub templates and workflows
├── ISSUE_TEMPLATE/    # Issue templates
└── workflows/         # CI/CD workflows (if any)
Dockerfile             # Production Docker image
Dockerfile.dev         # Development Docker image
docker-compose.yml     # Production Docker Compose
docker-compose.dev.yml # Development Docker Compose
Makefile               # Build and development commands
test_api.sh            # API testing script
.air.toml              # Air configuration for hot reload
dev.sh, dev.bat        # Development startup scripts
CONTRIBUTING.md        # Contribution guidelines
CODE_OF_CONDUCT.md     # Code of conduct
SECURITY.md            # Security policy
LICENSE                # MIT License
```

## 📖 API Documentation (Swagger)
After starting the server, access the interactive API docs at:

- [http://localhost:8080/swagger/index.html](http://localhost:8080/swagger/index.html)

You can try out all endpoints, including social logins and 2FA operations, directly from the browser.

## 🔄 Regenerating Swagger Documentation
If you make changes to your API routes or annotations, regenerate the Swagger docs with:

```bash
make swag-init
```
or
```bash
swag init -g cmd/api/main.go -o docs
```

- Requires the [swag CLI](https://github.com/swaggo/swag) (`go install github.com/swaggo/swag/cmd/swag@latest`)
- This will update the `docs/` folder with the latest API documentation

## ⚡ Quick Start (Docker)
1. Clone the repository
2. Copy `.env` and update with your credentials
3. Start development:
   - Windows: `dev.bat`
   - Linux/Mac: `./dev.sh`
4. API available at http://localhost:8080
5. Swagger docs at http://localhost:8080/swagger/index.html

## 🛠️ Manual Setup
1. Create PostgreSQL DB & update `.env`
2. `go mod tidy`
3. Install [Air](https://github.com/air-verse/air) for hot reload: `go install github.com/air-verse/air@latest`
4. Run: `air` or `go run cmd/api/main.go`

## 🛠️ Makefile Commands

The following `make` commands are available for development, testing, building, and Docker operations:

| Command                | Description                                                      |
|------------------------|------------------------------------------------------------------|
| `make build`           | Build the application binary (`bin/api.exe`).                    |
| `make run`             | Run the application without hot reload.                          |
| `make dev`             | Run with hot reload using [Air](https://github.com/air-verse/air).|
| `make test`            | Run all Go tests with verbose output.                            |
| `make test-totp`       | Run TOTP test (requires `TEST_TOTP_SECRET` environment variable). |
| `make clean`           | Remove build artifacts and temporary files.                      |
| `make install-air`     | Install Air for hot reloading.                                   |
| `make setup`           | Setup development environment (installs Air, tidy/download deps). |
| `make fmt`             | Format code using `go fmt`.                                      |
| `make lint`            | Run linter (`golangci-lint`).                                    |
| `make install-security-tools` | Install security scanning tools (`gosec` and `nancy`).    |
| `make security-scan`   | Run gosec security scanner.                                      |
| `make vulnerability-scan` | Run nancy vulnerability scanner.                               |
| `make security`        | Run all security checks (gosec + nancy).                         |
| `make build-prod`      | Build for production (Linux, static binary).                     |
| `make docker-dev`      | Run development environment with Docker (`./dev.sh`).             |
| `make docker-compose-build` | Build Docker images using docker-compose.                  |
| `make docker-compose-down`  | Stop and remove Docker containers, networks, images, volumes.|
| `make docker-compose-up`    | Start Docker containers in detached mode with build.        |
| `make docker-build`         | Build Docker image (`auth-api`).                            |
| `make docker-run`           | Run Docker container with environment from `.env`.          |
| `make swag-init`            | Generate Swagger documentation (`docs/`).                   |
| `make help`                 | Show all available make commands.                           |

> **Tip:** You can also run `make help` to see this list in your terminal.

## 🔑 API Endpoints

### Authentication
- `POST /register` — User registration
- `POST /login` — User login (with 2FA support)
- `POST /logout` — User logout and token revocation (protected)
- `POST /refresh-token` — Refresh JWT tokens
- `GET /verify-email` — Email verification
- `POST /forgot-password` — Request password reset
- `POST /reset-password` — Reset password with token

### Two-Factor Authentication (2FA)
- `POST /2fa/generate` — Generate 2FA secret and QR code (protected)
- `POST /2fa/verify-setup` — Verify initial 2FA setup (protected)
- `POST /2fa/enable` — Enable 2FA and get recovery codes (protected)
- `POST /2fa/disable` — Disable 2FA (protected)
- `POST /2fa/login-verify` — Verify 2FA code during login (public)
- `POST /2fa/recovery-codes` — Generate new recovery codes (protected)

### Social Authentication
- `GET /auth/google/login` — Initiate Google OAuth2 login
- `GET /auth/google/callback` — Google OAuth2 callback
- `GET /auth/facebook/login` — Initiate Facebook OAuth2 login
- `GET /auth/facebook/callback` — Facebook OAuth2 callback
- `GET /auth/github/login` — Initiate GitHub OAuth2 login
- `GET /auth/github/callback` — GitHub OAuth2 callback

### User Management
- `GET /profile` — Get user profile (protected)

## 📦 API Response Format
**Success:**
```json
{
  "success": true,
  "data": { "token": "..." }
}
```
**Error:**
```json
{
  "success": false,
  "error": "Invalid credentials"
}
```

## 🔒 Authentication Flow

### Standard Authentication
1. Register/login returns JWT access & refresh tokens
2. Access token in `Authorization: Bearer <token>` header
3. Refresh token endpoint issues new access/refresh tokens

### Two-Factor Authentication Flow
1. User enables 2FA via `/2fa/generate`, `/2fa/verify-setup`, and `/2fa/enable`
2. During login, if 2FA is enabled, a temporary token is returned
3. User provides TOTP code or recovery code via `/2fa/login-verify`
4. Final JWT tokens are issued upon successful 2FA verification

### Social Authentication Flow
1. Redirect to provider login endpoint (e.g., `/auth/google/login`)
2. User authorizes with social provider
3. Provider redirects back to callback endpoint
4. JWT tokens are issued for authenticated user

## 🧪 Testing

### Automated Testing
- Run all tests: `make test` or `go test ./...`
- Coverage: Unit & integration tests for core logic and endpoints

### Manual API Testing
- Use the provided test script: `./test_api.sh`
- Test basic authentication flows and error handling
- Interactive testing via Swagger UI at `/swagger/index.html`

## 🐳 Docker Configuration

### Development Environment
```bash
# Start development environment with hot reload
make docker-dev
# or
./dev.sh  # Linux/Mac
dev.bat   # Windows
```

### Production Environment
```bash
# Build and start production containers
make docker-compose-up
# or
docker-compose up -d --build
```

## ⚙️ Environment Variables

### Required Configuration
```bash
# Database Configuration
DB_HOST=localhost          # postgres (for Docker)
DB_PORT=5432
DB_USER=your_db_user
DB_PASSWORD=your_db_password
DB_NAME=auth_db

# Redis Configuration  
REDIS_ADDR=localhost:6379  # redis:6379 (for Docker)

# JWT Configuration
JWT_SECRET=supersecretjwtkey
ACCESS_TOKEN_EXPIRATION_MINUTES=15
REFRESH_TOKEN_EXPIRATION_HOURS=720

# Email Configuration
EMAIL_HOST=smtp.example.com
EMAIL_PORT=587
EMAIL_USERNAME=your_email@example.com
EMAIL_PASSWORD=your_email_password

# Social Authentication (OAuth2)
GOOGLE_CLIENT_ID=your_google_client_id
GOOGLE_CLIENT_SECRET=your_google_client_secret
FACEBOOK_CLIENT_ID=your_facebook_client_id
FACEBOOK_CLIENT_SECRET=your_facebook_client_secret
GITHUB_CLIENT_ID=your_github_client_id
GITHUB_CLIENT_SECRET=your_github_client_secret

# Server Configuration
PORT=8080
```

### Docker vs Local Development
For Docker Compose, use service names:
- `DB_HOST=postgres`
- `REDIS_ADDR=redis:6379`

For local development, use localhost:
- `DB_HOST=localhost`
- `REDIS_ADDR=localhost:6379`

## 🧩 Key Dependencies
- **Web Framework**: Gin
- **Database**: GORM + PostgreSQL
- **Caching**: Go-Redis + Redis
- **Authentication**: JWT, OAuth2
- **Configuration**: Viper, godotenv
- **Validation**: go-playground/validator
- **Email**: gopkg.in/mail.v2
- **2FA**: pquerna/otp, skip2/go-qrcode
- **Documentation**: Swaggo/Swag
- **Development**: Air (hot reload)

## 🧪 Development Workflow
1. `make setup` — Install dependencies and tools
2. `make dev` — Start development server with hot reload
3. `make test` — Run tests during development
4. `make fmt` — Format code before committing
5. `make lint` — Check code quality
6. `./test_api.sh` — Test API endpoints manually

## 🤝 Contributing
Please read [CONTRIBUTING.md](CONTRIBUTING.md) for details on our code of conduct and the process for submitting pull requests.

## 🛡️ Security
Please read [SECURITY.md](SECURITY.md) for information about reporting security vulnerabilities.

## 📚 Documentation
- [Architecture Documentation](docs/ARCHITECTURE.md)
- [API Reference](docs/API.md)
- [Implementation Phases](docs/implementation_phases/)

---

## 📄 License
This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.