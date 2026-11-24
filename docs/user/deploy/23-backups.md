# Backups and restore

## What actually needs backing up

| Data | Where | Loss impact |
|---|---|---|
| Users, tenants, applications, roles, templates, webhook history, activity logs | **PostgreSQL** | Unrecoverable — this is the backup |
| Sessions, refresh tokens, 2FA codes, rate-limit counters, SSO tokens | **Redis** | Users are logged out; nothing permanent is lost |
| `.env`, SMTP credentials, OAuth secrets | your secret store | Rotate, do not lose |
| Uploaded assets | — | None: the API stores no files |

Redis is a cache with a 720-hour refresh-token horizon, not a database. Do not restore it
from a snapshot after a PostgreSQL restore — stale sessions would outlive the users they
belong to. Start Redis empty and let everyone log in again.

## PostgreSQL backup

```bash
# Docker Compose
docker exec -t auth_db pg_dump -U postgres -d auth_db -Fc > auth_db_$(date +%F).dump

# native PostgreSQL
PGPASSWORD=… pg_dump -h localhost -p 5432 -U postgres -d auth_db -Fc > auth_db_$(date +%F).dump
```

Use the custom format (`-Fc`): it compresses, supports parallel restore and lets you
select tables. The repository ships `scripts/backup_db.sh` (and `scripts/backup_db.bat`)
that wrap this for the Docker setup.

## Restore

```bash
# 1. stop the API so nothing writes during the restore
docker compose stop auth-api

# 2. recreate an empty database
docker exec -it auth_db psql -U postgres -c "DROP DATABASE auth_db WITH (FORCE);"
docker exec -it auth_db psql -U postgres -c "CREATE DATABASE auth_db;"

# 3. restore
docker exec -i auth_db pg_restore -U postgres -d auth_db --clean --if-exists < auth_db_2026-11-24.dump

# 4. start the API and check
docker compose start auth-api
curl -s http://localhost:8080/health -H "X-App-ID: 00000000-0000-0000-0000-000000000001"
```

Then verify: log in with a known account, open the admin GUI, check *Users* and
*Applications* counts, and confirm a 2FA user can still complete a challenge.

## Retention and cadence

- **Daily** logical dump, kept 14 days.
- **Weekly** dump copied off the host (object storage, another machine).
- **Before every upgrade or migration**, an extra dump — named with the target version.
- **Quarterly**, a restore rehearsal on a scratch database. A backup you have never
  restored is a hypothesis, not a backup.

## Point-in-time recovery

If the database runs with WAL archiving (managed PostgreSQL or a self-hosted setup with
`archive_mode=on`), restore to a timestamp instead of the last dump. The API itself does
not manage WAL — that is the database operator's job. Document the command in your
runbook, not here.

## Backing up configuration

`.env` (or your secret manager entries) must be recoverable independently of the
database. Keep them in a password manager or a sealed secret store, never in the repository.
Losing `JWT_SECRET` invalidates every access token; losing `ADMIN_API_KEY` locks automation
out of `/admin/*`.

## Verify a dump without restoring it

```bash
pg_restore --list auth_db_2026-11-24.dump | head -20   # readable?
pg_restore --list auth_db_2026-11-24.dump | grep -c "TABLE DATA"   # table count still sane?
```
