param(
    [string]$OutputPath = ".omo/evidence/task-4-complete-jproxy-migration.txt",
    [int]$StabilityCount = 10
)

$ErrorActionPreference = "Stop"
$root = (Get-Location).Path
$output = Join-Path $root $OutputPath
$directory = Split-Path -Parent $output
if (-not (Test-Path -LiteralPath $directory)) {
    New-Item -ItemType Directory -Path $directory | Out-Null
}

function Invoke-EvidenceCommand {
    param(
        [string]$Command,
        [string[]]$Arguments,
        [int]$TimeoutSeconds = 180,
        [hashtable]$Environment = @{}
    )

    $start = [DateTime]::UtcNow
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
    [void]$process.Start()
    $stdout = $process.StandardOutput.ReadToEndAsync()
    $stderr = $process.StandardError.ReadToEndAsync()
    if (-not $process.WaitForExit($TimeoutSeconds * 1000)) {
        $process.Kill($true)
        $process.WaitForExit()
        $exitCode = "timeout"
    } else {
        $exitCode = $process.ExitCode
    }
    [PSCustomObject]@{
        Timestamp = $start.ToString("o")
        Command = ($Command + " " + ($Arguments -join " "))
        ExitCode = $exitCode
        Stdout = $stdout.GetAwaiter().GetResult()
        Stderr = $stderr.GetAwaiter().GetResult()
    }
}

$records = @(
    Invoke-EvidenceCommand "go" @("test", "./cmd/jproxy", "-run", "TestProcess_", "-count=1", "-v") 180
    Invoke-EvidenceCommand "go" @("test", "./cmd/jproxy", "./internal/app", "./internal/store/sqlite", "-count=$StabilityCount", "-shuffle=on") 900
    Invoke-EvidenceCommand "go" @("test", "./...", "-count=1", "-shuffle=on") 300
    Invoke-EvidenceCommand "go" @("build", "./cmd/jproxy") 180
    Invoke-EvidenceCommand "go" @("vet", "./...") 180
    Invoke-EvidenceCommand "golangci-lint" @("run") 180
    Invoke-EvidenceCommand "git" @("diff", "--check") 60 @{ GIT_MASTER = "1" }
    Invoke-EvidenceCommand "go" @("test", "-race", "./...", "-count=1", "-shuffle=on") 300 @{ CGO_ENABLED = "1" }
)

$header = @(
    "timestamp_utc=$([DateTime]::UtcNow.ToString('o'))"
    "cwd=$root"
    "platform=$([System.Environment]::OSVersion.Platform)/$([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture)"
    "evidence_generator=scripts/generate-task4-evidence.ps1"
)
$content = [System.Collections.Generic.List[string]]::new()
$content.AddRange([string[]]$header)
foreach ($record in $records) {
    $content.Add("--- record ---")
    $content.Add("timestamp_utc=$($record.Timestamp)")
    $content.Add("command=$($record.Command)")
    $content.Add("exit_code=$($record.ExitCode)")
    $content.Add("stdout:")
    $content.Add($record.Stdout)
    $content.Add("stderr:")
    $content.Add($record.Stderr)
}
[System.IO.File]::WriteAllLines($output, $content)
