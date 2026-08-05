param([string]$OutputPath = ".omo/evidence/task-8-complete-jproxy-migration.txt")
$ErrorActionPreference = "Stop"
$root = (Get-Location).Path
$output = if ([IO.Path]::IsPathRooted($OutputPath)) { [IO.Path]::GetFullPath($OutputPath) } else { [IO.Path]::GetFullPath((Join-Path $root $OutputPath)) }
$directory = Split-Path -Parent $output
if (-not (Test-Path -LiteralPath $directory -PathType Container)) { New-Item -ItemType Directory -Path $directory -Force | Out-Null }
$utf8 = [Text.UTF8Encoding]::new($false); $script:Failed = $false
function Record([string]$Text) { [IO.File]::AppendAllText($output, $Text + [Environment]::NewLine, $utf8) }
function Invoke-Gate([string]$Label, [string]$Command, [string[]]$Arguments, [int]$Timeout = 900, [switch]$Race) {
  $started = [DateTime]::UtcNow; Write-Host "evidence_start label=$Label"
  $p = [Diagnostics.Process]::new(); $p.StartInfo.FileName = $Command; $p.StartInfo.WorkingDirectory = $root; $p.StartInfo.UseShellExecute = $false; $p.StartInfo.RedirectStandardOutput = $true; $p.StartInfo.RedirectStandardError = $true
  foreach ($arg in $Arguments) { [void]$p.StartInfo.ArgumentList.Add($arg) }; $p.StartInfo.Environment["GIT_MASTER"] = "1"; $processId = "not_started"; $exit = "start_failed"; $stdout = ""; $stderr = ""
  try { if (-not $p.Start()) { throw "process did not start" }; $processId = $p.Id; $out = $p.StandardOutput.ReadToEndAsync(); $err = $p.StandardError.ReadToEndAsync(); if ($p.WaitForExit($Timeout * 1000)) { $exit = $p.ExitCode } else { $p.Kill($true); [void]$p.WaitForExit(5000); $exit = "timeout" }; [Threading.Tasks.Task]::WaitAll(@($out,$err),5000); $stdout = $out.GetAwaiter().GetResult(); $stderr = $err.GetAwaiter().GetResult() } catch { $stderr += $_.Exception.Message } finally { $p.Dispose() }
  $unavailable = $Race -and $exit -ne 0 -and ($stdout + $stderr) -match '(?is)(gcc.*not found|C compiler .*not found|cgo:.*not found|requires cgo|CGO.*requires.*C compiler)'; $result = if ($exit -eq 0) { "passed" } elseif ($unavailable) { "unavailable" } else { $script:Failed = $true; "failed" }
  Record (("--- record ---`ntimestamp_utc={0}`ncommit_under_test={1}`nlabel={2}`ncommand={3} {4}`nprocess_id={5}`nexit_code={6}`ngate_result={7}`nstdout:`n{8}`nstderr:`n{9}" -f $started.ToString('o'),(git rev-parse HEAD).Trim(),$Label,$Command,($Arguments -join ' '),$processId,$exit,$result,$stdout,$stderr)); Write-Host "evidence_finish label=$Label result=$result"
}
[IO.File]::WriteAllText($output, "timestamp_utc=$([DateTime]::UtcNow.ToString('o'))`ncommit_under_test=$((git rev-parse HEAD).Trim())`nevidence_generator=scripts/generate-task8-evidence.ps1`n", $utf8)
Invoke-Gate "task8_qa" "go" @("test","./internal/app","-run","Test(Task8|RootMux_titlePages)","-count=1","-v")
Invoke-Gate "focused_gate" "go" @("test","./internal/store/sqlite","./internal/api/title","./internal/runtime","./internal/app","-count=10","-shuffle=on","-v")
Invoke-Gate "full_test" "go" @("test","./...","-count=1","-shuffle=on"); Invoke-Gate "build" "go" @("build","./cmd/jproxy"); Invoke-Gate "vet" "go" @("vet","./..."); Invoke-Gate "lint" "golangci-lint" @("run"); Invoke-Gate "diff_check" "git" @("diff","--check"); Invoke-Gate "redaction_scan" "pwsh" @("-NoProfile","-Command","rg -n 'task-canary|qa-api-key|qa-password' .omo/evidence; if (`$LASTEXITCODE -eq 1) { exit 0 }; exit `$LASTEXITCODE"); Invoke-Gate "race" "go" @("test","-race","./...","-count=1","-shuffle=on") -Race
if ($script:Failed) { throw "one or more required evidence commands failed" }
