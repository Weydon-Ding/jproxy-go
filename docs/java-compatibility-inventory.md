# Java Compatibility Inventory

## Purpose and Version Boundary

This inventory is the durable migration contract for the Java JProxy replacement.
It records the Java HTTP surface and related service behavior found under
`../jproxy/src/main/java/com/lckp/jproxy`. It is intentionally a post-v0.1.0
work item: `v0.1.0` remains the historical Core Proxy MVP, not a claim of full
Java compatibility.

Status values:

- **Implemented in Go**: available in the current Go process.
- **Pending migration**: planned for a later complete-migration todo.
- **Intentional difference**: a documented Go behavior deliberately differs.
- **Java stub**: the Java source is incomplete and must not be copied as a Go
  compatibility target.

## Current Go Proxy Surface

| Java concern | Java source | Status in Go | Notes |
| --- | --- | --- | --- |
| Sonarr + Jackett proxy | `SonarrJackettServiceImpl` | Implemented in Go | `/sonarr/jackett/*`; forwards query parameters and performs baseline search/cache handling. |
| Sonarr + Prowlarr proxy | `SonarrProwlarrServiceImpl` | Implemented in Go | `/sonarr/prowlarr/*`; includes indexer subpaths. |
| Radarr + Jackett proxy | `RadarrJackettServiceImpl` | Implemented in Go | `/radarr/jackett/*`. |
| Radarr + Prowlarr proxy | `RadarrProwlarrServiceImpl` | Implemented in Go | `/radarr/prowlarr/*`; includes indexer subpaths. |
| Indexer cache keys | `IndexerServiceImpl.generateCacheKey` | Implemented in Go | API key is excluded from result-cache keys. |
| Indexer offset keys | `IndexerServiceImpl.generateOffsetKey` | Implemented in Go | API key and numeric offset are excluded from offset-cache keys. |
| Search expansion, merge, trim | `IndexerServiceImpl` and child indexer services | Implemented in Go | MVP/basic expansion only; full title-library expansion remains pending. |
| XML title formatting | Java indexer/rule/title services | Intentional difference | Go supports static opt-in Sonarr/Radarr formatters and optional read-only snapshot input; full runtime rule/title behavior remains pending. Disabled formatters preserve upstream bytes. |
| Health check | no Java controller mapping | Implemented in Go | `GET /health` is a Go operational endpoint. |

## Java Controller Route Inventory

All paths below combine the controller class-level mapping with the method-level
mapping. All Java management routes are pending migration unless noted otherwise.

| Controller | Java route | Java method | Status in Go | Planned migration |
| --- | --- | --- | --- | --- |
| `SystemConfigController` | `GET /api/system/config/version` | `version` | Implemented in Go | DB mode only; `debug.ReadBuildInfo().Main.Version` is used except empty/`(devel)` becomes `dev`; a bounded request-context GitHub latest-release lookup appends ` 🚨` on a different valid tag. |
| `SystemConfigController` | `GET /api/system/config/query` | `query` | Intentional difference | DB mode only; returns the ID-sorted 20-row array without Java's rename-task side effect. Persisted API keys and downloader passwords are returned as the write-only sentinel `******`, never as plaintext. |
| `SystemConfigController` | `POST /api/system/config/update` | `update` | Implemented in Go | DB mode only; strict complete 20-row update, Java formatter token sets, one SQLite transaction, and one prepared runtime publication. Reposting `******` for a sensitive key preserves its stored value; an explicit different value replaces it. |
| `SystemConfigController` | `GET /api/system/config/author/list` | `listAuthor` | Implemented in Go | DB mode only; every request attempts Java production primary `https://raw.githubusercontent.com/LuckyPuppy514/jproxy/main/src/main/resources/rule/author.json`, then backup `https://github.rn.lckp.top/LuckyPuppy514/jproxy/main/src/main/resources/rule/author.json`, otherwise `LuckyPuppy514`. |
| `SystemCacheController` | `POST /api/system/cache/clearAll` | `clearAll` | Implemented in Go | DB mode only; refreshes the provider before clearing current named caches. |
| `SystemCacheController` | `POST /api/system/cache/clear` | `clear` | Implemented in Go | DB mode only; unknown or blank names are strict 400 with no effects. |
| `SystemUserController` | `POST /api/system/user/login` | `login` | Implemented in Go | DB mode only; issues an expiring JWT when login is enabled and an in-process anonymous token when disabled. |
| `SystemUserController` | `GET /api/system/user/info` | `info` | Implemented in Go | DB mode only; requires current claims and always masks the stored password. |
| `SystemUserController` | `POST /api/system/user/update` | `update` | Implemented in Go | DB mode only; protected while login is enabled and intentionally rejected while disabled. |
| `SystemUserController` | `POST /api/system/user/logout` | `logout` | Implemented in Go | DB mode only; revokes the presented token until expiry. |
| `SystemUserController` | `GET /api/system/user/isLoginEnabled` | `isLoginEnabled` | Implemented in Go | DB mode only; remains public so the UI can select the login flow. |
| `SonarrRuleController` | `POST /api/sonarr/rule/sync` | `sync` | Implemented in Go | DB mode only; HTTP contract implemented, real sync deferred to Todo9/10, production returns 503 until adapter installed. |
| `SonarrRuleController` | `GET /api/sonarr/rule/query` | `query` | Implemented in Go | DB mode only; deterministic ordering. |
| `SonarrRuleController` | `POST /api/sonarr/rule/save` | `save` | Implemented in Go | DB mode only. |
| `SonarrRuleController` | `POST /api/sonarr/rule/remove` | `remove` | Implemented in Go | DB mode only. |
| `SonarrRuleController` | `POST /api/sonarr/rule/enable` | `enable` | Implemented in Go | DB mode only. |
| `SonarrRuleController` | `POST /api/sonarr/rule/disable` | `disable` | Implemented in Go | DB mode only. |
| `SonarrRuleController` | `POST /api/sonarr/rule/export` | `export` | Implemented in Go | DB mode only. |
| `SonarrRuleController` | `POST /api/sonarr/rule/import` | `importSonarrRule` | Implemented in Go | DB mode only; bounded single-file multipart import is atomic. |
| `SonarrRuleController` | `GET /api/sonarr/rule/token/list` | `listToken` | Implemented in Go | DB mode only. |
| `RadarrRuleController` | `POST /api/radarr/rule/sync` | `sync` | Implemented in Go | DB mode only; HTTP contract implemented, real sync deferred to Todo9/10, production returns 503 until adapter installed. |
| `RadarrRuleController` | `GET /api/radarr/rule/query` | `query` | Implemented in Go | DB mode only; deterministic ordering. |
| `RadarrRuleController` | `POST /api/radarr/rule/save` | `save` | Implemented in Go | DB mode only. |
| `RadarrRuleController` | `POST /api/radarr/rule/remove` | `remove` | Implemented in Go | DB mode only. |
| `RadarrRuleController` | `POST /api/radarr/rule/enable` | `enable` | Implemented in Go | DB mode only. |
| `RadarrRuleController` | `POST /api/radarr/rule/disable` | `disable` | Implemented in Go | DB mode only. |
| `RadarrRuleController` | `POST /api/radarr/rule/export` | `export` | Implemented in Go | DB mode only. |
| `RadarrRuleController` | `POST /api/radarr/rule/import` | `importRadarrRule` | Implemented in Go | DB mode only; bounded single-file multipart import is atomic. |
| `RadarrRuleController` | `GET /api/radarr/rule/token/list` | `listToken` | Implemented in Go | DB mode only. |
| `SonarrExampleController` | `POST /api/sonarr/example/save` | `save` | Implemented in Go | DB mode only. |
| `SonarrExampleController` | `GET /api/sonarr/example/query` | `query` | Implemented in Go | DB mode only; deterministic ordering. |
| `SonarrExampleController` | `POST /api/sonarr/example/remove` | `remove` | Implemented in Go | DB mode only. |
| `RadarrExampleController` | `POST /api/radarr/example/save` | `save` | Implemented in Go | DB mode only. |
| `RadarrExampleController` | `GET /api/radarr/example/query` | `query` | Implemented in Go | DB mode only; deterministic ordering. |
| `RadarrExampleController` | `POST /api/radarr/example/remove` | `remove` | Implemented in Go | DB mode only. |
| `SonarrTitleController` | `POST /api/sonarr/title/sync` | `sync` | Implemented in Go | DB mode only; dynamic SQLite configuration, redirect rejection, atomic stale-row replacement, and shared admission gate. |
| `SonarrTitleController` | `GET /api/sonarr/title/query` | `query` | Implemented in Go | DB mode only; deterministic PageResponse. |
| `SonarrTitleController` | `POST /api/sonarr/title/remove` | `remove` | Implemented in Go | DB mode only; exact runtime invalidation. |
| `RadarrTitleController` | `POST /api/radarr/title/sync` | `sync` | Implemented in Go | DB mode only; dynamic SQLite configuration, redirect rejection, atomic stale-row replacement, and shared admission gate. |
| `RadarrTitleController` | `GET /api/radarr/title/query` | `query` | Implemented in Go | DB mode only; deterministic PageResponse. |
| `RadarrTitleController` | `POST /api/radarr/title/remove` | `remove` | Implemented in Go | DB mode only; exact runtime invalidation. |
| `TmdbTitleController` | `POST /api/tmdb/title/sync` | `sync` | Implemented in Go | DB mode only; intentionally returns 503 until Todo10. |
| `TmdbTitleController` | `GET /api/tmdb/title/query` | `query` | Implemented in Go | DB mode only; deterministic PageResponse and no-write clean-title projection. |
| `TmdbTitleController` | `POST /api/tmdb/title/remove` | `remove` | Implemented in Go | DB mode only; exact runtime invalidation. |
| `TmdbTitleController` | `POST /api/tmdb/title/save` | `save` | Implemented in Go | DB mode only; supplied, generated and reused `tmdbId` semantics. |
| `RuleController` | `GET /api/rule/test` | `test` | Implemented in Go | DB mode only; Java-compatible regex projection. |

## Java Service Behavior Inventory

| Java service | Relevant public behavior | Status in Go | Planned migration / compatibility note |
| --- | --- | --- | --- |
| `IndexerServiceImpl` and Sonarr/Radarr Jackett/Prowlarr descendants | upstream URL/path rewrite, query forwarding, API-key-insensitive result key, offset key, offset tracking, request execution | Implemented in Go | The current Go test baseline locks all four proxy families, API-key key behavior, non-2xx/timeout handling, XML merge/trim, and caches. |
| `IndexerServiceImpl.getSearchTitle` | base implementation returns an empty list | Intentional difference | Go includes MVP basic Sonarr/Radarr expansion; data-backed title/alias expansion is Todo 10. |
| `IndexerServiceImpl.executeFormatRule` | base implementation returns XML unchanged | Intentional difference | Go uses opt-in static formatting before cache insertion; disabled formatters preserve bytes. Full Java rule behavior is Todo 7/10. |
| `SystemConfigServiceImpl` | config query/value lookup/update | Implemented in Go | Update accepts only the active fixed ID/key set, validates structurally without remote Sonarr/Radarr/TMDB/downloader calls, and uses Go RE2 rather than Java regex. Sonarr requires `{title}`, `{season}`, `{episode}`; Radarr requires `{title}`, `{year}`. Language and author values intentionally retain Java-accepted forms except global size/control safety checks. |
| `SystemCacheServiceImpl` | named cache clear and clear-all | Implemented in Go | DB-only management surface; token auth caches remain unavailable until Todo 11. |
| `SonarrRuleServiceImpl` / `RadarrRuleServiceImpl` | rule query/page/sync/validity switch | Implemented in Go | CRUD and validity switch are DB-mode routes with deterministic ordering. HTTP sync contract is implemented but real sync is deferred to Todo9/10 and returns 503 until an adapter is installed. |
| `SonarrExampleServiceImpl` / `RadarrExampleServiceImpl` | example CRUD and formatter examples | Implemented in Go | DB-mode CRUD and deterministic formatter projection; bounded input differs from Java's unbounded acceptance. |
| `SonarrTitleServiceImpl` / `RadarrTitleServiceImpl` | sync, query, title lookup, formatting support | Implemented in Go | DB-mode sync rereads persisted configuration every request and atomically removes stale rows. Atomic replacement intentionally differs from Java upsert-only sync. |
| `TmdbTitleServiceImpl` | TMDB find, sync, page query | Implemented in Go | DB-mode save/query/remove APIs; sync returns 503 until Todo10 installs a real adapter. |
| `SystemUserServiceImpl` | password check, JWT sign/verify/logout, user retrieval/update | Implemented in Go | Todo 11 provides legacy credential migration, expiring HS256 JWTs, bounded revocation, masked user responses, and login-disabled anonymous compatibility. |
| `QbittorrentServiceImpl` | downloader login, file lookup, torrent/file rename | Implemented in Go | Todo 13 provides a revision-aware standard-library client with bounded, redacted HTTP behavior. Periodic login and Sonarr/Radarr rename workflows remain pending in Todo 15. |
| `TransmissionServiceImpl` | RPC login/session, torrent lookup and rename | Implemented in Go | Todo 14 provides a revision-aware standard-library client plus a transparent compatibility proxy. The client supports anonymous or complete Basic Auth credentials, one bounded 409 session-ID replay, typed `session-get`, `torrent-get`, and `torrent-rename-path` operations, frozen-config rename operations, bounded I/O, and redacted errors. The proxy forwards caller data without injecting internal credentials/session state or retrying 409. Periodic login and Sonarr/Radarr rename workflows remain Todo 15. |

## Transmission RPC Protocol Decision

The Go Transmission client implements the legacy bespoke RPC dialect with the
`method`/`arguments`/`result` envelope. The client sends `method` and typed
`arguments`, and accepts only a `result` of `success` plus the operation-specific
`arguments` object. It does not claim support for a particular Transmission
release, does not perform silent protocol negotiation, and does not implement
JSON-RPC 2.0. Any future protocol migration requires explicit version detection
and migration planning.

### Compatibility Routes

| Route | Methods | Notes |
| --- | --- | --- |
| `/sonarr/transmission/transmission/rpc` | GET, POST | Public compatibility proxy; trailing slash variant supported; preserves caller headers/body/status; limits, path traversal, and redirect rules apply. |
| `/radarr/transmission/transmission/rpc` | GET, POST | Public compatibility proxy; trailing slash variant supported; preserves caller headers/body/status; limits, path traversal, and redirect rules apply. |

The proxy uses the current normalized runtime endpoint but remains transparent:
it does not inject internal credentials or session headers and does not perform
automatic 409 retry. Caller authentication and session headers are forwarded
after hop-by-hop header filtering.

### Endpoint Normalization and Security

The runtime Transmission base URL is normalized to its canonical RPC endpoint
through `internal/transmissionconfig`:

- **Java `/transmission/web`** is normalized to **canonical `/transmission/rpc`**.
- **Safe prefixes**: a base URL or safe path prefix gains
  `/transmission/rpc`; an existing `/transmission/web` suffix becomes
  `/transmission/rpc`, and an existing `/transmission/rpc` suffix is retained.

Rejected endpoint forms include:

- **userinfo**: `http://user:pass@host/...` is rejected.
- **query**: `http://host/...?query=value` is rejected.
- **fragment**: `http://host/...#fragment` is rejected.
- **control characters**: any control character in the URL is rejected.
- **ambiguous path**: `..`, `.`, `//`, `\`, or percent-encoded dot segments
  (`%2e`) in the path are rejected.
- **non-HTTP schemes**: `ftp://` or other non-HTTP/HTTPS schemes are rejected.

### Client Credentials and Basic Auth

Credentials are validated through `transmissionconfig.ValidateCredentials`:

- **Anonymous (empty/empty)**: `username=""` and `password=""` is valid. The
  internal client sends no `Authorization` header.
- **Complete pair**: both `username` and `password` must be non-empty and
  control-character-free. The internal client sends Basic Auth on the initial
  attempt and the single 409 replay.
- **Half-missing rejected**: `(username="", password="x")` or
  `(username="x", password="")` fails configuration validation before an
  internal client request is sent.

These configured credentials belong only to the internal client. The public
proxy never substitutes them for caller-provided authentication.

### Client Revision Freezing

Each internal operation captures one runtime endpoint, credential pair, and
revision. Rename lookup and mutation use that same capture. A revision change
during the 409 handshake or between lookup and mutation returns a controlled
configuration-changed error instead of continuing against a replacement
Transmission instance. Independent later operations read the latest snapshot.
Once a newer publication completes, a mutation captured from the prior revision
cannot begin. A mutation already admitted at transport entry may complete using
its captured configuration.

### Numeric Torrent ID and Ack

The Transmission RPC protocol uses numeric torrent IDs and acknowledgments:

- **`torrent-get` fields**: the internal client requests exactly `id`, `name`,
  and `files`; the decoded torrent `id` must be a positive integer.
- **`torrent-rename-path` ack**: internal rename success requires an exact
  positive `id`, `path`, and `name` match. A malformed or mismatched ack cannot
  produce false success.

### Error Boundaries

The internal client distinguishes caller cancellation, deadline/timeout,
transport failure, redirect, status/result failure, malformed response,
unknown torrent, session challenge, configuration change, and request/response
size limits. Request and response payloads are bounded at 1 MiB. Public proxy
requests and responses are bounded at 8 MiB; proxy timeout, redirect, invalid
configuration, and transport/limit failures map to controlled HTTP responses.
Generated errors and Todo 14 evidence do not expose credentials or session IDs.

### Legacy Transmission Dialect Decision

The internal client preserves the legacy Transmission
`method`/`arguments`/`result` dialect and its success result envelope. The public
proxy is byte-transparent and does not interpret or rewrite caller RPC payloads.

### Proxy Transparency

The Transmission proxy is **transparent**:

- **No internal auth injection**: the proxy does not add `Authorization` headers
  or session management.
- **No automatic 409 retry**: the proxy forwards 409 responses to the caller
  without retry logic.
- **Header preservation**: all caller headers are forwarded (minus hop-by-hop
  headers like `Connection`, `Proxy-Connection`).
- **Status preservation**: all upstream status codes are passed through
  unchanged (2xx, 4xx, 5xx).
- **Gzip byte transparency**: the proxy neither negotiates nor decompresses
  upstream gzip responses; their `Content-Encoding` and bytes are preserved.

### Java Stubs vs Go Implementation

`TransmissionServiceImpl` contains source-level placeholders that are not valid
compatibility behavior and have been replaced by the Go client implementation:

| Method | Java behavior | Status |
| --- | --- | --- |
| `files(String hash)` | returns `null` unconditionally | Java stub; replaced by Go `torrent-get` file mapping. Java `null` return is not a compatibility target. |
| `renameFile(String hash, String oldPath, String newPath)` | returns `false` unconditionally | Java stub; replaced by Go `torrent-rename-path` implementation. Java `false` return is not a compatibility target. |

The remaining Transmission methods have failure returns for real protocol or
lookup outcomes. The Go client implements protocol-correct behavior with explicit
testing of session-ID handshake, Basic authentication, request shape, failures,
and redaction.

### Login-Disabled Management Boundary

When login is disabled, DB-mode management routes under `/api/` accept anonymous
requests for Java compatibility. This mode is only compatible with deployment
behind a trusted management network, such as a private bind address, firewall,
or authenticated reverse proxy. It must not be exposed as an open public
management endpoint. The accepted compatibility risk is that an untrusted caller
who can reach these routes may use management writes to configure an upstream or
other runtime target, creating SSRF and configuration-takeover impact.

## qBittorrent Compatibility Routes

The legacy Java Charon constants reserve `/sonarr/qbittorrent` and
`/radarr/qbittorrent`. Go completes these safely as public compatibility
proxies for bounded `GET` and `POST` requests below `/api/v2/`. They use the
current runtime qBittorrent URL, preserve end-to-end request and response data,
and reject traversal, redirects, hop-by-hop headers, oversize messages, and
unconfigured upstreams. The routes do not perform login or use the internal
downloader session. Periodic downloader login and Sonarr/Radarr rename workflows
remain Todo 15 work.

## Todo 14 vs Todo 15 Boundary

Todo 14 covers the Transmission RPC client and compatibility proxy:

- Shared safe endpoint normalization for management writes, SQLite startup,
  client calls, and proxy forwarding.
- Anonymous or complete client credentials, Basic Auth per attempt, and one
  bounded 409 `X-Transmission-Session-Id` replay.
- Legacy `method`/`arguments`/`result` dialect and typed `session-get`,
  `torrent-get`, and `torrent-rename-path` operations.
- Positive torrent IDs, exact rename acknowledgments, and frozen-revision
  lookup/mutation.
- Transparent public proxy routes with no internal auth/session injection or
  automatic 409 retry.
- Typed, bounded, and redacted client/proxy failures.

Todo 15 covers scheduler/workflow integration:

- Periodic Transmission login/session refresh workflows.
- Scheduler-driven sync workflows.
- Sonarr/Radarr event-driven rename workflows.
- Filename policy enforcement.

The public proxy does not implement Todo 15 features. The internal client's
bounded session handshake is available for workflows, but periodic refresh and
event-driven orchestration remain deferred.

Todo 16 remains deferred. It covers broad README, deployment, and operational
documentation beyond this focused compatibility inventory.

Java `files` returning `null` and `renameFile` returning `false` are explicitly
not compatibility targets; they are replaced by the Go client implementation.

## Source Completeness Check

The route table covers every class-level and method-level Spring mapping in
`../jproxy/src/main/java/com/lckp/jproxy/controller/` at the time of this
inventory. It is intentionally updated before each later API migration todo;
an added Java route without a corresponding row is a migration blocker.
