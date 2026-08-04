# Architecture

## Layering

- `cmd/jproxy`: process entrypoint only.
- `internal/config`: configuration loading and validation.
- `internal/store/sqlite`: read-only startup snapshot loader for formatter data.
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
- Disabled-by-default static Radarr and Sonarr XML title formatters. Each has
  independent environment JSON configuration and runs after search processing,
  before result-cache insertion.
- Optional SQLite formatter snapshot mode. At startup, configuration loads both
  formatter payloads in one read-only transaction, validates them, and closes
  the database before the HTTP server starts. Requests never read, write, poll,
  watch, or reload the file.

## Formatter payload selection

- With `JPROXY_DB_ENABLED=false`, the existing formatter payload environment
  variables are used unchanged.
- With `JPROXY_DB_ENABLED=true`, `JPROXY_DB_PATH` is required. Active
  `system_config`, rules, and titles become the all-or-nothing payload for both
  formatters. Payload environment variables are ignored, while the Radarr and
  Sonarr `*_FORMAT_ENABLED` variables continue to control route execution.

Next:

1. Port title/rule sync services.
2. Extend the opt-in static title formatters toward full rule formatting.
3. Add API/UI compatibility layer.
