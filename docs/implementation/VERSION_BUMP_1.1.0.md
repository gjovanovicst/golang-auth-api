# Version Bump to 1.1.0

## Release Date
2024-12-04

## Version Information
- **Previous Version**: 1.0.0
- **New Version**: 1.1.0
- **Release Type**: Minor Release (new features, backward compatible)

## Files Updated

### 1. Core Application Files
- ✅ `cmd/api/main.go` - Updated Swagger version annotation from `1.0` to `1.1.0`

### 2. Documentation Files
- ✅ `docs/docs.go` - Regenerated with version 1.1.0
- ✅ `docs/swagger.json` - Updated version field to 1.1.0
- ✅ `docs/swagger.yaml` - Updated version field to 1.1.0
- ✅ `README.md` - Added CI/CD features and commands section
- ✅ `CHANGELOG.md` - Added comprehensive v1.1.0 release notes

### 3. Configuration Files
- ✅ `.github/workflows/ci.yml` - Updated with act-compatible configuration (ports and conditional artifacts)
- ✅ `internal/middleware/auth_test.go` - Updated to support environment variable configuration
- ✅ `internal/log/service.go` - Added security exception comment

### 4. Files NOT Changed (No Version References)
- ❌ `Dockerfile` - No version reference (builds from source)
- ❌ `docker-compose.yml` - No application version tag (uses local build)
- ❌ `go.mod` - Module path unchanged

## What's New in 1.1.0

### CI/CD Infrastructure
1. **GitHub Actions Workflow**
   - Complete CI/CD pipeline with test, build, and security-scan jobs
   - Automated testing with PostgreSQL and Redis services
   - Docker image building and artifact management

2. **Local Testing with Act**
   - Full compatibility with `nektos/act` for local CI testing
   - Smart port configuration to avoid conflicts (PostgreSQL: 5435, Redis: 6381)
   - Conditional artifact handling (skips upload/download for local runs)

3. **Security Scanning**
   - Gosec security scanner integration (0 issues)
   - Nancy vulnerability scanner (optional, requires authentication)
   - Proper security exception documentation

### Test Infrastructure Improvements
1. **Environment Variable Support**
   - Tests now read from CI environment using `viper.AutomaticEnv()`
   - Proper defaults with `viper.SetDefault()` allowing env override
   - Improved test reliability in CI environments

2. **Redis Connection Handling**
   - Better error handling for Redis availability
   - Tests properly skip when Redis is unavailable
   - Configurable Redis connection parameters

### Documentation Updates
1. **README.md Enhancements**
   - Added CI/CD to Developer Experience features
   - New CI/CD Commands section with act usage examples
   - Installation guide for act

2. **CHANGELOG.md**
   - Comprehensive release notes for v1.1.0
   - Clear separation between v1.0.0 and v1.1.0 changes
   - Detailed descriptions of fixes and improvements

## Breaking Changes
**None** - This is a fully backward-compatible release.

## Migration Required
**No** - No database migrations or configuration changes required.

## Docker Image Tagging
The application uses local build in docker-compose and does not use versioned tags. 
If you publish to a registry, you should tag as:
```bash
docker tag auth-api:latest your-registry/auth-api:1.1.0
docker tag auth-api:latest your-registry/auth-api:latest
docker push your-registry/auth-api:1.1.0
docker push your-registry/auth-api:latest
```

## Testing Verification

### All CI Jobs Passing ✅
1. **Test Job** - All tests passing with proper environment configuration
2. **Build Job** - Go build and Docker image build successful
3. **Security-Scan Job** - Gosec security scan passing with 0 issues

### Local Testing with Act
```bash
# Run all jobs locally
act -j test --container-architecture linux/amd64
act -j build --container-architecture linux/amd64
act -j security-scan --container-architecture linux/amd64

# List all available jobs
act -l
```

## Release Checklist

- [x] Update version in `cmd/api/main.go`
- [x] Regenerate Swagger documentation
- [x] Update `CHANGELOG.md` with release notes
- [x] Update `README.md` with new features
- [x] Verify all CI jobs passing
- [x] Test locally with act
- [x] Review security scan results
- [ ] Create git tag: `git tag -a v1.1.0 -m "Release v1.1.0"`
- [ ] Push tag: `git push origin v1.1.0`
- [ ] Create GitHub release with CHANGELOG notes
- [ ] Build and tag Docker images (if publishing)

## Post-Release Actions

1. **Git Tagging**
   ```bash
   git add .
   git commit -m "chore(release): bump version to 1.1.0"
   git tag -a v1.1.0 -m "Release version 1.1.0 - CI/CD Improvements"
   git push origin main
   git push origin v1.1.0
   ```

2. **GitHub Release**
   - Create a new release on GitHub
   - Use tag v1.1.0
   - Copy release notes from CHANGELOG.md
   - Attach any built artifacts if needed

3. **Docker Registry** (if applicable)
   ```bash
   docker build -t auth-api:1.1.0 -t auth-api:latest .
   docker tag auth-api:1.1.0 your-registry/auth-api:1.1.0
   docker push your-registry/auth-api:1.1.0
   docker push your-registry/auth-api:latest
   ```

## Notes

- All changes are backward compatible
- No database migrations required
- No breaking changes to API endpoints
- All existing functionality preserved
- CI/CD improvements enhance developer experience without affecting runtime behavior

