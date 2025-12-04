# Upgrade Guide

Step-by-step instructions for upgrading between versions of the Authentication API.

---

## Quick Upgrade

### From v1.0.0 to v1.1.0

**Summary:** Non-breaking upgrade with smart activity logging

**Time Required:** 5-10 minutes

**Downtime:** None required (can upgrade without downtime)

```bash
# 1. Backup database
pg_dump -U postgres -d auth_db > backup_$(date +%Y%m%d).sql

# 2. Pull latest code
git pull origin main
git checkout v1.1.0

# 3. Apply migration (automatic on startup)
make docker-dev

# 4. Done! (Optional: configure logging)
```

See [detailed instructions](#v100-to-v110) below.

---

## Detailed Upgrade Instructions

## v1.0.0 → v1.1.0 (Smart Activity Logging)

### Overview

| Item | Details |
|------|---------|
| **Release Date** | 2024-01-03 |
| **Type** | Minor Release (Non-Breaking) |
| **Breaking Changes** | None |
| **Migration Required** | Yes (Automatic) |
| **Downtime Required** | No |
| **Rollback Difficulty** | Easy |

### What's New

✨ **Smart Activity Logging System**
- 80-95% reduction in database size
- Automatic log cleanup
- Anomaly detection
- Configurable retention policies

See [CHANGELOG.md](CHANGELOG.md) for full details.

### Prerequisites

- [ ] PostgreSQL 12+ running
- [ ] Database backup
- [ ] Access to database
- [ ] Go 1.21+ (if building from source)

### Step-by-Step Upgrade

#### Option A: Docker Upgrade (Recommended)

**Step 1: Backup**
```bash
# Backup database
docker exec auth_db pg_dump -U postgres auth_db > backup_$(date +%Y%m%d_%H%M%S).sql

# Backup .env file
cp .env .env.backup
```

**Step 2: Stop Services**
```bash
docker-compose down
```

**Step 3: Update Code**
```bash
# Pull latest
git pull origin main

# OR checkout specific version
git fetch --tags
git checkout v1.1.0
```

**Step 4: Start Services**
```bash
# Migration runs automatically on startup
docker-compose up -d

# OR
make docker-dev
```

**Step 5: Verify**
```bash
# Check logs
docker logs auth_api_dev

# Should see:
# "Database migration completed!"
# "Activity log cleanup service initialized"
```

**Step 6: Test**
```bash
# Test API
curl http://localhost:8080/activity-logs/event-types

# Check Swagger
open http://localhost:8080/swagger/index.html
```

#### Option B: Manual Upgrade

**Step 1: Backup**
```bash
pg_dump -U postgres -d auth_db > backup_$(date +%Y%m%d_%H%M%S).sql
```

**Step 2: Update Code**
```bash
git pull origin main
git checkout v1.1.0
```

**Step 3: Build**
```bash
go mod download
go build -o auth_api cmd/api/main.go
```

**Step 4: Apply Migration**
```bash
# Migration applies automatically on startup
./auth_api

# OR apply manually:
psql -U postgres -d auth_db -f migrations/20240103_add_activity_log_smart_fields.sql
```

**Step 5: Configure (Optional)**
```bash
# Add to .env (optional)
echo "LOG_CLEANUP_ENABLED=true" >> .env
echo "LOG_CLEANUP_INTERVAL=24h" >> .env
```

**Step 6: Start Application**
```bash
./auth_api
```

### Post-Upgrade Tasks

#### Verify Migration

```bash
# Check new columns exist
docker exec -i auth_db psql -U postgres -d auth_db -c "\d activity_logs"

# Should show:
# - severity
# - expires_at  
# - is_anomaly

# Check indexes
docker exec -i auth_db psql -U postgres -d auth_db -c "\di activity_logs*"
```

#### Optional: Clean Old Logs

```bash
# Clean up old PROFILE_ACCESS and TOKEN_REFRESH logs (recommended)
docker exec -i auth_db psql -U postgres -d auth_db << 'EOF'
DELETE FROM activity_logs WHERE event_type IN ('PROFILE_ACCESS', 'TOKEN_REFRESH');
VACUUM ANALYZE activity_logs;
EOF
```

This can reduce database size by 90%+!

#### Optional: Configure Logging

Add to your `.env` file:
```bash
# Recommended settings
LOG_CLEANUP_ENABLED=true
LOG_CLEANUP_INTERVAL=24h
LOG_ANOMALY_DETECTION_ENABLED=true
LOG_RETENTION_CRITICAL=365
LOG_RETENTION_IMPORTANT=180
LOG_RETENTION_INFORMATIONAL=90
```

See [docs/QUICK_SETUP_LOGGING.md](docs/QUICK_SETUP_LOGGING.md) for more options.

### Rollback (If Needed)

If you encounter issues:

**Step 1: Stop Application**
```bash
docker-compose down
```

**Step 2: Rollback Database**
```bash
# Apply rollback migration
docker exec -i auth_db psql -U postgres -d auth_db -f migrations/20240103_add_activity_log_smart_fields_rollback.sql

# OR restore from backup
docker exec -i auth_db psql -U postgres -d auth_db < backup_YYYYMMDD_HHMMSS.sql
```

**Step 3: Revert Code**
```bash
git checkout v1.0.0
```

**Step 4: Restart**
```bash
docker-compose up -d
```

### Troubleshooting

**Issue: "column already exists"**
```bash
# Migration already applied, safe to continue
# Just restart the application
```

**Issue: Cleanup service not starting**
```bash
# Check logs
docker logs auth_api_dev | grep -i cleanup

# Verify migration applied
docker exec -i auth_db psql -U postgres -d auth_db -c "\d activity_logs"
```

**Issue: Old logs still present**
```bash
# Automatic cleanup runs daily
# For immediate cleanup, see "Optional: Clean Old Logs" above
```

---

## Version Compatibility

### Can I Skip Versions?

**Yes**, but you must apply migrations in order.

**Example: Upgrading from v1.0.0 to v1.2.0**
```bash
# Apply v1.1.0 migration first
psql -U postgres -d auth_db -f migrations/20240103_*.sql

# Then apply v1.2.0 migrations (when available)
psql -U postgres -d auth_db -f migrations/YYYYMMDD_*.sql

# Then checkout v1.2.0
git checkout v1.2.0
```

### Compatibility Matrix

| Upgrade Path | Supported | Notes |
|--------------|-----------|-------|
| v1.0.0 → v1.1.0 | ✅ Yes | Direct upgrade |
| v1.0.0 → v1.2.0 | ✅ Yes | Apply migrations in order |
| v1.1.0 → v1.0.0 | ⚠️ Rollback | Use rollback scripts |

---

## Production Upgrade Checklist

### Pre-Upgrade

- [ ] Read BREAKING_CHANGES.md for target version
- [ ] Test upgrade in development environment
- [ ] Test upgrade in staging environment
- [ ] Backup production database
- [ ] Backup .env configuration
- [ ] Schedule maintenance window (if downtime needed)
- [ ] Notify users (if downtime needed)
- [ ] Prepare rollback plan
- [ ] Have backup restore procedure ready

### During Upgrade

- [ ] Set application to maintenance mode (if needed)
- [ ] Stop application
- [ ] Backup database (final check)
- [ ] Apply migrations
- [ ] Verify migrations succeeded
- [ ] Update configuration
- [ ] Deploy new version
- [ ] Start application
- [ ] Check application logs
- [ ] Verify critical endpoints
- [ ] Run smoke tests

### Post-Upgrade

- [ ] Monitor application logs (15-30 minutes)
- [ ] Monitor database performance
- [ ] Test critical user flows
- [ ] Verify integrations work
- [ ] Monitor error rates
- [ ] Keep backup for 7+ days
- [ ] Document any issues
- [ ] Update monitoring dashboards
- [ ] Notify users upgrade complete

---

## Best Practices

### 1. Always Backup First

```bash
# Full database backup
pg_dump -U postgres -d auth_db > backup_full_$(date +%Y%m%d_%H%M%S).sql

# Schema only (faster restore)
pg_dump -U postgres -d auth_db --schema-only > backup_schema_$(date +%Y%m%d_%H%M%S).sql
```

### 2. Test in Development First

```bash
# Create dev database copy
createdb -U postgres auth_db_dev -T auth_db

# Test upgrade on copy
export DB_NAME=auth_db_dev
make docker-dev
```

### 3. Use Staging Environment

Always test in staging that mirrors production:
- Same database size
- Same configuration
- Same load patterns

### 4. Plan for Rollback

Know how to rollback before you start:
- Have rollback scripts ready
- Have backup verified
- Know rollback procedure
- Test rollback in staging

### 5. Monitor During Upgrade

Watch these metrics:
- Application logs
- Database queries
- Response times
- Error rates
- CPU/Memory usage

### 6. Gradual Rollout (Blue-Green)

For zero-downtime:
1. Deploy new version alongside old
2. Route small % of traffic to new
3. Monitor for issues
4. Gradually increase traffic
5. Decommission old version

---

## Downtime Estimates

| Upgrade | Est. Downtime | Database Size | Notes |
|---------|---------------|---------------|-------|
| v1.0.0 → v1.1.0 | 0 minutes | Any | No downtime needed |

---

## Support

### Getting Help

**Before Upgrading:**
- Read [MIGRATIONS.md](MIGRATIONS.md)
- Check [BREAKING_CHANGES.md](BREAKING_CHANGES.md)
- Review [CHANGELOG.md](CHANGELOG.md)

**During Upgrade:**
- Check application logs
- See [Troubleshooting](#troubleshooting) section
- Search GitHub issues

**After Upgrade:**
- Monitor for 24 hours
- Report any issues on GitHub
- Share feedback

### Report Issues

When reporting upgrade issues, include:
- Current version
- Target version
- Upgrade method used
- Error messages
- Application logs
- Database logs
- Steps to reproduce

**Template:**
```
**Upgrading From:** v1.0.0
**Upgrading To:** v1.1.0
**Method:** Docker
**Error:** [error message]
**Logs:** [relevant logs]
**Steps:** [what you did]
```

---

## FAQ

**Q: Do I need downtime?**
A: v1.0.0 → v1.1.0 requires no downtime. Migration is automatic and backward compatible.

**Q: How long does upgrade take?**
A: 5-10 minutes including backup and verification.

**Q: Can I rollback?**
A: Yes, rollback scripts provided for all migrations.

**Q: Will my API clients break?**
A: No, v1.1.0 has no breaking changes. All endpoints work as before.

**Q: Do I need to configure anything?**
A: No, works with zero configuration. Configuration is optional for customization.

**Q: What happens to existing logs?**
A: They're automatically updated with default values. You can optionally clean old high-frequency logs.

**Q: Is the migration automatic?**
A: Yes, GORM AutoMigrate handles it on startup. Or you can apply SQL manually.

**Q: How do I verify upgrade succeeded?**
A: Check application logs for "Database migration completed!" and "Activity log cleanup service initialized".

---

## Next Steps

After upgrading:
1. ✅ Read [docs/ACTIVITY_LOGGING_GUIDE.md](docs/ACTIVITY_LOGGING_GUIDE.md)
2. ✅ Configure logging (optional): [docs/QUICK_SETUP_LOGGING.md](docs/QUICK_SETUP_LOGGING.md)
3. ✅ Review [CHANGELOG.md](CHANGELOG.md) for all changes
4. ✅ Update your documentation
5. ✅ Monitor application

---

*Last Updated: 2024-01-03*
*For Version: v1.1.0*

