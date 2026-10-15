//go:build windows

package main

import "syscall"

var (
	modUser32            = syscall.NewLazyDLL("user32.dll")
	procGetAsyncKeyState = modUser32.NewProc("GetAsyncKeyState")
)

// isKeyDownGlobally polls the physical key state via GetAsyncKeyState, so the
// debug hotkeys keep working even when the screensaver window has no focus.
func isKeyDownGlobally(vk int) bool {
	r, _, _ := procGetAsyncKeyState.Call(uintptr(vk))
	return (r & 0x8000) != 0
}

func isAnyKeyPressedGlobally() bool {
	for vk := 8; vk <= 255; vk++ {
		if isKeyDownGlobally(vk) {
			return true
		}
	}
	return false
}
