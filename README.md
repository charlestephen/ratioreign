# RatioReign

A headless BitTorrent ratio-farming daemon written in Go. It announces to
your private trackers as if it were seeding real torrents — reporting a
believable, gradually-climbing upload counter and impersonating a real
BitTorrent client's announce fingerprint — **without transferring a single
byte of actual torrent data**.

This is a clean-room Go reimplementation of the ideas behind
[anthonyraymond/joal](https://github.com/anthonyraymond/joal) (a Java tool
with the same purpose), rebuilt as a single static binary with a container
image, plus two features Joal doesn't have: RSS feed intake and qBittorrent
sync.

## How it works

BitTorrent trackers learn everything they know about a "seed" from HTTP
announce requests: peer id, reported port, and the `uploaded`/`downloaded`/
`left` byte counters. RatioReign sends those announces on a realistic
schedule and increments `uploaded` at a randomized, configurable rate — but
never opens a peer connection or touches the actual file. As far as the
tracker is concerned, you're seeding; as far as your disk and upstream
bandwidth are concerned, nothing happened.

Client fingerprints (peer id format, announce query shape, HTTP headers) are
loaded from JSON "client profiles" in [`profiles/`](profiles/), so
RatioReign's announces look like a specific real client (qBittorrent,
Transmission, etc.) rather than a bespoke tool. The profile schema is
intentionally compatible with Joal's `*.client` files — the community
library of profiles at
[anthonyraymond/joal-conf-generator](https://github.com/anthonyraymond/joal-conf-generator)
or hand-copied from a Joal install drop straight into `profiles/` unmodified.

## Features

- **Fake seeding engine** — HTTP tracker announces (started/stopped/regular),
  simulated upload speed distributed across active torrents by leecher
  demand, configurable simultaneous-seed cap, auto-archive on zero leechers /
  ratio target reached / too many failed announces.
- **Client profiles** — JSON-defined peer id / key generation algorithms,
  per-client query templates and headers. Ships with qBittorrent 4.3.9
  through 5.2.3 (see [`profiles/`](profiles/)) and a Transmission profile.
- **Watched folder** — drop a `.torrent` file into `data/torrents/` and it's
  picked up automatically.
- **RSS intake** — poll one or more torrent-site RSS feeds; new items'
  enclosure links are downloaded straight into the watched folder.
- **qBittorrent sync** — poll a real qBittorrent instance's WebUI API; any
  torrent added there gets its `.torrent` file pulled and seeded (faked) by
  RatioReign too, so you only manage one torrent list.
- **Web UI** — a config-editing dashboard at `/`: a sortable, filterable
  torrent table (click a column header to sort, type in the filter row to
  narrow by name/size/ratio/etc.) with upload and remove, full config
  editing (upload rate, profile, RSS feeds, qBittorrent sync), and a "Test
  connection" button that runs a live qBittorrent login and shows the exact
  response back.
- **Ratio persists across restarts** — each torrent's cumulative uploaded
  total is saved to disk and resumed on the next start, the same role
  qBittorrent's own resume data plays, instead of every restart resetting
  every torrent's ratio to zero.
- **JSON API** — everything the web UI uses is a plain JSON endpoint too
  (see [API](#api) below), so it's fully scriptable.

## Quick start

Create `config/config.yaml` (see [Configuration](#configuration) below for
every field — at minimum pick a `clientProfile`) and an empty `data/`
directory, then run the published image:

```bash
podman run -d --replace --name ratioreign --user 1000:1000 \
  -p 7070:7070 \
  -v $PWD/config:/app/config \
  -v $PWD/data:/app/data \
  ghcr.io/charlestephen/ratioreign:latest
```

Open `http://localhost:7070/` for the web UI, or drop a `.torrent` file into
`data/torrents/` (or configure RSS/qBittorrent sync below) and watch it show
up:

```bash
curl -s localhost:7070/api/torrents | jq
```

## Configuration

See [`config/config.example.yaml`](config/config.example.yaml) for a full
annotated example. Key fields:

| Field | Meaning |
|---|---|
| `minUploadRateKBs` / `maxUploadRateKBs` | Bounds (kB/s) for the simulated aggregate upload speed, re-rolled every 20 minutes and split across active torrents by leecher demand. |
| `simultaneousSeed` | Max torrents announcing "seeding" at once; extras wait in a queue. |
| `clientProfile` | Name (no extension) of a `*.client` file in `profilesDir` to impersonate. |
| `keepTorrentWithZeroLeechers` | If `false`, archive a torrent once its tracker reports zero leechers. |
| `uploadRatioTarget` | Archive a torrent once `uploaded/size` reaches this ratio. `-1` disables it. |
| `torrentsDir` / `archiveDir` | Watched folder and where finished torrents' `.torrent` files get moved. |
| `statePath` | Where cumulative per-torrent uploaded totals are saved so ratio survives a restart (see [Ratio persistence](#ratio-persistence)). |
| `rss` | List of `{name, url, pollInterval}` RSS feeds to poll for new torrents. |
| `qbittorrent` | qBittorrent WebUI sync settings (see below). |

### RSS feeds

```yaml
rss:
  - name: my-tracker
    url: https://tracker.example/rss?key=xxxx
    pollInterval: 10m
```

Each feed's `<enclosure>` link (falling back to `<link>`) is downloaded and
saved as `<feedname>-<hash>.torrent` in `torrentsDir`. Re-polling is
idempotent — already-downloaded items (by GUID/link) are skipped.

### qBittorrent sync

```yaml
qbittorrent:
  enabled: true
  url: http://localhost:8080
  username: admin
  password: your-password
  pollInterval: 30s
  # category: ratioreign   # optional: only sync this category
```

RatioReign logs into qBittorrent's WebUI API, polls `torrents/info`, and for
every torrent it hasn't seen exports the `.torrent` file (`torrents/export`
— **requires qBittorrent >= 4.5** / WebAPI >= 2.8.19) into `torrentsDir`.
From there it's seeded (faked) exactly like any manually-added torrent —
qBittorrent itself is left completely alone; RatioReign only reads from it.

#### Troubleshooting: qBittorrent behind Gluetun (or any VPN sidecar)

If qBittorrent and RatioReign both attach to a VPN container (commonly
Gluetun) for network egress, login can fail even with correct credentials,
for reasons that have nothing to do with RatioReign's code. Use the web UI's
**Test connection** button (or `POST /api/qbittorrent/test`) to see
qBittorrent's actual response — the exact status/body distinguishes these:

1. **Wrong host/address for the topology.** If RatioReign uses
   `network_mode: "service:gluetun"` (shares Gluetun's network namespace,
   same as qBittorrent), there is no separate container hostname to reach it
   by — use `http://localhost:8080` (or `127.0.0.1`), not a container name.
   If RatioReign is a *separate* container that just shares Gluetun's
   container network, and qBittorrent is *also* `network_mode: "service:gluetun"`, it
   has no network identity of its own either — reach it via Gluetun's own
   container name/port (e.g. `http://gluetun:8080`), not `qbittorrent:8080`.
2. **Gluetun's firewall blocks the container bridge network by default.** If
   RatioReign shares Gluetun's network namespace, it inherits Gluetun's
   iptables rules, which by default only allow the VPN tunnel — not traffic
   to other containers on your bridge network. Set
   `FIREWALL_OUTBOUND_SUBNETS=<your bridge subnet>` (e.g.
   `172.17.0.0/16`) on the **Gluetun** container's environment.
3. **qBittorrent's Host header validation** (Options → Web UI → "Enable
   Host header validation", on by default since qBittorrent 4.6.1) rejects
   requests whose `Host` isn't `localhost`/loopback or in its "Server
   domains" allow-list — showing up as a bare `401` with no
   login-specific body (as opposed to a `200` body of `Fails.` for a wrong
   password). Either add RatioReign's address to "Server domains", or
   disable the check, or reach qBittorrent via loopback per point 1.

#### Can RatioReign report its ratio back into qBittorrent's own UI?

**No — confirmed against qBittorrent's own WebUI API reference (all 46
torrent-management endpoints), not assumed.** There is no endpoint to set,
add to, or otherwise credit a torrent's `uploaded` byte count or share
ratio; the closest thing, `setShareLimits`, only sets a *threshold* (when
to auto-stop seeding), not the accumulated value itself. qBittorrent computes
what it displays purely from bytes its own BitTorrent engine actually
transferred — there's no supported way to inject a number it didn't earn.

What you get instead, and it's usually what actually matters: RatioReign
announces the **same info-hash to the same tracker** (using whatever
`.torrent` file — and passkey embedded in its announce URL — you gave it),
so **the tracker's own server-side ratio for your account already reflects
both** qBittorrent's real uploads and RatioReign's reported ones combined.
Private trackers compute ratio from announce data, not from any client's
local UI, so that combined total is what actually satisfies ratio
requirements — it just won't show up inside qBittorrent's own torrent list,
only on the tracker's website. RatioReign's own web UI shows its side of
that total.

## Ratio persistence

Each torrent's cumulative `uploaded` total is saved to `statePath`
(default `./data/state.json`) — on a timer (every 60s), on graceful
shutdown, and whenever a torrent is removed. Restarting RatioReign resumes
a torrent's ratio from there instead of starting back at zero, the same
role qBittorrent's own resume data plays for its torrents.

Archiving a torrent (ratio target reached, zero leechers, dead tracker)
keeps its history in the state file in case you re-add the same torrent
later; only an explicit removal (web UI / `DELETE /api/torrents/{hash}`)
forgets it — the same distinction as pausing versus deleting a torrent in
a real client.

## Web UI

Visit `http://<host>:7070/` (the port matches `listenAddr`) for a dashboard
covering everything the JSON API does:

- **Torrents** — live table (uploaded, ratio, seeders/leechers, last
  announce error), upload a `.torrent` file, remove a torrent.
- **qBittorrent sync status** — last poll time, last error, torrents
  tracked, so a broken sync is visible without reading container logs.
- **Configuration** — every field in `config.yaml`, including dynamic
  RSS feed rows and the qBittorrent panel's **Test connection** button.

Saving the config writes `config.yaml` and then **restarts the process**
(exits cleanly; the container's `restart: unless-stopped` policy brings it
back up reading the new file) rather than hot-reloading each subsystem in
place — simpler and more predictable than trying to tear down and rebuild
the seeder/watchers/qBittorrent sync live. Expect a few seconds of
downtime on every config save.

## Client profiles

A `*.client` file describes how to build an announce request for one real
BitTorrent client. See [`profiles/qbittorrent-4.6.3.client`](profiles/qbittorrent-4.6.3.client)
for a complete example. Fields:

- `peerIdGenerator` — `REGEX` (a small regex subset: literals, one
  `[character class]`, optional `{n}` repeat) or `RANDOM_POOL_WITH_CHECKSUM`.
- `keyGenerator` (optional) — `HASH`, `HASH_NO_LEADING_ZERO`, `REGEX`, or
  `DIGIT_RANGE_TRANSFORMED_TO_HEX_WITHOUT_LEADING_ZEROES`.
- `urlEncoder` — which characters are left unescaped, and hex-escape case.
- `query` — the announce query template, with `{infohash} {peerid} {port}
  {uploaded} {downloaded} {left} {key} {event} {numwant} {ip} {ipv6}`
  placeholders. A param whose placeholder resolves empty (e.g. `{event}` on a
  routine announce) is dropped from the query entirely.
- `requestHeaders` — HTTP headers sent with every announce.

## API

| Method | Path | Description |
|---|---|---|
| `GET` | `/healthz` | Liveness check. |
| `GET` | `/api/torrents` | JSON array of every known torrent's status (uploaded bytes, seeders/leechers, ratio, last error). |
| `POST` | `/api/torrents` | Upload a `.torrent` file (multipart form field `torrent`); seeds it immediately. |
| `DELETE` | `/api/torrents/{hash}` | Stop seeding a torrent immediately (sends a `stopped` announce). |
| `GET` | `/api/config` | Current config as JSON (qBittorrent password redacted; see `passwordSet`). |
| `PUT` | `/api/config` | Validate, write, and apply a new config (triggers a restart — see [Web UI](#web-ui)). A blank qBittorrent `password` keeps the currently-saved one. |
| `GET` | `/api/profiles` | Names of `*.client` files available in `profilesDir`. |
| `GET` | `/api/qbittorrent/status` | Sync health: `enabled`, `lastPollAt`, `lastError`, `torrentsTracked`. |
| `POST` | `/api/qbittorrent/test` | Test a qBittorrent login (`{url, username, password}`) without saving it; returns the exact success/failure message. |

## Development Quick Start

```bash
git clone https://github.com/charlestephen/ratioreign.git && cd ratioreign
go build -o ratioreign ./cmd/ratioreign
cp config/config.example.yaml config/config.yaml
# edit config/config.yaml: at minimum pick a clientProfile
./ratioreign -config config/config.yaml
```

```bash
go build ./...
go vet ./...
go test ./...
```

To build and run the container image locally instead of pulling the
published one (e.g. to test a Containerfile change):

```bash
podman build -t ratioreign -f Containerfile .
podman run -d --replace --name ratioreign --user 1000:1000 \
  -p 7070:7070 \
  -v $PWD/config:/app/config \
  -v $PWD/data:/app/data \
  ratioreign
```

## CI and security scanning

Every push/PR runs (see [`.github/workflows/`](.github/workflows/)):

- **Tests** — build, vet, gofmt, unit tests.
- **Container build** — multi-arch (amd64/arm64), pushed to both
  `ghcr.io/charlestephen/ratioreign` and `docker.io/charlestephen/ratioreign`
  on pushes to `main` (`DOCKERHUB_USERNAME`/`DOCKERHUB_TOKEN` repo secrets
  required for the Docker Hub half).
- **CodeQL** — security + code-quality static analysis (Go and the
  workflows themselves).
- **golangci-lint** — a second static-analysis pass, this one Go-specific:
  ~50 bundled linters (staticcheck, errcheck, unused, govet, and more) via
  the official `golangci/golangci-lint-action`, no account needed. Results
  land in the same Security tab as CodeQL's via native SARIF output
  (`--output.sarif.path`). Replaced an earlier Codacy integration
  (`codacy/codacy-analysis-cli-action`) that, in its account-free mode,
  turned out not to work for a Go-only repo at all — its Go-relevant tools
  (GoSec, bundled Trivy) either got silently skipped or crashed needing
  config only a codacy.com-registered project has, so the job ran green
  without ever producing a real finding. golangci-lint needs no account
  and, verified locally before adopting it, actually finds real issues in
  this codebase.
- **Trivy** — container image vulnerability scan (CRITICAL/HIGH), also
  reported to the Security tab.
- **GitGuardian** — secret-scanning on every PR.

All GitHub Actions are pinned to a commit SHA (not a mutable version tag)
and kept current automatically by Renovate — see
[`renovate.json`](renovate.json) (`config:best-practices`, which includes
`helpers:pinGitHubActionDigests` and `docker:pinDigests`), so the
Containerfile's base images and every action stay on their latest release
without hand-editing SHAs.

## Scope / limitations

- HTTP tracker announces only — no UDP trackers, no DHT, no PEX (matching
  Joal's scope; HTTP is what private trackers almost universally use).
- No scrape support.
- `torrents/export` requires a reasonably recent qBittorrent; older versions
  aren't supported for the sync feature (the watched folder and RSS intake
  work regardless).

## Changelog

See [CHANGELOG.md](CHANGELOG.md) for a detailed history of changes.

## License

MIT — see [LICENSE](LICENSE).
