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
| `SystemConfigController` | `GET /api/system/config/query` | `query` | Intentional difference | DB mode only; returns the raw ID-sorted array and deliberately has no Java rename-task side effect. |
| `SystemConfigController` | `POST /api/system/config/update` | `update` | Implemented in Go | DB mode only; strict complete 20-row update, Java formatter token sets, one SQLite transaction and one prepared publication of formatter plus current Jackett/Prowlarr upstream URLs. |
| `SystemConfigController` | `GET /api/system/config/author/list` | `listAuthor` | Implemented in Go | DB mode only; every request attempts Java production primary `https://raw.githubusercontent.com/LuckyPuppy514/jproxy/main/src/main/resources/rule/author.json`, then backup `https://github.rn.lckp.top/LuckyPuppy514/jproxy/main/src/main/resources/rule/author.json`, otherwise `LuckyPuppy514`. |
| `SystemCacheController` | `POST /api/system/cache/clearAll` | `clearAll` | Implemented in Go | DB mode only; refreshes the provider before clearing current named caches. |
| `SystemCacheController` | `POST /api/system/cache/clear` | `clear` | Implemented in Go | DB mode only; unknown or blank names are strict 400 with no effects. |
| `SystemUserController` | `POST /api/system/user/login` | `login` | Pending migration | Todo 11 |
| `SystemUserController` | `GET /api/system/user/info` | `info` | Pending migration | Todo 11 |
| `SystemUserController` | `POST /api/system/user/update` | `update` | Pending migration | Todo 11 |
| `SystemUserController` | `POST /api/system/user/logout` | `logout` | Pending migration | Todo 11 |
| `SystemUserController` | `GET /api/system/user/isLoginEnabled` | `isLoginEnabled` | Pending migration | Todo 11 |
| `SonarrRuleController` | `POST /api/sonarr/rule/sync` | `sync` | Pending migration | Todo 7 |
| `SonarrRuleController` | `GET /api/sonarr/rule/query` | `query` | Pending migration | Todo 7 |
| `SonarrRuleController` | `POST /api/sonarr/rule/save` | `save` | Pending migration | Todo 7 |
| `SonarrRuleController` | `POST /api/sonarr/rule/remove` | `remove` | Pending migration | Todo 7 |
| `SonarrRuleController` | `POST /api/sonarr/rule/enable` | `enable` | Pending migration | Todo 7 |
| `SonarrRuleController` | `POST /api/sonarr/rule/disable` | `disable` | Pending migration | Todo 7 |
| `SonarrRuleController` | `POST /api/sonarr/rule/export` | `export` | Pending migration | Todo 7 |
| `SonarrRuleController` | `POST /api/sonarr/rule/import` | `importSonarrRule` | Pending migration | Todo 7 |
| `SonarrRuleController` | `GET /api/sonarr/rule/token/list` | `listToken` | Pending migration | Todo 7 |
| `RadarrRuleController` | `POST /api/radarr/rule/sync` | `sync` | Pending migration | Todo 7 |
| `RadarrRuleController` | `GET /api/radarr/rule/query` | `query` | Pending migration | Todo 7 |
| `RadarrRuleController` | `POST /api/radarr/rule/save` | `save` | Pending migration | Todo 7 |
| `RadarrRuleController` | `POST /api/radarr/rule/remove` | `remove` | Pending migration | Todo 7 |
| `RadarrRuleController` | `POST /api/radarr/rule/enable` | `enable` | Pending migration | Todo 7 |
| `RadarrRuleController` | `POST /api/radarr/rule/disable` | `disable` | Pending migration | Todo 7 |
| `RadarrRuleController` | `POST /api/radarr/rule/export` | `export` | Pending migration | Todo 7 |
| `RadarrRuleController` | `POST /api/radarr/rule/import` | `importRadarrRule` | Pending migration | Todo 7 |
| `RadarrRuleController` | `GET /api/radarr/rule/token/list` | `listToken` | Pending migration | Todo 7 |
| `SonarrExampleController` | `POST /api/sonarr/example/save` | `save` | Pending migration | Todo 7 |
| `SonarrExampleController` | `GET /api/sonarr/example/query` | `query` | Pending migration | Todo 7 |
| `SonarrExampleController` | `POST /api/sonarr/example/remove` | `remove` | Pending migration | Todo 7 |
| `RadarrExampleController` | `POST /api/radarr/example/save` | `save` | Pending migration | Todo 7 |
| `RadarrExampleController` | `GET /api/radarr/example/query` | `query` | Pending migration | Todo 7 |
| `RadarrExampleController` | `POST /api/radarr/example/remove` | `remove` | Pending migration | Todo 7 |
| `SonarrTitleController` | `POST /api/sonarr/title/sync` | `sync` | Pending migration | Todo 8/9 |
| `SonarrTitleController` | `GET /api/sonarr/title/query` | `query` | Pending migration | Todo 8 |
| `SonarrTitleController` | `POST /api/sonarr/title/remove` | `remove` | Pending migration | Todo 8 |
| `RadarrTitleController` | `POST /api/radarr/title/sync` | `sync` | Pending migration | Todo 8/9 |
| `RadarrTitleController` | `GET /api/radarr/title/query` | `query` | Pending migration | Todo 8 |
| `RadarrTitleController` | `POST /api/radarr/title/remove` | `remove` | Pending migration | Todo 8 |
| `TmdbTitleController` | `POST /api/tmdb/title/sync` | `sync` | Pending migration | Todo 8/10 |
| `TmdbTitleController` | `GET /api/tmdb/title/query` | `query` | Pending migration | Todo 8 |
| `TmdbTitleController` | `POST /api/tmdb/title/remove` | `remove` | Pending migration | Todo 8 |
| `TmdbTitleController` | `POST /api/tmdb/title/save` | `save` | Pending migration | Todo 8 |
| `RuleController` | `GET /api/rule/test` | `test` | Pending migration | Todo 7 |

## Java Service Behavior Inventory

| Java service | Relevant public behavior | Status in Go | Planned migration / compatibility note |
| --- | --- | --- | --- |
| `IndexerServiceImpl` and Sonarr/Radarr Jackett/Prowlarr descendants | upstream URL/path rewrite, query forwarding, API-key-insensitive result key, offset key, offset tracking, request execution | Implemented in Go | The current Go test baseline locks all four proxy families, API-key key behavior, non-2xx/timeout handling, XML merge/trim, and caches. |
| `IndexerServiceImpl.getSearchTitle` | base implementation returns an empty list | Intentional difference | Go includes MVP basic Sonarr/Radarr expansion; data-backed title/alias expansion is Todo 10. |
| `IndexerServiceImpl.executeFormatRule` | base implementation returns XML unchanged | Intentional difference | Go uses opt-in static formatting before cache insertion; disabled formatters preserve bytes. Full Java rule behavior is Todo 7/10. |
| `SystemConfigServiceImpl` | config query/value lookup/update | Implemented in Go | Update accepts only the active fixed ID/key set, validates structurally without remote Sonarr/Radarr/TMDB/downloader calls, and uses Go RE2 rather than Java regex. Sonarr requires `{title}`, `{season}`, `{episode}`; Radarr requires `{title}`, `{year}`. Language and author values intentionally retain Java-accepted forms except global size/control safety checks. |
| `SystemCacheServiceImpl` | named cache clear and clear-all | Implemented in Go | DB-only management surface; token auth caches remain unavailable until Todo 11. |
| `SonarrRuleServiceImpl` / `RadarrRuleServiceImpl` | rule query/page/sync/validity switch | Pending migration | Todo 7 and Todo 10 for remote sync. |
| `SonarrExampleServiceImpl` / `RadarrExampleServiceImpl` | example CRUD and formatter examples | Pending migration | Todo 7. |
| `SonarrTitleServiceImpl` / `RadarrTitleServiceImpl` | sync, query, title lookup, formatting support | Pending migration | Static formatter input exists today; persisted titles, sync, and full format behavior are Todos 3, 8, and 9. |
| `TmdbTitleServiceImpl` | TMDB find, sync, page query | Pending migration | Todos 8 and 10. |
| `SystemUserServiceImpl` | password check, JWT sign/verify/logout, user retrieval/update | Pending migration | Todo 11. |
| `QbittorrentServiceImpl` | downloader login, file lookup, torrent/file rename | Pending migration | Todo 13. |
| `TransmissionServiceImpl` | RPC login/session, torrent rename | Pending migration | Todo 14 must implement protocol-correct behavior rather than Java placeholders. |

## Transmission Java Stubs

`TransmissionServiceImpl` contains source-level placeholders that are not valid
compatibility behavior and must be replaced, not preserved:

| Method | Java behavior | Status |
| --- | --- | --- |
| `files(String hash)` | returns `null` unconditionally | Java stub; Todo 14 must implement `torrent-get` file mapping. |
| `renameFile(String hash, String oldPath, String newPath)` | returns `false` unconditionally | Java stub; Todo 14 must implement protocol-correct file/path rename. |

The remaining Transmission methods have failure returns for real protocol or
lookup outcomes; they are not classified as unconditional stubs. Todo 14 will
test the session-ID handshake, Basic authentication, request shape, failures,
and redaction explicitly.

## Source Completeness Check

The route table covers every class-level and method-level Spring mapping in
`../jproxy/src/main/java/com/lckp/jproxy/controller/` at the time of this
inventory. It is intentionally updated before each later API migration todo;
an added Java route without a corresponding row is a migration blocker.
