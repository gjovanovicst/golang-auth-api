# Release process

How a version of the Auth API is cut, published and verified.

## Versioning

[Semantic Versioning](https://semver.org): `MAJOR.MINOR.PATCH`, tags prefixed with `v`.

| Change | Version |
|---|---|
| Breaking API, schema or behaviour change | `MAJOR` (and `docs/BREAKING_CHANGES.md`) |
| New feature, additive schema | `MINOR` |
| Fix only, no API/schema change | `PATCH` |

Pre-release tags (`v1.0.0-alpha.1`, `-rc.1`) are allowed; the release workflow marks any
tag containing `-` as a pre-release.

## What a release publishes

Pushing a `v*` tag runs `.github/workflows/release.yml`:

1. **Images** — `ghcr.io/gjovanovicst/golang-auth-api` for `linux/amd64` and
   `linux/arm64`, tagged `X.Y.Z`, `X.Y`, `latest` and `sha-<commit>`; signed with cosign
   (keyless) and published with `provenance` and `sbom` attestations.
2. **Archives** — `auth-api_X.Y.Z_{linux,darwin}_{amd64,arm64}.tar.gz` and
   `auth-api_X.Y.Z_windows_amd64.zip` with `checksums.txt`.
3. **GitHub Release** — body from `docs/releases/vX.Y.Z.md`, assets attached.

## Checklist

### Before the PR

- [ ] `main` is green (`Test`, `Build`, `Security Scan`, `changelog`).
- [ ] `CHANGELOG.md`: `## [Unreleased]` renamed to `## [X.Y.Z] - YYYY-MM-DD`, new empty
      `## [Unreleased]` added.
- [ ] `docs/releases/vX.Y.Z.md` written from `docs/releases/TEMPLATE.md`.
- [ ] `cmd/api/main.go` `@version` updated, then `make swag-init` and rebuild the site.
- [ ] Breaking changes documented in `docs/BREAKING_CHANGES.md` (major only).
- [ ] Migrations reviewed; a rollback file exists for each destructive change.
- [ ] `go build ./...`, `go vet ./...`, `go test ./internal/social/` pass locally.

### Cut the release

```bash
git checkout main && git pull
git tag -a vX.Y.Z -m "Auth API X.Y.Z"
git push origin vX.Y.Z
```

The tag push is the only manual step; the workflow does the rest. Watch it:

```bash
gh run watch $(gh run list --workflow release.yml --limit 1 --json databaseId --jq '.[0].databaseId')
```

### Verify

- [ ] Release page has the notes and all assets (`checksums.txt` included).
- [ ] `docker pull ghcr.io/gjovanovicst/golang-auth-api:X.Y.Z` succeeds.
- [ ] `docker run --rm ghcr.io/…:X.Y.Z` starts and logs `Auth API X.Y.Z (commit …)`.
- [ ] `cosign verify …` passes for the digest printed in the run summary.
- [ ] `sha256sum -c checksums.txt` passes on one archive.
- [ ] The documentation site deployed and the API page shows the new version.
- [ ] `docker run … /health` with a real `.env` returns `healthy`.

**First release only:** the GHCR package is created private. Make it public once in
*Repository → Packages → golang-auth-api → Package settings → Change visibility* and
connect it to the repository.

## If something is wrong

Tags are immutable — never force-push a tag.

| Problem | Action |
|---|---|
| Workflow failed before publishing | delete the tag (`git push --delete origin vX.Y.Z`), fix, re-tag |
| Image is broken | publish a patch (`X.Y.Z+1`) with the fix; edit the broken release to say "superseded" |
| Notes are wrong | edit the GitHub Release (the tag stays) |
| Database migration is wrong | restore the dump, ship a corrective migration in a patch |

## Patch releases

After `1.0.0`, fixes that must reach production before the next minor go to a release
branch:

```bash
git switch -c release/1.0 v1.0.0
```

Merge the fix into `main`, add the `backport/1.0` label to the PR, and cherry-pick it to
`release/1.0`; tag the patch from that branch. The same `release.yml` handles it.
