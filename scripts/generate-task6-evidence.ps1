param(
    [string]$OutputPath = ".omo/evidence/task-6-complete-jproxy-migration.txt"
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
if (-not (Test-Path -LiteralPath $directory -PathType Container)) {
    throw "evidence output directory was not created"
}

$utf8 = [System.Text.UTF8Encoding]::new($false)
$script:RequiredFailure = $false

function Add-EvidenceText([string]$Text) {
    [System.IO.File]::AppendAllText($output, $Text + [Environment]::NewLine, $utf8)
}

function Test-RaceToolchainUnavailable([string]$Stdout, [string]$Stderr) {
    ($Stdout + [Environment]::NewLine + $Stderr) -match '(?is)(gcc.*not found|C compiler .*not found|cgo:.*not found|CGO.*requires.*C compiler)'
}

function Invoke-EvidenceCommand {
    param(
        [string]$Label,
        [string]$Command,
        [string[]]$Arguments,
        [int]$TimeoutSeconds = 300,
        [hashtable]$Environment = @{},
        [switch]$RaceGate
    )

    $started = [DateTime]::UtcNow
    Write-Host "evidence_start label=$Label timeout_seconds=$TimeoutSeconds"
    $process = [System.Diagnostics.Process]::new()
    $process.StartInfo.FileName = $Command
    $process.StartInfo.WorkingDirectory = $root
    $process.StartInfo.UseShellExecute = $false
    $process.StartInfo.RedirectStandardOutput = $true
    $process.StartInfo.RedirectStandardError = $true
    foreach ($argument in $Arguments) { [void]$process.StartInfo.ArgumentList.Add($argument) }
    foreach ($key in $Environment.Keys) { $process.StartInfo.Environment[$key] = $Environment[$key] }

    $stdoutText = ""
    $stderrText = ""
    $exitCode = "start_failed"
    $processId = "not_started"
    try {
        if (-not $process.Start()) { throw "process did not start" }
        $processId = $process.Id
        $stdout = $process.StandardOutput.ReadToEndAsync()
        $stderr = $process.StandardError.ReadToEndAsync()
        if ($process.WaitForExit($TimeoutSeconds * 1000)) {
            $exitCode = $process.ExitCode
        } else {
            $process.Kill($true)
            if ($process.WaitForExit(5000)) { $exitCode = "timeout" } else { $exitCode = "cleanup_timeout" }
        }
        if ([System.Threading.Tasks.Task]::WaitAll(@($stdout, $stderr), 5000)) {
            $stdoutText = $stdout.GetAwaiter().GetResult()
            $stderrText = $stderr.GetAwaiter().GetResult()
        } else {
            $exitCode = "cleanup_timeout"
            $stderrText = "output streams did not exit within 5 seconds"
        }
    } catch {
        $stderrText += $_.Exception.Message
    } finally {
        $process.Dispose()
    }

    $gateResult = if ($RaceGate -and $exitCode -ne 0 -and (Test-RaceToolchainUnavailable $stdoutText $stderrText)) {
        "unavailable"
    } elseif ($exitCode -eq 0) {
        "passed"
    } else {
        $script:RequiredFailure = $true
        "failed"
    }
    Add-EvidenceText (@(
        "--- record ---", "timestamp_utc=$($started.ToString('o'))", "label=$Label",
        "command=$Command $($Arguments -join ' ')", "pid=$processId", "exit_code=$exitCode", "gate_result=$gateResult",
        "stdout:", $stdoutText, "stderr:", $stderrText
    ) -join [Environment]::NewLine)
    Write-Host "evidence_finish label=$Label exit=$exitCode result=$gateResult"
}

[System.IO.File]::WriteAllText($output, (@(
    "timestamp_utc=$([DateTime]::UtcNow.ToString('o'))", "cwd=$root",
    "commit_under_test=$((git rev-parse HEAD).Trim())", "commit_time=$((git show -s --format=%cI HEAD).Trim())",
    "evidence_generator=scripts/generate-task6-evidence.ps1", "focused_count=10"
) -join [Environment]::NewLine) + [Environment]::NewLine, $utf8)

Invoke-EvidenceCommand "task6_qa" "go" @("test", "./internal/api/system", "./internal/app", "-run", "TestTask6", "-count=1", "-v") 180
Invoke-EvidenceCommand "focused_gate" "go" @("test", "./internal/api/system", "./internal/app", "./internal/runtime", "./internal/store/sqlite", "./internal/proxy", "-count=10", "-shuffle=on", "-v") 900
Invoke-EvidenceCommand "full_test" "go" @("test", "./...", "-count=1", "-shuffle=on") 300
Invoke-EvidenceCommand "build" "go" @("build", "./cmd/jproxy") 180
Invoke-EvidenceCommand "vet" "go" @("vet", "./...") 180
Invoke-EvidenceCommand "lint" "golangci-lint" @("run") 180
Invoke-EvidenceCommand "diff_check" "git" @("diff", "--check") 60 @{ GIT_MASTER = "1" }
Invoke-EvidenceCommand "race" "go" @("test", "-race", "./...", "-count=1", "-shuffle=on") 300 @{ CGO_ENABLED = "1" } -RaceGate

if ($script:RequiredFailure) { throw "one or more required evidence commands failed; inspect the generated records" }
