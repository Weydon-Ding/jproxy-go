param(
    [string]$OutputPath = ".omo/evidence/task-5-complete-jproxy-migration.txt"
)

$ErrorActionPreference = "Stop"
$root = (Get-Location).Path
$output = if ([System.IO.Path]::IsPathRooted($OutputPath)) {
    [System.IO.Path]::GetFullPath($OutputPath)
} else {
    [System.IO.Path]::GetFullPath((Join-Path $root $OutputPath))
}
$directory = Split-Path -Parent $output
if (-not (Test-Path -LiteralPath $directory -PathType Container)) {
    New-Item -ItemType Directory -Path $directory -Force | Out-Null
}

$utf8 = [System.Text.UTF8Encoding]::new($false)
$script:RequiredFailure = $false

function Add-EvidenceRecord {
    param(
        [string]$Label,
        [string]$Command,
        [string[]]$Arguments,
        [hashtable]$Environment = @{},
        [switch]$RaceGate
    )

    $process = [System.Diagnostics.Process]::new()
    $process.StartInfo.FileName = $Command
    $process.StartInfo.WorkingDirectory = $root
    $process.StartInfo.UseShellExecute = $false
    $process.StartInfo.RedirectStandardOutput = $true
    $process.StartInfo.RedirectStandardError = $true
    foreach ($argument in $Arguments) {
        [void]$process.StartInfo.ArgumentList.Add($argument)
    }
    foreach ($key in $Environment.Keys) {
        $process.StartInfo.Environment[$key] = $Environment[$key]
    }

    $started = [DateTime]::UtcNow
    $process.Start() | Out-Null
    $stdout = $process.StandardOutput.ReadToEndAsync()
    $stderr = $process.StandardError.ReadToEndAsync()
    $process.WaitForExit()
    [System.Threading.Tasks.Task]::WaitAll(@($stdout, $stderr))
    $stdoutText = $stdout.GetAwaiter().GetResult()
    $stderrText = $stderr.GetAwaiter().GetResult()
    $gateResult = "passed"
    $combined = $stdoutText + [Environment]::NewLine + $stderrText
    if ($RaceGate -and $process.ExitCode -ne 0 -and $combined -match '(?is)(gcc.*not found|C compiler .*not found|cgo:.*not found|CGO.*requires.*C compiler)') {
        $gateResult = "unavailable"
    } elseif ($process.ExitCode -ne 0) {
        $gateResult = "failed"
        $script:RequiredFailure = $true
    }

    $record = @(
        "--- record ---"
        "timestamp_utc=$($started.ToString('o'))"
        "label=$Label"
        "command=$Command $($Arguments -join ' ')"
        "exit_code=$($process.ExitCode)"
        "gate_result=$gateResult"
        "stdout:"
        $stdoutText
        "stderr:"
        $stderrText
    ) -join [Environment]::NewLine
    [System.IO.File]::AppendAllText($output, $record + [Environment]::NewLine, $utf8)
    $process.Dispose()
}

[System.IO.File]::WriteAllText($output, (@(
    "timestamp_utc=$([DateTime]::UtcNow.ToString('o'))"
    "cwd=$root"
    "evidence_generator=scripts/generate-task5-evidence.ps1"
) -join [Environment]::NewLine) + [Environment]::NewLine, $utf8)

Add-EvidenceRecord "runtime_proxy_regressions" "go" @("test", "./internal/runtime", "./internal/proxy", "-count=1", "-v")
Add-EvidenceRecord "full_test" "go" @("test", "./...", "-count=1", "-shuffle=on")
Add-EvidenceRecord "build" "go" @("build", "./cmd/jproxy")
Add-EvidenceRecord "vet" "go" @("vet", "./...")
Add-EvidenceRecord "lint" "golangci-lint" @("run")
Add-EvidenceRecord "diff_check" "git" @("diff", "--check") @{ GIT_MASTER = "1" }
Add-EvidenceRecord "race" "go" @("test", "-race", "./...", "-count=1", "-shuffle=on") @{ CGO_ENABLED = "1" } -RaceGate

if ($script:RequiredFailure) {
    throw "one or more required evidence commands failed; inspect the generated records"
}
