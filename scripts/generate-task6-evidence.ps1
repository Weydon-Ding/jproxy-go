param([string]$OutputPath = ".omo/evidence/task-6-complete-jproxy-migration.txt")
$ErrorActionPreference = "Stop"
$root = (Get-Location).Path
$output = [IO.Path]::GetFullPath((Join-Path $root $OutputPath))
$directory = Split-Path -Parent $output
New-Item -ItemType Directory -Force -Path $directory | Out-Null
$utf8 = [Text.UTF8Encoding]::new($false)
[IO.File]::WriteAllText($output, "timestamp_utc=$([DateTime]::UtcNow.ToString('o'))`ncommit_under_test=$((git rev-parse HEAD).Trim())`ncommit_time=$((git show -s --format=%cI HEAD).Trim())`nevidence_generator=scripts/generate-task6-evidence.ps1`nfocused_count=10`n", $utf8)
function Record([string]$Label, [string]$Command, [string[]]$Arguments, [int]$Timeout = 300) {
  $process = [Diagnostics.Process]::new(); $process.StartInfo.FileName = $Command; $process.StartInfo.Arguments = $Arguments -join ' '; $process.StartInfo.WorkingDirectory = $root; $process.StartInfo.UseShellExecute = $false; $process.StartInfo.RedirectStandardOutput = $true; $process.StartInfo.RedirectStandardError = $true
  $start=[DateTime]::UtcNow; [void]$process.Start(); $stdout=$process.StandardOutput.ReadToEndAsync(); $stderr=$process.StandardError.ReadToEndAsync()
  if(-not $process.WaitForExit($Timeout*1000)){ $process.Kill($true); throw "timeout: $Label" }; [Threading.Tasks.Task]::WaitAll(@($stdout,$stderr)); $out=$stdout.GetAwaiter().GetResult(); $err=$stderr.GetAwaiter().GetResult()
  [IO.File]::AppendAllText($output,"--- record ---`nlabel=$Label`ncommand=$Command $($Arguments -join ' ')`nexit_code=$($process.ExitCode)`nstdout:`n$out`nstderr:`n$err`n",$utf8); if($process.ExitCode -ne 0){throw "failed: $Label"}
}
Record "route_matrix_http_and_fallbacks" "go" @("test","./internal/api/system","-count=10","-shuffle=on","-v") 300
Record "full_test" "go" @("test","./...","-count=1","-shuffle=on") 300
Record "build" "go" @("build","./cmd/jproxy") 180
Record "vet" "go" @("vet","./...") 180
Record "lint" "golangci-lint" @("run") 180
Record "diff_check" "git" @("diff","--check") 60
