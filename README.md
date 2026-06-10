# jproxy-go

A Go migration prototype of [JProxy](https://github.com/LuckyPuppy514/jproxy), focused on reducing memory footprint while preserving the indexer proxy core.

## Status

Structured initial port, not feature-complete yet.

## Project layout

```text
cmd/jproxy              Application entrypoint
internal/config         Runtime configuration
internal/cache          Generic TTL cache
internal/proxy          HTTP proxy and query expansion logic
configs                 Configuration examples
deployments/docker      Docker build artifacts
docs                    Architecture and migration notes
```

## Configuration

See:

```text
configs/jproxy.env.example
```

## Development

```text
make tidy
make test
make build
```

## Migration notes

Implemented:

- `/sonarr/jackett/*`
- `/sonarr/prowlarr/*`
- `/radarr/jackett/*`
- `/radarr/prowlarr/*`
- Basic query expansion
- XML merge/count/trim
- TTL result cache
- TTL offset cache

Pending:

- SQLite store and migrations
- Existing database compatibility
- Sonarr/Radarr/TMDB title sync
- Rule-based title formatting
- Admin API/UI
- Login/JWT
- Downloader integration
