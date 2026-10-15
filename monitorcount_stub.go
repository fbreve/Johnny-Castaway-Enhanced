//go:build !windows

package main

// getMonitorCountWin32 has no Win32 shortcut on other platforms; the caller
// falls back to Raylib's own monitor enumeration instead.
func getMonitorCountWin32() int {
	return 0
}
