# Architecture

## Layering

- `cmd/jproxy`: process entrypoint only.
- `internal/config`: configuration loading and validation.
- `internal/store/sqlite`: writable SQLite lifecycle, repositories, and startup snapshot loader.
- `internal/app`: composition root for store, HTTP server, structured logging, and shutdown.
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
- Optional SQLite formatter snapshot mode. The application opens and migrates a
  writable store before the HTTP listener exists, then loads both formatter
  payloads in one read-only transaction from that store. The store remains open
  while serving and closes after HTTP shutdown. Requests never poll, watch, or
  reload the file.
- In DB mode only, the root mux mounts six unauthenticated-until-Todo-11 system
  configuration/cache routes, Todo7 rule/example routes, and Todo8 title routes
  before the proxy fallback. Environment mode has no management routes. In DB
  mode Sonarr/Radarr title sync uses dynamic SQLite configuration, atomic replace
  services, no-redirect HTTP requests, and the shared title-sync admission gate.
  TMDB sync intentionally remains unavailable until Todo10. Config updates accept exactly the fixed Java-compatible
  active ID/key set, reject malformed JSON and invalid values with a redacted
  400, validate only local structure (including Go RE2 compilation), commit the
  SQLite rows and formatter snapshot together, then publish that prepared
  snapshot without another DB or network read. Query is intentionally
  side-effect free: unlike Java it never runs rename work. A committed complete
  configuration update also publishes the currently implemented proxy
  dependencies, `jackettUrl` and `prowlarrUrl`, in that same immutable request
  snapshot. The next proxy request uses both new values; formatter and
  result-cache revisions advance together. Other persisted service settings
  remain future-Todo data and do not construct clients yet. A successful config
  update clears retained sync gate state, so the next sync rereads current URL,
  key, and regex from SQLite without publishing secrets in runtime snapshots. Sonarr templates
  require `{title}`, `{season}`, and `{episode}`; Radarr templates require
  `{title}` and `{year}`, matching Java `CheckUtil`. Language and author strings
  retain Java-compatible values except for the global string-size and
  control-character boundary checks. Version checks use the build's local
  version and a bounded request-context remote fetch. Author fallback starts
  with Java production primary on every request, then uses its configured backup
  only when primary is invalid.

## Formatter payload selection

- With `JPROXY_DB_ENABLED=false`, the existing formatter payload environment
  variables are used unchanged.
- With `JPROXY_DB_ENABLED=true`, `JPROXY_DB_PATH` is required. Active
  `system_config`, rules, and titles become the all-or-nothing payload for both
  formatters. Payload environment variables are ignored, while the Radarr and
  Sonarr `*_FORMAT_ENABLED` variables continue to control route execution.

Intentional difference: successful title sync atomically replaces a domain's
rows and deletes stale rows, rather than preserving Java's upsert-only behavior.
If runtime refresh fails after a database commit, the rows remain committed and
the retained success gate prevents a duplicate replace until explicit invalidation.

Next:

1. Port title/rule sync services.
2. Extend the opt-in static title formatters toward full rule formatting.
3. Add API/UI compatibility layer.

Java JProxy and jproxy-go must not concurrently write a database. With database
mode disabled, formatter environment variables retain their existing behavior.
Runtime reload is deferred to Todo 5.
