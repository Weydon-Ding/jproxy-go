param(
    [string]$OutputPath = ".omo/evidence/task-4-complete-jproxy-migration.txt",
    [ValidateRange(1, [int]::MaxValue)]
    [int]$StabilityCount = 10
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

function Add-EvidenceText {
    param([string]$Text)

    [System.IO.File]::AppendAllText($output, $Text + [Environment]::NewLine, $utf8)
}

function Add-EvidenceRecord {
    param([pscustomobject]$Record)

    $recordText = @(
        "--- record ---"
        "timestamp_utc=$($Record.Timestamp)"
        "label=$($Record.Label)"
        "command=$($Record.Command)"
        "pid=$($Record.Pid)"
        "exit_code=$($Record.ExitCode)"
        "gate_result=$($Record.GateResult)"
        "stdout:"
        $Record.Stdout
        "stderr:"
        $Record.Stderr
    ) -join [Environment]::NewLine
    Add-EvidenceText $recordText
}

function Test-RaceToolchainUnavailable {
    param([string]$Stdout, [string]$Stderr)

    $combined = $Stdout + [Environment]::NewLine + $Stderr
    return $combined -match '(?is)(gcc.*not found|C compiler .*not found|cgo:.*not found|CGO.*requires.*C compiler)'
}

function Invoke-EvidenceCommand {
    param(
        [string]$Label,
        [string]$Command,
        [string[]]$Arguments,
        [ValidateRange(1, [int]::MaxValue)]
        [int]$TimeoutSeconds,
        [hashtable]$Environment = @{},
        [switch]$RaceGate
    )

    $started = [DateTime]::UtcNow
    Write-Host "evidence_start label=$Label timeout_seconds=$TimeoutSeconds"
    $process = [System.Diagnostics.Process]::new()
    $process.StartInfo.FileName = $Command
    $process.StartInfo.UseShellExecute = $false
    $process.StartInfo.RedirectStandardOutput = $true
    $process.StartInfo.RedirectStandardError = $true
    $process.StartInfo.WorkingDirectory = $root
    foreach ($argument in $Arguments) {
        [void]$process.StartInfo.ArgumentList.Add($argument)
    }
    foreach ($key in $Environment.Keys) {
        $process.StartInfo.Environment[$key] = $Environment[$key]
    }

    $processId = "not_started"
    $stdoutText = ""
    $stderrText = ""
    $exitCode = "start_failed"
    $gateResult = "failed"
    try {
        if (-not $process.Start()) {
            throw "process did not start"
        }
        $processId = $process.Id
        $stdout = $process.StandardOutput.ReadToEndAsync()
        $stderr = $process.StandardError.ReadToEndAsync()
        if ($process.WaitForExit($TimeoutSeconds * 1000)) {
            $exitCode = $process.ExitCode
        } else {
            try {
                $process.Kill($true)
            } catch {
                $stderrText += "terminate_error=$($_.Exception.Message)" + [Environment]::NewLine
            }
            if ($process.WaitForExit(5000)) {
                $exitCode = "timeout"
            } else {
                $exitCode = "cleanup_timeout"
                $stderrText += "cleanup_error=process tree did not exit within 5 seconds" + [Environment]::NewLine
            }
        }
        if ($process.HasExited -and [System.Threading.Tasks.Task]::WaitAll(@($stdout, $stderr), 5000)) {
            $stdoutText += $stdout.GetAwaiter().GetResult()
            $stderrText += $stderr.GetAwaiter().GetResult()
        } else {
            $exitCode = "cleanup_timeout"
            $stderrText += "cleanup_error=process or output streams did not exit within 5 seconds" + [Environment]::NewLine
        }
    } catch {
        $stderrText += "start_or_capture_error=$($_.Exception.Message)" + [Environment]::NewLine
    } finally {
        $process.Dispose()
    }

    if ($RaceGate -and $exitCode -ne 0 -and (Test-RaceToolchainUnavailable $stdoutText $stderrText)) {
        $gateResult = "unavailable"
    } elseif ($exitCode -eq 0) {
        $gateResult = "passed"
    } else {
        $script:RequiredFailure = $true
    }
    $elapsed = ([DateTime]::UtcNow - $started).TotalSeconds.ToString("F2", [System.Globalization.CultureInfo]::InvariantCulture)
    $record = [PSCustomObject]@{
        Timestamp = $started.ToString("o")
        Label = $Label
        Command = ($Command + " " + ($Arguments -join " "))
        Pid = $processId
        ExitCode = $exitCode
        GateResult = $gateResult
        Stdout = $stdoutText
        Stderr = $stderrText
    }
    Add-EvidenceRecord $record
    Write-Host "evidence_finish label=$Label timeout_seconds=$TimeoutSeconds elapsed_seconds=$elapsed exit=$exitCode result=$gateResult"
}

[System.IO.File]::WriteAllText($output, (@(
    "timestamp_utc=$([DateTime]::UtcNow.ToString('o'))"
    "cwd=$root"
    "platform=$([System.Environment]::OSVersion.Platform)/$([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture)"
    "evidence_generator=scripts/generate-task4-evidence.ps1"
    "stability_count=$StabilityCount"
    "stability_count_note=10 is the formal Todo 4 gate; use -StabilityCount 1 only for fast debugging."
) -join [Environment]::NewLine) + [Environment]::NewLine, $utf8)

Invoke-EvidenceCommand "process_scenarios" "go" @("test", "./cmd/jproxy", "-run", "TestProcess_", "-count=1", "-v") 180
Invoke-EvidenceCommand "stability_gate" "go" @("test", "./cmd/jproxy", "./internal/app", "./internal/store/sqlite", "-count=$StabilityCount", "-shuffle=on") 900
Invoke-EvidenceCommand "full_test" "go" @("test", "./...", "-count=1", "-shuffle=on") 300
Invoke-EvidenceCommand "build" "go" @("build", "./cmd/jproxy") 180
Invoke-EvidenceCommand "vet" "go" @("vet", "./...") 180
Invoke-EvidenceCommand "lint" "golangci-lint" @("run") 180
Invoke-EvidenceCommand "diff_check" "git" @("diff", "--check") 60 @{ GIT_MASTER = "1" }
Invoke-EvidenceCommand "race" "go" @("test", "-race", "./...", "-count=1", "-shuffle=on") 300 @{ CGO_ENABLED = "1" } -RaceGate

if ($script:RequiredFailure) {
    throw "one or more required evidence commands failed; inspect the generated records"
}

exit 0
