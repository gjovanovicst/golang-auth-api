# Testing

---

## Running Tests

```bash
# All tests with verbose output
make test

# Specific package
go test -v ./internal/auth/...

# Specific test function
go test -v ./internal/user -run TestRegister

# No caching
go test -v ./internal/user -run TestRegister -count=1

# With race detector
go test -v -race ./internal/user

# With coverage report
go test -cover ./...

# 2FA TOTP test (requires TEST_TOTP_SECRET env var)
make test-totp
```

---

## Manual API Testing

```bash
# Using the test script
./test_api.sh

# Or use the interactive Swagger UI
# Navigate to: http://localhost:8080/swagger/index.html
```

---

## Test Coverage

The project includes:

- Unit tests for core logic
- Integration tests for API endpoints
- 2FA/TOTP verification tests
- Authentication flow tests
- Database operation tests
- Rate limiter tests
- Security header tests
- DTO validation tests
- CSRF comparison tests
- Error type tests
- API key utility tests

---

## Before Committing

```bash
make test             # Ensure all tests pass
make fmt              # Format code
make lint             # Check linting rules
make security         # Run security scans
make swag-init        # Update Swagger docs (if API changed)
```
