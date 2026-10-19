# Run in a separate PowerShell window before the screensaver starts.
# Keep running until Ctrl+C; logs continue after Johnny exits.
$folder = Join-Path $env:TEMP 'JohnnyCastawayHDR'
New-Item -ItemType Directory -Force -Path $folder | Out-Null
$path = Join-Path $folder ("external-{0}.log" -f $PID)
$last = ''
while ($true) {
    $processes = Get-Process -Name JohnnyCastaway -ErrorAction SilentlyContinue |
        Sort-Object Id | ForEach-Object { $_.Id }
    $current = $processes -join ','
    if ($current -ne $last) {
        Add-Content -Path $path -Value ("{0} processes={1}" -f (Get-Date -Format o), $current)
        $last = $current
    }
    Start-Sleep -Seconds 1
}
