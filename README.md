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
- SQLite UI management or runtime synchronization
- Full title library, aliases, or remote rule synchronization
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

### Podman integration test stack

The repository includes a local Podman Compose stack for testing jproxy-go with
Prowlarr, Jackett, Sonarr, and Radarr. It uses `docker.m.daocloud.io` mirrors for
the `linuxserver/*arr` images because direct Docker Hub access can be unreliable
in some networks.

Start the stack from the repository root:

```bat
podman compose -f deployments/podman/compose.yml up -d --build
```

Exposed ports:

| Service | URL |
|---|---|
| jproxy-go | `http://127.0.0.1:8117/health` |
| Prowlarr | `http://127.0.0.1:9696` |
| Jackett | `http://127.0.0.1:9117` |
| Sonarr | `http://127.0.0.1:8989` |
| Radarr | `http://127.0.0.1:7878` |

Inside the Compose network, jproxy-go is configured with these upstream URLs:

```text
JACKETT_URL=http://jackett:9117
PROWLARR_URL=http://prowlarr:9696
```

Use the Compose service name when configuring test indexers from the Sonarr/Radarr
web UI. For example, a Prowlarr indexer with ID `1` should use the base URL
`http://jproxy-go:8117/sonarr/prowlarr/1/` in Sonarr and API path `/api`.

```text
Jackett for Sonarr:
http://jproxy-go:8117/sonarr/jackett/api/v2.0/indexers/all/results/torznab

Prowlarr for Sonarr:
http://jproxy-go:8117/sonarr/prowlarr/1/

Jackett for Radarr:
http://jproxy-go:8117/radarr/jackett/api/v2.0/indexers/all/results/torznab

Prowlarr for Radarr:
http://jproxy-go:8117/radarr/prowlarr/1/
```

Stop the stack while keeping the named volumes:

```bat
podman compose -f deployments/podman/compose.yml down
```

Reset all test data:

```bat
podman compose -f deployments/podman/compose.yml down -v
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
| `RESULT_CACHE_MAX_ENTRIES` | `1000` | Maximum number of cached XML result entries. |
| `CACHE_EXPIRES` | `4320` | Offset cache TTL in minutes. |
| `OFFSET_CACHE_MAX_ENTRIES` | `1000` | Maximum number of cached offset entries. |
| `HTTP_TIMEOUT_SECONDS` | `60` | Upstream HTTP timeout in seconds. |
| `JPROXY_DB_ENABLED` | `false` | Open a writable long-lived SQLite store and load formatter payloads at startup. |
| `JPROXY_DB_PATH` | none | Required when DB mode is enabled; ignored while DB mode is disabled. |
| `JPROXY_RADARR_FORMAT_ENABLED` | `false` | Enable the Phase 1 Radarr-only XML title formatter. |
| `JPROXY_RADARR_FORMAT` | none | Required template when formatting is enabled; it must contain `{title}`. |
| `JPROXY_RADARR_TITLE_CLEAN_REGEX` | empty | Optional Java-compatible clean-title regex used for `{cleanTitle}` rules. |
| `JPROXY_RADARR_FORMAT_RULES` | `[]` | Static JSON array of `token`, `regex`, `replacement`, optional numeric `offset`/`priority`, and optional `validStatus` rules. |
| `JPROXY_RADARR_TITLES` | `[]` | Optional static JSON title array with `mainTitle`, `title`, `cleanTitle`, and `year`, used by `{cleanTitle}` title rules. |
| `JPROXY_SONARR_FORMAT_ENABLED` | `false` | Enable the static Sonarr XML title formatter. |
| `JPROXY_SONARR_FORMAT` | none | Required template when Sonarr formatting is enabled; it must contain `{title}`. |
| `JPROXY_SONARR_TITLE_CLEAN_REGEX` | empty | Optional Java-compatible clean-title regex used for Sonarr `{cleanTitle}` rules. |
| `JPROXY_SONARR_FORMAT_RULES` | `[]` | Static JSON array of Sonarr `token`, `regex`, `replacement`, optional numeric `offset`/`priority`, and optional `validStatus` rules. |
| `JPROXY_SONARR_TITLES` | `[]` | Optional static JSON title array with `mainTitle`, `title`, `cleanTitle`, and required `seasonNumber`, used by Sonarr `{cleanTitle}` title rules. |

## SQLite Formatter Snapshot

Set `JPROXY_DB_ENABLED=true` and `JPROXY_DB_PATH` to a writable SQLite file.
jproxy-go opens one long-lived writable store, validates and applies migrations
before it creates the listener, then loads formatter payloads in one read-only
transaction from that same store. The store remains open until HTTP shutdown
completes. Stop Java JProxy before starting database mode: Java and Go must not
write the same database concurrently.

The snapshot requires exactly one active non-NULL `system_config` value each for
`radarrIndexerFormat`, `sonarrIndexerFormat`, and `cleanTitleRegex`. It loads
active Radarr/Sonarr rules and titles, and uses the database clean-title regex
for both formatters. Invalid or incomplete records fail startup.

Database mode is all-or-nothing for formatter payloads: `*_FORMAT`, rules,
titles, and clean-regex environment variables are ignored, so no formatter
mixes database and environment data. `JPROXY_RADARR_FORMAT_ENABLED` and
`JPROXY_SONARR_FORMAT_ENABLED` remain independent execution switches. The
database is still opened and validated when both switches are false. With
database mode disabled, existing environment JSON behavior is unchanged. The
snapshot is startup-only; runtime reload remains Todo 5.

## Radarr Title Formatting

Phase 1 provides an opt-in, static Radarr XML title formatter. It runs only for
Radarr responses after upstream requests and search expansion complete, before a
valid response enters the result cache. Sonarr responses and disabled Radarr
responses are not parsed or rewritten; disabled mode therefore preserves the
upstream XML bytes.

Set `JPROXY_RADARR_FORMAT_ENABLED=true` together with a template containing `{title}` and
`JPROXY_RADARR_FORMAT_RULES` JSON. Invalid enabled JSON or regular expressions
fail startup rather than silently changing responses. Rules run in ascending
numeric `priority`; equal priorities retain JSON declaration order. Omitted
`priority` is `0`, so legacy arrays retain declaration order when priorities tie.
Omitted `validStatus` and `validStatus: 1` enable a rule; `validStatus: 0`
disables it for every token, including `title` and `year`. Any other explicit
`validStatus` value fails startup. Unknown template tokens are removed after
matching.

`{cleanTitle}` title rules require optional static records in
`JPROXY_RADARR_TITLES` when database mode is disabled. See
`configs/jproxy.env.example` for an escaped environment-file example.

## Sonarr Title Formatting

Sonarr formatting is independently opt-in through `JPROXY_SONARR_FORMAT_ENABLED`.
It runs only for Sonarr responses after upstream requests and search expansion
complete, before a valid response enters the result cache. Radarr formatting is
unaffected. Disabled Sonarr responses are returned without XML parsing or byte
changes.

Sonarr uses its own static JSON title records rather than Radarr movie records.
Each record requires `mainTitle`, at least one of `title` or `cleanTitle`, and
`seasonNumber`. `{cleanTitle}` matches fill `{title}` with `mainTitle`; unlike
Radarr, they do not perform a year consistency check. A season number other than
`-1` or `1` fills `{season}` as `S<number>`. If `{episode}` cannot be resolved,
the formatter uses the complete matching title plus optional description, which
matches the Java fallback. Database snapshot mode provides only formatter
payloads; title sync, API/UI configuration, local rule files, remote rule
sources, and runtime reload are not included.

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

This README documents the v0.1.0 MVP plus opt-in Radarr and Sonarr formatters.
The original Java JProxy UI, login, remote rule features, and runtime database
reload remain out of scope. DB-mode Sonarr/Radarr title sync is available through
the management API; TMDB sync remains deferred to Todo10. SQLite writes here are
limited to the owned migration lifecycle described above.
