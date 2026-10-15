//go:build windows

package main

import "syscall"

// getMonitorCountWin32 queries GetSystemMetrics(SM_CMONITORS) directly,
// avoiding the need to initialize and destroy a dummy Raylib window first.
func getMonitorCountWin32() int {
	user32 := syscall.NewLazyDLL("user32.dll")
	getSystemMetrics := user32.NewProc("GetSystemMetrics")
	ret, _, _ := getSystemMetrics.Call(80) // SM_CMONITORS = 80
	return int(ret)
}
