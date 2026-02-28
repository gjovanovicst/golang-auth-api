# Documentation

Complete documentation for the Authentication API.

---

## Getting Started

| Document | Description |
|----------|-------------|
| [Getting Started](getting-started.md) | Installation, prerequisites, and first steps |
| [Configuration](configuration.md) | Environment variables, OAuth, and logging config |
| [Admin GUI](admin-gui.md) | Setting up and using the admin panel |

---

## Using the API

| Document | Description |
|----------|-------------|
| [API Endpoints](api-endpoints.md) | Full endpoint reference and authentication flows |
| [API Reference (detailed)](API.md) | Complete request/response schemas and examples |
| [Multi-Tenancy](multi-tenancy.md) | Tenant/app management, data isolation, OAuth per-app |
| [Activity Logging](activity-logging.md) | Smart logging, anomaly detection, retention policies |
| [Swagger UI](http://localhost:8080/swagger/index.html) | Interactive API docs (when running) |

---

## Development

| Document | Description |
|----------|-------------|
| [Project Structure](project-structure.md) | Codebase layout and key files |
| [Architecture](ARCHITECTURE.md) | System design, patterns, and layers |
| [Database Migrations](database-migrations.md) | Migration system, commands, and creating migrations |
| [Testing](testing.md) | Running tests, coverage, and pre-commit checks |
| [Makefile Reference](makefile-reference.md) | All available make commands |
| [Contributing](../CONTRIBUTING.md) | Contribution process and standards |
| [Code of Conduct](../CODE_OF_CONDUCT.md) | Community guidelines |

---

## Feature Guides

### Activity Logging
- [Activity Logging Guide](features/ACTIVITY_LOGGING_GUIDE.md) - Complete guide
- [Quick Setup](features/QUICK_SETUP_LOGGING.md) - Quick setup instructions
- [Smart Logging Implementation](features/SMART_LOGGING_IMPLEMENTATION.md) - Implementation details
- [Smart Logging Reference](features/SMART_LOGGING_QUICK_REFERENCE.md) - Quick reference card

### Social Authentication
- [Social Login Data Storage](features/SOCIAL_LOGIN_DATA_STORAGE.md) - How social login data is handled
- [Quick Reference](features/QUICK_REFERENCE_SOCIAL_DATA.md) - Social data quick reference
- [Troubleshooting](features/TROUBLESHOOTING_SOCIAL_LOGIN.md) - Common issues and fixes

### Profile Management
- [Profile Management](features/PROFILE_MANAGEMENT_IMPLEMENTATION.md) - Profile management guide
- [Profile Sync on Login](features/PROFILE_SYNC_ON_LOGIN.md) - Profile synchronization
- [Profile Sync Reference](features/PROFILE_SYNC_QUICK_REFERENCE.md) - Quick reference

### Security
- [Token Blacklisting](features/SECURITY_TOKEN_BLACKLISTING.md) - Token blacklisting system
- [Security Patches](features/SECURITY_PATCH_SUMMARY.md) - Security patches and updates

---

## Setup Guides

- [Environment Variables](guides/ENV_VARIABLES.md) - Complete environment variable reference
- [Validation Endpoint](guides/auth-api-validation-endpoint.md) - Auth validation endpoint guide
- [Multi-App OAuth Config](guides/multi-app-oauth-config.md) - Per-application OAuth setup
- [Nancy Setup](guides/NANCY_SETUP.md) - Dependency vulnerability scanner setup

---

## Database & Migrations

- [Migration Documentation Index](migrations/README.md) - Migration system overview
- [User Guide](migrations/USER_GUIDE.md) - How to run migrations
- [Upgrade Guide](migrations/UPGRADE_GUIDE.md) - Version upgrade instructions
- [Quick Reference](migrations/MIGRATION_QUICK_REFERENCE.md) - Command cheat sheet
- [Docker Migrations](migrations/MIGRATIONS_DOCKER.md) - Docker-specific commands
- [Migration Strategy](migrations/MIGRATION_STRATEGY.md) - Migration planning

---

## Reference

- [Pre-Release Migration Guide](BREAKING_CHANGES.md) - For early fork users upgrading
- [Changelog](../CHANGELOG.md) - Version history and release notes
- [Security Policy](../SECURITY.md) - Vulnerability reporting

---

## Quick Lookup

| I want to... | Go to |
|--------------|-------|
| Set up the project | [Getting Started](getting-started.md) |
| Configure environment | [Configuration](configuration.md) |
| See API endpoints | [API Endpoints](api-endpoints.md) |
| Set up social login | [Configuration - OAuth](configuration.md#social-authentication) |
| Manage tenants/apps | [Multi-Tenancy](multi-tenancy.md) |
| Use the admin panel | [Admin GUI](admin-gui.md) |
| Run database migrations | [Database Migrations](database-migrations.md) |
| Run tests | [Testing](testing.md) |
| Contribute code | [Contributing](../CONTRIBUTING.md) |
| Understand the architecture | [Architecture](ARCHITECTURE.md) |
| See all make commands | [Makefile Reference](makefile-reference.md) |
| Configure logging | [Activity Logging](activity-logging.md) |
| Troubleshoot social login | [Troubleshooting](features/TROUBLESHOOTING_SOCIAL_LOGIN.md) |
