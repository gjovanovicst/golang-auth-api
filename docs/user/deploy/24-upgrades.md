# Upgrades

The API ships as a single Go binary and a Docker image. Upgrading is a restart plus
whatever migrations the release carries — but auth is the worst place to improvise, so
follow the checklist.

## Before you upgrade

1. Read the release notes and
   [`docs/BREAKING_CHANGES.md`](https://github.com/gjovanovicst/golang-auth-api/blob/main/docs/BREAKING_CHANGES.md)
   for the versions you are crossing.
2. Take a database dump ([Backups](23-backups.md)).
3. Note the current version (`docker inspect` image tag, or the binary's release).
4. Check the environment for variables that were renamed or added (`.env.example` is the
   reference).

## Upgrade

```bash
# 1. pull the new image or replace the binary
docker compose pull auth-api

# 2. apply SQL migrations that ship with the release
make migrate-up

# 3. restart
docker compose up -d auth-api

# 4. watch the boot log
docker compose logs -f auth-api
```

Successful boot ends with `Server starting on port 8080` after
`Database connected successfully!`, `Connected to Redis!` and
`Database migration check completed!`. Any `GORM AutoMigrate Warning` should be read, not
ignored.

## Post-upgrade checks

| Check | Command |
|---|---|
| Health | `curl -H "X-App-ID: $APP" http://localhost:8080/health` |
| Login still works | log in with a test account (password and 2FA) |
| Admin GUI loads | open `/gui/login` and one list page |
| Tokens still validate | `GET /auth/validate` with an existing token |
| Email sends | test email from *Email Servers* |
| Webhooks deliver | trigger a login and inspect *Webhooks → Deliveries* |

## Rollback

```bash
# 1. stop the new version
docker compose stop auth-api

# 2. roll back the schema if the release had migrations
make migrate-down

# 3. restore the previous image tag / binary
docker compose up -d auth-api

# 4. restore the dump if the migration was not reversible
```

Keep the previous image tag pinned until the upgrade has been stable for a few days.

## Versioning policy

- **Patch** (`1.0.1`) — fixes only, no schema changes that need a rollback.
- **Minor** (`1.1.0`) — features, additive schema, no breaking API changes.
- **Major** (`2.0.0`) — breaking changes, documented ahead of time in
  `docs/BREAKING_CHANGES.md` and announced in the release notes.

Release tags are immutable. `latest` follows the newest stable tag; pin an explicit
version in production.

## Multi-instance deployments

The service is stateless apart from PostgreSQL and Redis, so a rolling restart works:

1. Upgrade one instance at a time behind the load balancer.
2. Apply additive migrations **before** the first new instance starts.
3. Deploy the code that reads the new column only after the migration ran everywhere.
4. Never run destructive migrations in the same release as the code that stops using the
   old column — ship it one release apart.

## After the upgrade

Update the version in your infrastructure repository, record the date and the migration
list in your change log, and delete the pre-upgrade dump only after the retention window.
