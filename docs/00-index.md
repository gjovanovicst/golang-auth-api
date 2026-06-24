# Centrora Platform — Documentation Index

Use this file as your navigation guide. Do not read all docs at once — find what you need below.

## Architecture Docs

| Document | Read When |
|---|---|
| [01-overview.md](architecture/01-overview.md) | Starting fresh or need the big picture |
| [02-service-boundaries.md](architecture/02-service-boundaries.md) | Unsure which service should own something |
| [03-jwt-flow.md](architecture/03-jwt-flow.md) | Working on auth, tokens, JWT claims, or `/auth/reissue` |
| [04-webhook-outbox.md](architecture/04-webhook-outbox.md) | Implementing a webhook handler or the outbox emitter |
| [08-open-questions.md](architecture/08-open-questions.md) | Before making an architectural decision |

## Service References

| Service | Reference | Port |
|---|---|---|
| Auth API | [services/auth-api.md](services/auth-api.md) | `8080` |
| Centrora | [services/centrora.md](services/centrora.md) | `3005` |
| Permissio | [services/permissio.md](services/permissio.md) | `3001` |
| Planora | [services/planora.md](services/planora.md) | `3003` |
| Transora | [services/transora.md](services/transora.md) | `3002` |

## Common Task → Doc Map

| Task | Start Here |
|---|---|
| Add a new endpoint to any service | `CLAUDE.md` → service `CLAUDE.md` |
| Implement org/project creation | `docs/architecture/03-jwt-flow.md` + `docs/services/centrora.md` |
| Add a webhook listener in Transora or Planora | `docs/architecture/04-webhook-outbox.md` |
| Add a permission rule or check | `docs/services/permissio.md` |
| Add a feature entitlement gate | `docs/services/planora.md` |
| Understand why JWT claims contain orgID | `docs/architecture/03-jwt-flow.md` |
| Debug a 403 from auth middleware | `docs/architecture/03-jwt-flow.md` + service `CLAUDE.md` |
| Add a new consumer app to the platform | `docs/architecture/01-overview.md` + `docs/architecture/04-webhook-outbox.md` |
| Check what architectural decisions are pending | `docs/architecture/08-open-questions.md` |
