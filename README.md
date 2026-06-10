# jproxy-go

Go migration of JProxy, scoped to `v0.1.0 Core Proxy MVP`.

It sits between Sonarr/Radarr and Jackett/Prowlarr, forwards Torznab requests, expands basic search terms, merges/trims XML results, and caches results/offsets.

## v0.1.0 scope

Included:

- `/sonarr/jackett/*`
- `/sonarr/prowlarr/*`
- `/radarr/jackett/*`
- `/radarr/prowlarr/*`
- Basic query expansion
- XML merge/count/trim
- Result cache and offset cache
- `/health`
- Docker build/run

Not included in v0.1.0:

- Web admin UI, login/JWT
- SQLite / Java database compatibility
- Full title library, aliases, rule formatting
- Downloader integration

## Quick start

### Run locally

Set the environment variables you need, then run:

```bat
set JACKETT_URL=http://127.0.0.1:9117
set PROWLARR_URL=http://127.0.0.1:9696
go run ./cmd/jproxy
```

The service listens on `:8117` by default. Health check:

```text
http://127.0.0.1:8117/health
```

### Docker

Build from the repository root:

```bat
docker build -t jproxy-go:v0.1.0 -f deployments/docker/Dockerfile .
```

Run:

```bat
docker run --rm --name jproxy-go -p 8117:8117 ^
  -e JACKETT_URL=http://host.docker.internal:9117 ^
  -e PROWLARR_URL=http://host.docker.internal:9696 ^
  jproxy-go:v0.1.0
```

Or with Compose:

```bat
docker compose -f deployments/docker/docker-compose.yml up -d --build
```

## Environment variables

See `configs/jproxy.env.example`.

| Variable | Default | Description |
|---|---:|---|
| `ADDR` | `:8117` | Listen address inside the process/container. |
| `JACKETT_URL` | `http://127.0.0.1:9117` | Upstream Jackett base URL. |
| `PROWLARR_URL` | `http://127.0.0.1:9696` | Upstream Prowlarr base URL. |
| `MIN_COUNT` | `6` | If merged results are fewer than this, try additional basic search titles. |
| `INDEXER_RESULT_CACHE_EXPIRES` | `15` | Result cache TTL in minutes. |
| `CACHE_EXPIRES` | `4320` | Offset cache TTL in minutes. |
| `HTTP_TIMEOUT_SECONDS` | `60` | Upstream HTTP timeout in seconds. |

## Sonarr/Radarr indexer URLs

Keep the original Jackett/Prowlarr API path and replace only the base URL with jproxy-go plus the matching prefix.

Examples:

```text
Jackett for Sonarr:
http://127.0.0.1:8117/sonarr/jackett/api/v2.0/indexers/all/results/torznab

Prowlarr for Sonarr:
http://127.0.0.1:8117/sonarr/prowlarr

Jackett for Radarr:
http://127.0.0.1:8117/radarr/jackett/api/v2.0/indexers/all/results/torznab

Prowlarr for Radarr:
http://127.0.0.1:8117/radarr/prowlarr
```

API keys and query parameters are passed through to Jackett/Prowlarr.

## Development

```bat
go test ./...
go build ./cmd/jproxy
```

## Migration note

This README only documents the v0.1.0 MVP. The original Java JProxy includes database, UI, login, title sync, and rule features that are intentionally out of scope here.
