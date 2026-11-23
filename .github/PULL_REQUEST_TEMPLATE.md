# Pull Request

## What

<!-- What changes, in 1–3 bullets. Link the issue: Closes #123 -->

## Why

<!-- The problem this solves and why now. Motivation, not a restatement of the diff. -->

## How tested

<!-- Commands, test names and the manual flow you verified. Be specific:
  - `go test ./internal/twofa/`
  - `make check`
  - manual: registered a user, enabled TOTP, completed /2fa/login-verify
-->

## Risk and rollback

- **Risk:** low | medium | high — <!-- what can break, who is affected -->
- **Rollback:** <!-- revert commit? reverse migration? feature flag? -->
- **Breaking change:** yes | no — <!-- API/schema/behaviour; if yes, describe the migration path -->

## Screenshot (UI changes)

<!-- Before/after or a short recording. Delete this section for non-UI changes. -->

## Checklist

- [ ] I read `AGENTS.md` / `CONTRIBUTING.md` and followed the project conventions
- [ ] `CHANGELOG.md` has an entry under `## [Unreleased]` (or this PR has the `skip-changelog` label)
- [ ] Documentation is updated when behaviour, configuration or the API changed
- [ ] New/changed endpoints have Swagger annotations and `docs/` was regenerated
- [ ] Database changes ship with a migration (and a rollback file)
- [ ] `go build ./...` and `go vet ./...` pass; tests pass locally
- [ ] Required checks are green (`Test`, `Build`, `Security Scan`, `changelog`)
