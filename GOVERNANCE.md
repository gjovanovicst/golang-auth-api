# Governance

## Maintainers

Maintainers review and merge changes, cut releases and own security triage. The
current maintainers are listed in [`.github/CODEOWNERS`](.github/CODEOWNERS).
New maintainers are nominated by an existing maintainer and accepted by lazy
consensus after a week of public review.

## Decisions

- **Small changes** — one maintainer review.
- **Architectural or breaking changes** — an ADR in `docs/adr/`, public review,
  and lazy consensus before implementation.
- **Security issues** — private advisory, coordinated disclosure; details are
  published after a fix ships (see [`SECURITY.md`](SECURITY.md)).
- **Roadmap** — issues and [`ROADMAP.md`](ROADMAP.md), not private chat.

## Releases

- Semantic versioning for the API, container images and binaries.
- `main` is always releasable; release tags are immutable.
- Release notes list breaking changes, migrations and rollback steps.
- No release is cut from uncommitted work and without the CI gates green.

## Contact

- **Security:** private GitHub security advisory (see `SECURITY.md`).
- **Conduct:** the contact in [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md).
- **Everything else:** GitHub issues and pull requests.

## Scope

The core application, its documentation and security fixes are open source under
the MIT license. Hosted operations, support/SLA and compliance attestations may be
commercial; security fixes are never paywalled.
