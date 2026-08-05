param(
  [Parameter(Mandatory = $true)][string]$Path,
  [switch]$SelfTest
)

$ErrorActionPreference = "Stop"
$markers = @(
  "task-canary-db-path",
  "task-canary-dsn",
  "task-canary-api-key",
  "task-canary-password",
  "task-canary-token",
  "task-canary-url",
  "task-canary-regex",
  "task-canary-xml",
  "task-canary-upload-name"
)

function Test-SanitizedEvidence([string]$Target) {
  $content = [IO.File]::ReadAllText($Target)
  foreach ($marker in $markers) {
    if ($content.Contains($marker, [StringComparison]::Ordinal)) {
      throw "sensitive marker found in sanitized evidence"
    }
  }
}

if ($SelfTest) {
  $clean = Join-Path ([IO.Path]::GetTempPath()) ("jproxy-redaction-clean-" + [Guid]::NewGuid() + ".txt")
  $leak = Join-Path ([IO.Path]::GetTempPath()) ("jproxy-redaction-leak-" + [Guid]::NewGuid() + ".txt")
  try {
    [IO.File]::WriteAllText($clean, "canary_leaks=0")
    Test-SanitizedEvidence $clean
    [IO.File]::WriteAllText($leak, "task-canary-token")
    $detected = $false
    try { Test-SanitizedEvidence $leak } catch { $detected = $true }
    if (-not $detected) { throw "injected marker was not rejected" }
  } finally {
    Remove-Item -LiteralPath $clean,$leak -Force -ErrorAction SilentlyContinue
  }
}

Test-SanitizedEvidence $Path
