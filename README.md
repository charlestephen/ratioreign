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
- **JSON status API** — `GET /api/torrents`, `DELETE /api/torrents/{hash}`,
  `GET /healthz`.

## Quick start

```bash
git clone https://github.com/charlestephen/ratioreign.git && cd ratioreign
go build -o ratioreign ./cmd/ratioreign
cp config/config.example.yaml config/config.yaml
# edit config/config.yaml: at minimum pick a clientProfile
./ratioreign -config config/config.yaml
```

Drop a `.torrent` file into `data/torrents/` (or configure RSS/qBittorrent
sync below) and watch it show up:

```bash
curl -s localhost:7070/api/torrents | jq
```

### Container

```bash
cp config/config.example.yaml config/config.yaml
docker compose up -d --build
```

or with plain `docker`/`podman`:

```bash
docker build -t ratioreign -f Containerfile .
docker run -d --name ratioreign \
  -p 7070:7070 \
  -v $PWD/config:/app/config \
  -v $PWD/data:/app/data \
  ratioreign
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
| `DELETE` | `/api/torrents/{hash}` | Stop seeding a torrent immediately (sends a `stopped` announce). |

## Development

```bash
go build ./...
go vet ./...
go test ./...
```

## Scope / limitations

- HTTP tracker announces only — no UDP trackers, no DHT, no PEX (matching
  Joal's scope; HTTP is what private trackers almost universally use).
- No scrape support.
- `torrents/export` requires a reasonably recent qBittorrent; older versions
  aren't supported for the sync feature (the watched folder and RSS intake
  work regardless).

## License

MIT — see [LICENSE](LICENSE).
