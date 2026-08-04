# Rule Formatting Migration Plan

## Goal

Migrate the Java JProxy rule-formatting behavior into jproxy-go without pulling
the full Java database/UI stack into the first step.

The first implementation target is a Radarr-first MVP because the current local
test stack has already verified this path:

```text
Radarr -> jproxy-go -> Prowlarr -> movie indexers
```

## Current Java Reference

Important Java source locations in `../jproxy`:

| Area | File | Notes |
|---|---|---|
| XML merge/count/trim | `src/main/java/com/lckp/jproxy/util/XmlUtil.java` | Go already has equivalent MVP behavior in `internal/proxy`. |
| Format helpers | `src/main/java/com/lckp/jproxy/util/FormatUtil.java` | `cleanTitle`, token replacement, token removal, numeric offset. |
| Base proxy service | `src/main/java/com/lckp/jproxy/service/impl/IndexerServiceImpl.java` | Default `executeFormatRule` returns XML unchanged. |
| Radarr formatting | `src/main/java/com/lckp/jproxy/service/impl/RadarrIndexerServiceImpl.java` | Parses XML, rewrites each `<item><title>`. |
| Sonarr formatting | `src/main/java/com/lckp/jproxy/service/impl/SonarrIndexerServiceImpl.java` | Similar concept; migrate after Radarr MVP. |

The Java Radarr flow is:

```text
XML response
-> parse RSS channel item elements
-> read item title and optional description
-> match title against known Radarr title data
-> apply token rules and format template
-> replace item title
-> return XML
```

## Migration Principle

Do not copy the Java class graph one-to-one.

Keep the same behavior concepts, but implement them as small Go packages:

| Java concept | Go target |
|---|---|
| `FormatUtil` static helpers | `internal/format` pure functions |
| `executeFormatRule(xml)` | formatter called from `internal/proxy` before caching/writing |
| `RadarrRule` / `SonarrRule` DB entities | JSON/config structs first; SQLite later |
| `systemConfig` format templates | environment/config file first; DB later |
| title services | simple in-memory test fixtures first; title library later |

## Phase 1: Radarr XML Title Formatting MVP

Status: implemented as an opt-in static configuration MVP.

Scope:

1. Add `internal/format` package.
2. Implement pure helpers equivalent to the small Java `FormatUtil` subset:
   - clean title text.
   - replace `{token}` with a value.
   - remove unknown tokens.
   - apply numeric offset in token values.
3. Add an XML formatter that rewrites `<item><title>` values.
4. Use an explicit, static config shape in tests.
5. Wire the formatter into `proxy.Server` behind a disabled-by-default config.

Non-goals:

- No SQLite.
- No Java DB compatibility.
- No Web UI.
- No title sync.
- No remote rule sync.
- No Sonarr formatting yet unless the Radarr shape proves reusable with no scope creep.

Acceptance criteria:

- Existing proxy behavior is unchanged when formatting is disabled.
- A fixture RSS XML with multiple items is rewritten when formatting is enabled.
- XML remains valid enough for Radarr/Prowlarr consumers.
- Unknown tokens are removed or left according to the documented MVP rule.
- Tests cover title-only and title-plus-description input.

Phase 1 implementation notes:

- Configuration is disabled by default and read only from the documented
  `JPROXY_RADARR_*` environment variables.
- Rules and optional static titles are JSON arrays parsed and validated at
  process startup. Invalid enabled configuration fails startup.
- Static titles support only `mainTitle`, `title`, `cleanTitle`, and `year` for
  `{cleanTitle}` matching. There is no title synchronization, database access,
  UI, or remote rule source.
- The formatter runs only for Radarr on a result-cache miss, after expanded
  search/merge/trim and before cache insertion. Cached XML is already formatted;
  disabled and Sonarr paths do not parse or rewrite XML.
- Successful formatting uses `encoding/xml`; unsupported input, invalid XML,
  a blank template, a template without `{title}`, or an unmatched title returns
  the original XML string unchanged.

## Phase 2: Config File Rules

Add local rule files after the pure formatter is stable:

```text
configs/rules/radarr.json
configs/rules/sonarr.json
```

Suggested config shape:

```json
{
  "enabled": true,
  "format": "{title} {year} {resolution}",
  "cleanTitleRegex": "...",
  "tokens": {
    "resolution": [
      { "regex": "2160p", "value": "2160p" },
      { "regex": "1080p", "value": "1080p" }
    ]
  }
}
```

Keep this file format intentionally small. It is not a public compatibility
contract until a release note says so.

## Phase 3: Sonarr Formatting

After Radarr is proven with real Prowlarr results, add Sonarr support using the
same formatter primitives. Keep Sonarr-specific rules in separate config so TV
and movie behavior can diverge without branching deeply in the formatter.

## Phase 4: Title Library and Database Compatibility

Only after the formatter works from static config:

1. Evaluate the Java SQLite schema.
2. Decide whether jproxy-go reads the Java DB directly or imports data into its
   own simpler format.
3. Add title matching and aliases.
4. Add sync tasks if still needed.

## Suggested First Implementation Task

```text
feat(format): add Radarr XML title formatter
```

Files likely involved:

| File | Purpose |
|---|---|
| `internal/format/*.go` | Pure formatting helpers and XML title rewrite logic. |
| `internal/format/*_test.go` | Unit tests with RSS XML fixtures. |
| `internal/config/config.go` | Disabled-by-default formatting config toggle. |
| `internal/proxy/server.go` | Call formatter before caching/writing response. |
| `README.md` | Document the disabled-by-default MVP toggle if exposed. |

Verification commands:

```powershell
go test ./internal/format ./internal/proxy ./internal/config -count=1
go test ./...
go build ./cmd/jproxy
```
