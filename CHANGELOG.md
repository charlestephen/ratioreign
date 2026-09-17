# Changelog

All notable changes to RatioReign are documented in this file, starting
from this entry onward. Format loosely follows
[Keep a Changelog](https://keepachangelog.com/).

## [1.0.1] - 2026-09-17

### Added

- **Ratio persistence across restarts.** Each torrent's cumulative
  `uploaded` byte total is now saved to a state file (`statePath` in
  config, default `./data/state.json`) — on a 60-second timer, on clean
  shutdown, and whenever a torrent is explicitly removed. Restarting
  RatioReign resumes a torrent's ratio from where it left off instead of
  resetting to zero, the same role qBittorrent's own resume data plays.
  Archiving a torrent (ratio target reached, zero leechers, dead tracker)
  keeps its history for if the same torrent is re-added later; only an
  explicit removal forgets it. (`internal/seeder/state.go`,
  `internal/seeder/seeder.go`)
- **Sortable, filterable web UI torrent table.** Click any column header
  (Name, Uploaded, Ratio, Seeders, Leechers, Active, Last error) to sort by
  it, ascending/descending; a filter row under the headers narrows the
  table live (substring match for text columns, minimum-value for numeric
  ones, active/waiting for the Active column) — all client-side against
  the already-fetched list, so it doesn't add API calls and survives the
  3-second auto-refresh without losing the user's current sort/filter.
  (`internal/api/webui/index.html`)
- **Alternating row colors** (zebra striping) on the torrent table for
  readability, using a theme-aware CSS variable so it works in both light
  and dark mode rather than hardcoding white.
- **golangci-lint** (`.github/workflows/golangci-lint.yml`) — Go-specific
  static analysis: ~50 bundled linters (staticcheck, errcheck, unused,
  govet, and more) via the official `golangci/golangci-lint-action`, no
  account needed, native SARIF output straight to the Security tab
  alongside CodeQL's. Verified locally before adopting it — a real run
  against this codebase found 9 genuine issues (6 unchecked error returns,
  2 De Morgan simplifications, 1 dead struct field), confirming it
  actually works, unlike what it replaced (see below).
  - *Superseded attempt*: Codacy security/quality scanning via
    `codacy/codacy-analysis-cli-action`'s account-free "GitHub code
    scanning" mode was tried first and, after three rounds of fixes (a
    SARIF-category collision with Trivy's upload; Codacy's own bundled
    `trivy` sub-tool crashing on missing pattern config only a
    codacy.com-registered project has; wiring both the analysis and
    upload steps with `continue-on-error` so Codacy's internal failures
    couldn't fail CI), the job ran green but never actually produced a
    usable finding for this repo — in this mode almost every Codacy tool
    relevant to Go (GoSec, StaticCheck, its bundled Trivy) gets skipped or
    crashes without a paid/free Codacy account. Removed
    (`.github/workflows/codacy.yml`, `.codacy.yml`) in favor of
    golangci-lint, which needs no account and demonstrably works.
- **Trivy container vulnerability scanning**
  (`.github/workflows/trivy.yml`) — builds a single-arch image from the
  Containerfile and scans it for CRITICAL/HIGH CVEs, reporting to the
  Security tab. Chosen over a named-but-nonexistent "Privvy" scanner and
  over Snyk (which needs a paid/free-tier account token the repo doesn't
  have) as the free, no-account-needed industry standard.
- `CHANGELOG.md` (this file) — going forward, updated with every
  significant change, not just at release/tag time.
- **Tag-triggered GitHub Releases** (`.github/workflows/release.yml`).
  Pushing a `v*` tag (e.g. `v0.1.0`) extracts that version's section from
  this changelog (matched by its `## [x.y.z]` header) and publishes it as
  the GitHub Release notes automatically — the changelog entry *is* the
  release notes, written once. The same tag push already triggers
  `build.yml`'s existing `type=semver` image tags, so the image and the
  release are two views of the same tag rather than separate manual
  steps. Changed headers from `## 2026-09-17` to `## [0.1.0] - 2026-09-17`
  to make each version's section machine-extractable.

### Changed

- **Go floor bumped 1.23 → 1.27** (`go.mod`) to match the Containerfile's
  already-current `golang:1.27-alpine` builder image.
- Repo branch ruleset: removed the `copilot_code_review` and `code_quality`
  rules, which had no way to ever be satisfied in this repo (no Copilot
  license / no separate code-quality tool reporting to that specific
  check) and were silently blocking every PR merge — including routine
  Renovate dependency-digest bumps — regardless of all real checks (tests,
  CodeQL, container build) passing. Required review count, signed commits,
  and CodeQL's own gate are unchanged.
- All Renovate-managed dependency bumps that had accumulated while the
  ruleset issue above was unresolved were merged: `docker/setup-buildx-action`
  and `docker/setup-qemu-action` digest updates (#15, #16).

- **README simplified to a container-first Quick Start.** The published
  `ghcr.io/charlestephen/ratioreign` image is now the primary path — a
  single `podman run` command, no repo clone or local build required.
  The old clone/build/run-from-source flow moved to a renamed
  **Development Quick Start** section, alongside a `podman build` +
  `podman run` option for testing local Containerfile changes. All
  user-facing example commands now use `podman` instead of `docker`
  (`docker compose`/`docker build`/`docker run` are gone); references to
  the real Docker Hub registry and GitHub Actions' `docker/*` tooling
  are unchanged since those name actual external things, not a CLI
  choice.

### Fixed / Investigated

- **Container image CVEs (1 high, 4 medium, 10 low), all the same root
  cause.** Trivy flagged `libssl3`/`libcrypto3` in the `alpine:3.24`
  runtime image at `3.5.7-r0`, one patch behind the fixed `3.5.8-r0`
  (CVE-2026-14456 high; CVE-2026-75803/63076/63072/18798 medium; several
  more low). Added `apk upgrade --no-cache` to the Containerfile's
  runtime stage so pinned base-image digests can't silently carry
  already-fixed OS package CVEs between Renovate's digest bumps.
  Verified by building the image locally and checking the installed
  package versions directly (`apk list -I`) — confirmed
  `libssl3-3.5.8-r0` / `libcrypto3-3.5.8-r0`, not just assumed from the
  Containerfile diff.
- **Confirmed, not assumed: qBittorrent's WebUI API cannot receive fake
  ratio credit.** Checked all 46 documented torrent-management endpoints
  directly against qBittorrent's own API reference — none can set or add
  to a torrent's uploaded-byte count or share ratio; `setShareLimits` only
  sets an auto-stop *threshold*, not the accumulated value. Documented in
  the README along with the practical equivalent: RatioReign already
  announces to the same tracker under the same passkey as qBittorrent, so
  the tracker's own server-side ratio (the number that actually matters
  for ratio requirements) already reflects both clients' contributions
  combined — it just won't appear inside qBittorrent's own local UI.

## Prior work (not itemized here)

RatioReign's initial build — the fake-seeding engine, client-profile
system, watched-folder/RSS/qBittorrent intake, the web UI's core
config-editing and torrent-management surface, the Gluetun/qBittorrent
login fix, and the CI/CodeQL/signed-commits setup — predates this
changelog. See the README and git history for that work; entries from this
point forward are logged here going forward.
