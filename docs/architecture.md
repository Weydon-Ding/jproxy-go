# Architecture

## Layering

- `cmd/jproxy`: process entrypoint only.
- `internal/config`: configuration loading and validation.
- `internal/cache`: reusable in-memory TTL cache.
- `internal/proxy`: indexer proxy orchestration and HTTP handlers.
- `configs`: sample runtime configuration.
- `deployments`: Docker and deployment artifacts.
- `docs`: design and migration notes.

## Current migration scope

This is a structured initial port, not a feature-complete replacement.

Implemented:

- Sonarr/Radarr Jackett/Prowlarr proxy routes.
- Query expansion basics.
- Result and offset caches.
- RSS/Torznab XML count, merge and trim helpers.

Next:

1. Add SQLite store package.
2. Read original `system_config` records.
3. Port title/rule sync services.
4. Extend the opt-in Radarr static title formatter toward full rule formatting.
5. Add API/UI compatibility layer.
