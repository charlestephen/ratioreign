# RatioReign

A headless BitTorrent ratio-farming daemon written in Go. It announces to your private trackers as if it were seeding real torrents, reporting a believable, gradually climbing upload counter and impersonating a real BitTorrent client's announce fingerprint, **without transferring any torrent data**.

It ships as a single static binary in a small Alpine-based image (`linux/amd64` and `linux/arm64`), with a web UI, a JSON API, RSS feed intake and qBittorrent sync.

- Source and full documentation: https://github.com/charlestephen/ratioreign
- Issues: https://github.com/charlestephen/ratioreign/issues
- Also published at `ghcr.io/charlestephen/ratioreign`

## Tags

| Tag | Meaning |
|---|---|
| `latest` | Most recent build of `main` or the latest release. |
| `2.2`, `2.2.0` | A release, by minor or exact version. |
| `sha-<commit>` | The image built from that commit. |

Tags other than `latest` are not rebuilt. Pull a current tag to get base-image security updates.

## Quick start

Create a config directory and an empty data directory, then pick a `clientProfile` in `config/config.yaml`. A full annotated example is in the repository at [`config/config.example.yaml`](https://github.com/charlestephen/ratioreign/blob/main/config/config.example.yaml).

```bash
mkdir -p config data
cat > config/config.yaml <<'EOF'
clientProfile: qbittorrent-4.6.3
EOF
```

The container runs as UID/GID 1000, so the mounted directories must be writable by that user:

```bash
sudo chown -R 1000:1000 config data
```

Run the container:

```bash
docker run -d --name ratioreign --restart unless-stopped \
  --user 1000:1000 \
  -p 7070:7070 \
  -v "$PWD/config:/app/config" \
  -v "$PWD/data:/app/data" \
  charlestephen/ratioreign:latest
```

To replace a running container with a newer image:

```bash
docker pull charlestephen/ratioreign:latest
docker rm -f ratioreign
# then run the docker run command above again
```

### Docker Compose

```yaml
services:
  ratioreign:
    image: charlestephen/ratioreign:latest
    container_name: ratioreign
    user: "1000:1000"
    restart: unless-stopped
    ports:
      - "7070:7070"
    volumes:
      - ./config:/app/config
      - ./data:/app/data
```

```bash
docker compose up -d
docker compose pull && docker compose up -d   # update
```

`restart: unless-stopped` matters: saving the configuration in the web UI makes the process exit, and Docker's restart policy starts it again with the new file.

## Volumes and ports

| Path / port | Purpose |
|---|---|
| `/app/config` | `config.yaml`. RatioReign writes the generated admin password and any UI changes here. |
| `/app/data` | Watched `torrents/` folder, the archive, and `state.json` (cumulative uploaded totals). |
| `/app/profiles` | Built-in client profiles (`*.client`). Mounting a directory here replaces the built-in set, so copy the profiles you need into it first. |
| `7070/tcp` | Web UI, JSON API and `/healthz`. The port follows `listenAddr` in the config. |

## First login

HTTP Basic Auth is on by default. On the first start with an empty `auth.password`, RatioReign generates a random password and saves it to `config/config.yaml`. The password is never written to the container log. Read it from the file:

```bash
grep -A3 '^auth:' config/config.yaml
```

Then open `http://localhost:7070/`, sign in as `admin` and change the password in the Configuration panel.

Add torrents by dropping `.torrent` files into `data/torrents/`, uploading them in the web UI, configuring RSS feeds, or enabling qBittorrent sync. List what is seeding:

```bash
curl -s -u admin:"$RATIOREIGN_PASSWORD" localhost:7070/api/torrents
```

## Health check

`GET /healthz` returns `ok` and is not behind Basic Auth:

```bash
curl -fs localhost:7070/healthz
```

## Viewing logs and the process

```bash
docker logs -f ratioreign
docker exec ratioreign ls /app/profiles
```

## Key configuration fields

| Field | Meaning |
|---|---|
| `clientProfile` | Name (no extension) of a `*.client` file in `profilesDir` to impersonate. |
| `minUploadRateKBs` / `maxUploadRateKBs` | Bounds in kB/s for the simulated aggregate upload speed, split across active torrents. |
| `simultaneousSeed` | Maximum torrents announcing as seeding at once. Extras wait in a queue. |
| `uploadRatioTarget` | Archive a torrent once `uploaded/size` reaches this ratio. `-1` disables it. |
| `keepTorrentWithZeroLeechers` | If `false`, archive a torrent once its tracker reports zero leechers. |
| `rss` | List of `{name, url, pollInterval}` feeds to poll for new torrents. |
| `qbittorrent` | qBittorrent WebUI sync settings. |
| `auth` | Basic Auth settings. Disable only if the listen address is otherwise unreachable. |

Full field reference: https://github.com/charlestephen/ratioreign#configuration

## Available client profiles

`qbittorrent-4.3.9`, `qbittorrent-4.4.5`, `qbittorrent-4.5.5`, `qbittorrent-4.6.3`, `qbittorrent-4.6.7`, `qbittorrent-5.0.3`, `qbittorrent-5.1.2`, `qbittorrent-5.2.3`, `transmission-4.0.5`.

## Limitations

- HTTP tracker announces only. There is no UDP, DHT or PEX support.
- No scrape support.
- qBittorrent sync needs a reasonably recent qBittorrent that supports `torrents/export`.

## License

MIT. See https://github.com/charlestephen/ratioreign/blob/main/LICENSE
