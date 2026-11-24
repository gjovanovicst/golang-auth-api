# Documentation site

Static site for the public documentation: a product landing page, the user guide
(`docs/user/**`) and the API reference rendered from `docs/swagger.json`.

No framework, no bundler, no runtime dependencies.

## Layout

```
site/
├── index.html            hand-written landing page
├── styles.css            shared styles (light/dark, responsive)
├── main.js               theme toggle, mobile nav, copy buttons, screenshot lightbox
├── assets/favicon.svg
├── assets/screens/       WebP screenshots used on the landing page and in the guide
└── docs/                 GENERATED — do not edit (gitignored)
```

## Build and preview

```bash
make docs-site PYTHON=python     # Windows; on Linux/macOS: make docs-site
make site-serve PYTHON=python    # rebuild + http://localhost:4173
```

`scripts/build_user_docs.py` reads `docs/user/<group>/*.md` and `docs/swagger.json`
and writes `site/docs/*.html` (index, one page per guide, `api.html`). It runs with
the Python standard library only.

## Pages and ordering

- `docs/user/start/` — getting started (Overview, Quickstart, Concepts)
- `docs/user/integrate/` — using the API (Authentication, 2FA, Sessions/SSO, Webhooks, Errors)
- `docs/user/deploy/` — running it (Configuration, Email, Migrations, Backups, Upgrades, Admin GUI)

The numeric filename prefix only controls ordering; the slug strips it
(`01-quickstart.md` → `quickstart.html`). Cross-page links use the `.md` filename
(e.g. `../start/01-quickstart.md`) and are rewritten to the generated page.

## Screenshots

`assets/screens/*.webp` are real captures of the admin GUI, taken at **2× device pixel
ratio** (2880×1800, light theme) and exported at three widths:

| Variant | Used by |
|---|---|
| `-2880.webp` | the lightbox (crisp on HiDPI and full-HD screens) |
| `-1440.webp` | the hero and the guide pages |
| `-720.webp` | small cards via `srcset` |

WebP quality is 88 — dense text screens lose legibility below that. The lightbox reads
`data-full` (landing) or derives the `-2880` sibling from the `-1440` name (guides), with
a fallback to the inline image if the 2× asset is missing.

The capture script is intentionally not committed: it drives headless Chrome over CDP,
logs into the admin GUI and writes PNGs that are then converted with Pillow.

When the GUI changes, re-capture the affected screens and keep the same filenames so
the pages do not need edits. Keep them under ~100 KB each and always write meaningful
`alt` text.

## GitHub Pages

`.github/workflows/pages.yml` builds the site and deploys it. Enable Pages in the
repository settings (Source: GitHub Actions) and set the repository variable
`PAGES_ENABLED=true`; until then the workflow is skipped instead of failing.
