# Independent process observer; keep this PowerShell window open until Ctrl+C.
$ErrorActionPreference = 'Stop'
$folder = Join-Path $env:TEMP 'JohnnyCastawayHDR'
New-Item -ItemType Directory -Force -Path $folder | Out-Null
$path = Join-Path $folder ("external-{0}.log" -f $PID)
function Write-Trace([string]$message) {
    $timestamp = Get-Date -Format o
    Add-Content -LiteralPath $path -Encoding UTF8 -Value "$timestamp observer_pid=$PID $message"
}
# Native process exit status; the managed ExitCode property can be unavailable.
Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;
public static class JohnnyNativeDiagnostics {
    [DllImport("user32.dll")]
    private static extern int GetDisplayConfigBufferSizes(uint flags, out uint paths, out uint modes);
    [DllImport("user32.dll")]
    private static extern int QueryDisplayConfig(uint flags, ref uint paths, IntPtr pathBuffer, ref uint modes, IntPtr modeBuffer, IntPtr topology);
    [DllImport("user32.dll")]
    private static extern int DisplayConfigGetDeviceInfo(IntPtr request);
    public static string GetAdvancedColorStatus() {
        const uint ACTIVE = 2;
        uint paths, modes;
        int result = GetDisplayConfigBufferSizes(ACTIVE, out paths, out modes);
        if (result != 0) return "buffer_error=" + result;
        // DISPLAYCONFIG_PATH_INFO is 72 bytes; DISPLAYCONFIG_MODE_INFO is 64 bytes.
        IntPtr pb = Marshal.AllocHGlobal(checked((int)(paths * 72)));
        IntPtr mb = Marshal.AllocHGlobal(checked((int)(modes * 64)));
        try {
            result = QueryDisplayConfig(ACTIVE, ref paths, pb, ref modes, mb, IntPtr.Zero);
            if (result != 0) return "query_error=" + result;
            var statuses = new System.Collections.Generic.List<string>();
            for (int i = 0; i < paths; i++) {
                IntPtr target = IntPtr.Add(pb, i * 72 + 20);
                IntPtr info = Marshal.AllocHGlobal(32);
                try {
                    // DISPLAYCONFIG_DEVICE_INFO_HEADER: type, size, adapter LUID, target ID.
                    Marshal.WriteInt32(info, 0, 9); // GET_ADVANCED_COLOR_INFO
                    Marshal.WriteInt32(info, 4, 32);
                    Marshal.WriteInt64(info, 8, Marshal.ReadInt64(target, 0));
                    uint targetId = unchecked((uint)Marshal.ReadInt32(target, 8));
                    Marshal.WriteInt32(info, 16, unchecked((int)targetId));
                    int rc = DisplayConfigGetDeviceInfo(info);
                    if (rc == 0) {
                        uint flags = unchecked((uint)Marshal.ReadInt32(info, 20));
                        // DISPLAYCONFIG_GET_ADVANCED_COLOR_INFO bitfields (LSB first):
                        // advancedColorSupported=bit 0, advancedColorEnabled=bit 1,
                        // wideColorEnforced=bit 2, advancedColorForceDisabled=bit 3.
                        // Higher bits are reserved: retain them for diagnostics, but never
                        // interpret them as feature-state changes.
                        uint known = flags & 0xFu;
                        statuses.Add("target=" + targetId + ",supported=" + ((known & 1)!=0) +
                            ",enabled=" + ((known & 2)!=0) + ",wide_color_enforced=" + ((known & 4)!=0) +
                            ",force_disabled=" + ((known & 8)!=0) +
                            ",known_flags=0x" + known.ToString("X") +
                            ",reserved_flags=0x" + (flags & ~0xFu).ToString("X") +
                            ",raw_flags=0x" + flags.ToString("X"));
                    } else statuses.Add("target=" + targetId + ",error=" + rc);
                } finally { Marshal.FreeHGlobal(info); }
            }
            return string.Join(";", statuses);
        } finally { Marshal.FreeHGlobal(pb); Marshal.FreeHGlobal(mb); }
    }
    [DllImport("kernel32.dll", SetLastError=true)]
    public static extern bool GetExitCodeProcess(IntPtr process, out uint exitCode);
}
'@
Write-Trace "observer.start path=$path"
Write-Host "HDR observer logging to: $path"
$last = $null
$lastHeartbeat = [datetime]::MinValue
$lastColorSnapshot = [datetime]::MinValue
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
                            Write-Trace "process.open pid=$processId start=$($handle.StartTime.ToString('o')) handle=$($handle.Handle)"
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
                        $nativeCode = [uint32]0
                        $nativeOk = [JohnnyNativeDiagnostics]::GetExitCodeProcess($handle.Handle, [ref]$nativeCode)
                        $nativeError = [Runtime.InteropServices.Marshal]::GetLastWin32Error()
                        $managedCode = try { [string]$handle.ExitCode } catch { "error:$($_.Exception.Message)" }
                        Write-Trace "process.exit pid=$processId native_ok=$nativeOk native_code=$nativeCode native_error=$nativeError managed_code=$managedCode"
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
                Write-Trace "display.advanced_color $([JohnnyNativeDiagnostics]::GetAdvancedColorStatus())"
                Write-Trace "processes.changed $current"
                $last = $current
            }
        } catch {
            Write-Trace "processes.error $($_.Exception.Message)"
        }
        if (((Get-Date) - $lastColorSnapshot).TotalSeconds -ge 30) {
            try { Write-Trace "display.advanced_color $([JohnnyNativeDiagnostics]::GetAdvancedColorStatus())" } catch { Write-Trace "display.advanced_color.error $($_.Exception.Message)" }
            $lastColorSnapshot = Get-Date
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
