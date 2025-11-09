# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added - Profile Sync on Social Login (2025-11-08)

#### Automatic Profile Synchronization
- **Profile data now automatically syncs** from social providers on every login
- System updates both `social_accounts` and `users` tables with latest provider data
- **Smart update strategy**: Only updates fields that have changed
- **Non-blocking**: Authentication succeeds even if profile update fails
- Supports all providers: Google, Facebook, GitHub

#### What Gets Synced
- Profile picture (avatar/photo URL)
- Full name, first name, last name
- Email from provider
- Locale/language preference
- Username (GitHub login, etc.)
- Complete raw provider response (JSONB)
- OAuth access token

#### Benefits
- Users see updated profile pictures immediately after changing them on social platforms
- Name changes on social accounts automatically reflected in app
- No manual sync or refresh needed
- Data stays current with social provider

#### Repository Enhancement
- Added `UpdateSocialAccount()` method to social repository
- Enables full social account record updates via GORM

#### Profile Endpoint Enhancement
- Updated `UserResponse` DTO to include all new profile fields
- Added `SocialAccountResponse` DTO for social account data
- Modified `GetUserByID` repository to preload social accounts
- Enhanced `GetProfile` handler to return complete user profile with social accounts
- Profile endpoint now returns: name, first_name, last_name, profile_picture, locale, social_accounts
- Regenerated Swagger documentation to reflect new profile structure

### Added - Social Login Data Enhancement (2025-11-08)

#### User Model Enhancements
- Added `Name` field to store full name from social login or user input
- Added `FirstName` field for first name from social login
- Added `LastName` field for last name from social login
- Added `ProfilePicture` field to store profile picture URL from social providers
- Added `Locale` field for user's language/locale preference

#### Social Account Model Enhancements
- Added `Email` field to store email from social provider
- Added `Name` field to store name from social provider
- Added `FirstName` field for first name from social provider
- Added `LastName` field for last name from social provider
- Added `ProfilePicture` field for profile picture URL from social provider
- Added `Username` field for username/login from providers (e.g., GitHub login)
- Added `Locale` field for locale from social provider
- Added `RawData` JSONB field to store complete raw JSON response from provider

#### Service Layer Enhancements
- Added `UpdateUser()` method to user repository for updating user profile data
- Enhanced Google login handler to capture: email, verified_email, name, given_name, family_name, picture, locale
- Enhanced Facebook login handler to capture: email, name, first_name, last_name, picture (large), locale
- Enhanced GitHub login handler to capture: email, name, login, avatar_url, bio, location, company
- Implemented smart profile update logic: only update user fields if currently empty when linking social accounts
- Store complete provider response in `RawData` field for all providers

#### API Changes
- Profile endpoint (`GET /profile`) now returns additional fields: name, first_name, last_name, profile_picture, locale
- Social account objects now include all new fields in responses
- No breaking changes - all new fields are optional and nullable

### Changed
- Modified social login data extraction to request extended fields from providers
- Updated Facebook Graph API call to request: `id,name,email,first_name,last_name,picture.type(large),locale`
- Enhanced social account linking to preserve and enrich existing user profile data

### Technical Details
- **Migration Method:** GORM AutoMigrate (automatic on application startup)
- **Database Impact:** Adds 5 columns to `users` table, 8 columns to `social_accounts` table
- **Backward Compatibility:** Fully backward compatible - all new fields are nullable
- **Files Modified:**
  - `pkg/models/user.go` - User model with new profile fields
  - `pkg/models/social_account.go` - Social account model with extended data fields
  - `internal/social/service.go` - Enhanced provider handlers for Google, Facebook, GitHub
  - `internal/user/repository.go` - Added UpdateUser method
  - `docs/migrations/MIGRATION_SOCIAL_LOGIN_DATA.md` - Migration documentation

### Documentation
- Added comprehensive migration documentation in `docs/migrations/MIGRATION_SOCIAL_LOGIN_DATA.md`
- Documents data flow changes, database schema updates, and testing recommendations
- Includes security considerations and rollback plan

---

## [1.0.0] - Previous Release

### Features
- User registration and authentication
- Email verification
- Password reset functionality
- Two-factor authentication (TOTP)
- Social login integration (Google, Facebook, GitHub)
- JWT-based authentication (access & refresh tokens)
- Activity logging
- Redis-based session management
- Comprehensive API documentation with Swagger

