# Contributing to Authentication API

Thank you for your interest in contributing! We welcome pull requests, issues, and suggestions to make this project better.

## How to Contribute

1. **Fork the repository** and create your branch from `main` or `develop`.
2. **Clone your fork** and set up the project locally (see README for instructions).
3. **Create a descriptive branch name** (e.g., `feature/social-login`, `fix/email-verification-bug`).
4. **Make your changes** with clear, concise commits.
5. **Test your changes** using `make test` or `go test ./...`.
6. **Lint and format** your code: `make fmt` and `make lint`.
7. **Push to your fork** and open a pull request (PR) against the `develop` branch.
8. **Describe your PR** clearly, referencing any related issues.

## Code Style
- Follow Go best practices and idioms.
- Use `go fmt` for formatting.
- Write clear, descriptive commit messages.
- Add/update tests for new features or bug fixes.

## Database Migrations

### When to Create a Migration

Create a migration when making:
- Database schema changes (add/remove/modify tables/columns)
- Data transformations
- Index changes
- Constraint modifications
- Any breaking database changes

### How to Create a Migration

**1. Use the Template**
```bash
# Copy migration template
timestamp=$(date +%Y%m%d_%H%M%S)
cp migrations/TEMPLATE.md migrations/${timestamp}_your_description.md
```

**2. Create SQL Files**

Forward migration (`migrations/YYYYMMDD_HHMMSS_description.sql`):
```sql
BEGIN;
-- Your changes here
ALTER TABLE table_name ADD COLUMN new_column VARCHAR(100);
COMMIT;
```

Rollback migration (`migrations/YYYYMMDD_HHMMSS_description_rollback.sql`):
```sql
BEGIN;
-- Reverse your changes
ALTER TABLE table_name DROP COLUMN new_column;
COMMIT;
```

**3. Test Thoroughly**
```bash
# Test on local database
psql -U postgres -d auth_db_test -f migrations/YYYYMMDD_description.sql

# Verify changes
psql -U postgres -d auth_db_test -c "\d table_name"

# Test rollback
psql -U postgres -d auth_db_test -f migrations/YYYYMMDD_description_rollback.sql
```

**4. Document the Migration**

Fill out the migration documentation file:
- What changed and why
- Impact assessment
- Breaking changes (if any)
- Verification steps

**5. Update Documentation**

- [ ] [migrations/MIGRATIONS_LOG.md](migrations/MIGRATIONS_LOG.md)
- [ ] [MIGRATIONS.md](docs/migrations/MIGRATIONS.md) (if user-facing)
- [ ] [BREAKING_CHANGES.md](docs/BREAKING_CHANGES.md) (if breaking)
- [ ] [UPGRADE_GUIDE.md](docs/migrations/UPGRADE_GUIDE.md) (if version upgrade)
- [ ] [CHANGELOG.md](CHANGELOG.md)

### Migration Checklist

Before submitting PR with migration:

- [ ] Forward migration SQL created
- [ ] Rollback migration SQL created
- [ ] Migration documentation completed
- [ ] Tested locally (apply + rollback)
- [ ] Tested with realistic data volume
- [ ] All documentation files updated
- [ ] Tests added/updated for schema changes
- [ ] PR labeled with "migration"
- [ ] Breaking changes clearly marked (if any)

### Breaking Changes

If your migration is breaking:

1. **Document extensively** in [BREAKING_CHANGES.md](docs/BREAKING_CHANGES.md)
2. **Provide migration path** for users
3. **Bump version appropriately** (major version for breaking changes)
4. **Add to [UPGRADE_GUIDE.md](docs/migrations/UPGRADE_GUIDE.md)** with step-by-step instructions
5. **Consider deprecation first** instead of immediate removal

**Versioning:**

This project is in pre-release (`1.0.0-alpha.N`). After `1.0.0` is officially published, standard semver applies:
- **Major (x.0.0):** Breaking database or API changes
- **Minor (x.y.0):** New features, backward compatible
- **Patch (x.y.z):** Bug fixes, backward compatible

### Resources

- [migrations/README.md](migrations/README.md) - Detailed migration guide
- [migrations/TEMPLATE.md](migrations/TEMPLATE.md) - Migration template
- [MIGRATIONS.md](docs/migrations/MIGRATIONS.md) - User migration guide
- [BREAKING_CHANGES.md](docs/BREAKING_CHANGES.md) - Breaking changes tracker

## Reporting Issues
- Use the GitHub Issues tab.
- Provide as much detail as possible (steps to reproduce, logs, screenshots).

## Code of Conduct
Please be respectful and inclusive. See `CODE_OF_CONDUCT.md` for details.

---
Thank you for helping make this project better!
