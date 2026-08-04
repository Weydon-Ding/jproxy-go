# Local Podman Test Environment

This document records the local integration environment used for jproxy-go
validation. It intentionally does not store API keys or credentials.

## Stack

Start from the jproxy-go repository root:

```powershell
podman compose -f deployments/podman/compose.yml up -d --build
```

Current service map:

| Service | Container | Host URL | Compose URL |
|---|---|---|---|
| jproxy-go | `jproxy-go-test` | `http://127.0.0.1:8117` | `http://jproxy-go:8117` |
| Prowlarr | `jproxy-test-prowlarr` | `http://127.0.0.1:9696` | `http://prowlarr:9696` |
| Jackett | `jproxy-test-jackett` | `http://127.0.0.1:9117` | `http://jackett:9117` |
| Sonarr | `jproxy-test-sonarr` | `http://127.0.0.1:8989` | `http://sonarr:8989` |
| Radarr | `jproxy-test-radarr` | `http://127.0.0.1:7878` | `http://radarr:7878` |

Check running containers:

```powershell
podman ps --format "{{.Names}} {{.Status}} {{.Ports}}"
```

Health check:

```powershell
curl http://127.0.0.1:8117/health
```

Expected body:

```text
ok
```

## API Keys

Do not paste API keys into docs, commits, or chat logs.

For local one-off verification, read keys from the container config files in the
same shell command that performs the request. Do not print the key.

Prowlarr API example:

```powershell
$cfgText = podman exec jproxy-test-prowlarr cat /config/config.xml
[xml]$cfg = ($cfgText -join "`n")
$key = $cfg.Config.ApiKey
$headers = @{ 'X-Api-Key' = $key }
Invoke-WebRequest -UseBasicParsing -Uri 'http://127.0.0.1:9696/api/v1/indexer' -Headers $headers
```

Sonarr API example:

```powershell
$cfgText = podman exec jproxy-test-sonarr cat /config/config.xml
[xml]$cfg = ($cfgText -join "`n")
$key = $cfg.Config.ApiKey
$headers = @{ 'X-Api-Key' = $key }
Invoke-WebRequest -UseBasicParsing -Uri 'http://127.0.0.1:8989/api/v3/system/status' -Headers $headers
```

Radarr API example:

```powershell
$cfgText = podman exec jproxy-test-radarr cat /config/config.xml
[xml]$cfg = ($cfgText -join "`n")
$key = $cfg.Config.ApiKey
$headers = @{ 'X-Api-Key' = $key }
Invoke-WebRequest -UseBasicParsing -Uri 'http://127.0.0.1:7878/api/v3/system/status' -Headers $headers
```

## Prowlarr-First Route

Prowlarr is the primary route for current validation. Jackett is available in the
stack but is not the current priority.

### Sonarr

Sonarr indexer base URL:

```text
http://jproxy-go:8117/sonarr/prowlarr/1/
```

API path:

```text
/api
```

Verified status:

- Sonarr API returned HTTP 200.
- Sonarr Prowlarr indexer Test passed.
- `Sonarr -> jproxy-go -> Prowlarr -> Mikan` real route passed.

### Radarr

Radarr currently has Prowlarr indexers synced through jproxy-go:

| Indexer | Base URL | API path |
|---|---|---|
| Nyaa.si (Prowlarr) | `http://jproxy-go:8117/radarr/prowlarr/3/` | `/api` |
| YTS (Prowlarr) | `http://jproxy-go:8117/radarr/prowlarr/4/` | `/api` |

Verified status:

- Radarr API returned HTTP 200.
- Radarr has 2 Prowlarr indexers pointing at jproxy-go.
- Both Radarr indexer Test calls returned HTTP 200.
- Radarr release query returned 123 results.

## Manual Verification Commands

### Prowlarr Search Counts

Movie-category search count:

```powershell
$cfgText = podman exec jproxy-test-prowlarr cat /config/config.xml
[xml]$cfg = ($cfgText -join "`n")
$key = $cfg.Config.ApiKey
$headers = @{ 'X-Api-Key' = $key }
$resp = Invoke-WebRequest -UseBasicParsing -Uri 'http://127.0.0.1:9696/api/v1/search?query=test&categories=2000&limit=10&offset=0' -Headers $headers
$items = @((ConvertFrom-Json $resp.Content))
"count=$($items.Count)"
```

Last verified result:

```text
count=135
```

### jproxy-go Radarr/Prowlarr XML Route

Use the Prowlarr API key in-memory and do not print it:

```powershell
$cfgText = podman exec jproxy-test-prowlarr cat /config/config.xml
[xml]$cfg = ($cfgText -join "`n")
$key = [uri]::EscapeDataString($cfg.Config.ApiKey)
foreach ($id in @(3,4)) {
  $url = "http://127.0.0.1:8117/radarr/prowlarr/$id/api?t=search&q=test&cat=2000&limit=10&offset=0&apikey=$key"
  $resp = Invoke-WebRequest -UseBasicParsing -Uri $url
  $body = $resp.Content
  $count = ([regex]::Matches($body, '<item[> ]')).Count
  "id=$id status=$($resp.StatusCode) items=$count"
}
```

Last verified result:

```text
id=3 status=200 items=10
id=4 status=200 items=10
```

### Radarr Indexer Tests

```powershell
$cfgText = podman exec jproxy-test-radarr cat /config/config.xml
[xml]$cfg = ($cfgText -join "`n")
$key = $cfg.Config.ApiKey
$headers = @{ 'X-Api-Key' = $key; 'Content-Type' = 'application/json' }
$idxResp = Invoke-WebRequest -UseBasicParsing -Uri 'http://127.0.0.1:7878/api/v3/indexer' -Headers @{ 'X-Api-Key' = $key }
$items = @((ConvertFrom-Json $idxResp.Content))
foreach ($i in $items) {
  $json = $i | ConvertTo-Json -Depth 20
  $resp = Invoke-WebRequest -UseBasicParsing -Method Post -Uri 'http://127.0.0.1:7878/api/v3/indexer/test' -Headers $headers -Body $json
  "id=$($i.id) name=$($i.name) status=$($resp.StatusCode)"
}
```

Last verified result:

```text
Nyaa.si (Prowlarr): 200
YTS (Prowlarr): 200
```

### Radarr Release Query

```powershell
$cfgText = podman exec jproxy-test-radarr cat /config/config.xml
[xml]$cfg = ($cfgText -join "`n")
$key = $cfg.Config.ApiKey
$headers = @{ 'X-Api-Key' = $key }
$resp = Invoke-WebRequest -UseBasicParsing -Uri 'http://127.0.0.1:7878/api/v3/release?term=test' -Headers $headers
$items = @((ConvertFrom-Json $resp.Content))
"count=$($items.Count)"
```

Last verified result:

```text
count=123
```

## Remaining Validation

| Item | Status | Notes |
|---|---|---|
| Native Docker build/run | Pending | Podman Compose passed, but native Docker still needs explicit validation. |
| Jackett E2E search | Deferred | Jackett is not the current priority. |
| Real XML fixture regression | Pending | Collect representative Prowlarr RSS XML samples for formatter and merge/trim tests. |
