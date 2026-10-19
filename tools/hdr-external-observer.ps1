# Independent process observer; keep this PowerShell window open until Ctrl+C.
$ErrorActionPreference = 'Stop'
$folder = Join-Path $env:TEMP 'JohnnyCastawayHDR'
New-Item -ItemType Directory -Force -Path $folder | Out-Null
$path = Join-Path $folder ("external-{0}.log" -f $PID)
function Write-Trace([string]$message) {
    $timestamp = Get-Date -Format o
    Add-Content -LiteralPath $path -Encoding UTF8 -Value "$timestamp observer_pid=$PID $message"
}
Write-Trace "observer.start path=$path"
Write-Host "HDR observer logging to: $path"
$last = $null
$lastHeartbeat = [datetime]::MinValue
# Keep native process handles so an exit code can be read after a process disappears from CIM.
$tracked = @{}
try {
    while ($true) {
        try {
            $matches = @(Get-CimInstance Win32_Process -ErrorAction Stop |
                Where-Object {
                    ($_.Name -match '(?i)johnny.*\.(scr|exe)$') -or
                    ($_.ExecutablePath -match '(?i)johnny.*\.(scr|exe)$') -or
                    ($_.CommandLine -match '(?i)johnny.*\.scr')
                } |
                Sort-Object ProcessId |
                ForEach-Object { "pid=$($_.ProcessId) ppid=$($_.ParentProcessId) name=$($_.Name)" })
            $current = $matches -join '; '
            $seen = @{}
            foreach ($entry in $matches) {
                if ($entry -match '^pid=(\d+) ') {
                    $processId = [int]$Matches[1]
                    $seen[$processId] = $true
                    if (-not $tracked.ContainsKey($processId)) {
                        try {
                            $handle = [System.Diagnostics.Process]::GetProcessById($processId)
                            $tracked[$processId] = $handle
                            Write-Trace "process.open pid=$processId start=$($handle.StartTime.ToString('o'))"
                        } catch {
                            Write-Trace "process.open.error pid=$processId error=$($_.Exception.Message)"
                        }
                    }
                }
            }
            foreach ($processId in @($tracked.Keys)) {
                $handle = $tracked[$processId]
                try {
                    if ($handle.HasExited) {
                        Write-Trace "process.exit pid=$processId code=$($handle.ExitCode)"
                        $handle.Dispose()
                        $tracked.Remove($processId)
                    } elseif (-not $seen.ContainsKey($processId)) {
                        Write-Trace "process.cim.missing.but.alive pid=$processId"
                    }
                } catch {
                    Write-Trace "process.exit.error pid=$processId error=$($_.Exception.Message)"
                }
            }
            if ($current -ne $last) {
                Write-Trace "processes.changed $current"
                $last = $current
            }
        } catch {
            Write-Trace "processes.error $($_.Exception.Message)"
        }
        if (((Get-Date) - $lastHeartbeat).TotalSeconds -ge 30) {
            Write-Trace "observer.heartbeat"
            $lastHeartbeat = Get-Date
        }
        Start-Sleep -Seconds 1
    }
} finally {
    foreach ($handle in $tracked.Values) { $handle.Dispose() }
    Write-Trace "observer.stop"
}
