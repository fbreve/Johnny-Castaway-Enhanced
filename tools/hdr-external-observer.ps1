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
    Write-Trace "observer.stop"
}
