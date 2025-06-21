# Authentication API

A modern, production-ready Go REST API for authentication and authorization, featuring social login, email verification, JWT, and Redis integration.

---

## 🚀 Features
- Secure user registration & login (JWT access/refresh tokens)
- Social login: Google, Facebook, GitHub
- Email verification & password reset
- Role-based access control (middleware)
- Redis for token/session management
- Dockerized for local development & deployment
- Unit & integration tests

## 🗂️ Project Structure
```
cmd/api/main.go         # Entry point
internal/               # Core logic (auth, user, social, email, middleware, db)
pkg/                    # Models, DTOs, errors, JWT helpers
.env                    # Environment variables
Dockerfile, docker-compose.yml, dev.sh, dev.bat
```

## ⚡ Quick Start (Docker)
1. Clone the repository
2. Copy `.env` and update with your credentials
3. Start development:
   - Windows: `dev.bat`
   - Linux/Mac: `./dev.sh`
4. API available at http://localhost:8080

## 🛠️ Manual Setup
1. Create PostgreSQL DB & update `.env`
2. `go mod tidy`
3. Install [Air](https://github.com/air-verse/air) for hot reload: `go install github.com/air-verse/air@latest`
4. Run: `air` or `go run cmd/api/main.go`

## 🔑 API Endpoints (Summary)
- `POST /register` — Register
- `POST /login` — Login
- `POST /refresh-token` — Refresh JWT
- `POST /forgot-password` — Request password reset
- `POST /reset-password` — Reset password
- `GET /verify-email` — Email verification
- Social login: `/auth/{provider}/login` & `/callback`
- `GET /profile` — User profile (protected)

## 📦 API Response Example
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
- Register/login returns JWT access & refresh tokens
- Access token in `Authorization: Bearer <token>` header
- Refresh token endpoint issues new access/refresh tokens
- Social login redirects to provider, then back to `/callback`

## 🧪 Testing
- Run all tests: `make test` or `go test ./...`
- Coverage: Unit & integration tests for core logic and endpoints

## 🤝 Contributing
Pull requests and issues are welcome! Please open an issue to discuss changes or improvements.

## 🛡️ Security
If you discover a vulnerability, please open an issue or contact the maintainer directly.

## ⚙️ Environment Variables (example)
```
DB_HOST=localhost
DB_PORT=5432
DB_USER=your_db_user
DB_PASSWORD=your_db_password
DB_NAME=your_db_name
JWT_SECRET=supersecretjwtkey
REDIS_ADDR=localhost:6379
EMAIL_HOST=smtp.example.com
EMAIL_USERNAME=your_email@example.com
GOOGLE_CLIENT_ID=your_google_client_id
# ...etc
```

## 🧩 Dependencies
- Gin, GORM, PostgreSQL, Redis, JWT, OAuth2, Viper, godotenv, validator, mail.v2

## 🧪 Development Commands
- `make dev` — Hot reload
- `make test` — Run tests
- `make build` — Build binary

---

## 📄 License
MIT