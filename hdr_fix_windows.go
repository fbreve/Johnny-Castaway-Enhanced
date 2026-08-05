//go:build windows

package main

import (
	"syscall"
	"time"
	"unsafe"
)

var (
	user32dll          = syscall.NewLazyDLL("user32.dll")
	procCreateWindowEx = user32dll.NewProc("CreateWindowExW")
	procGetMessage     = user32dll.NewProc("GetMessageW")
	procDispatchMsg    = user32dll.NewProc("DispatchMessageW")
	procChangeDisplay  = user32dll.NewProc("ChangeDisplaySettingsW")
)

const (
	WM_POWERBROADCAST      = 0x0218
	PBT_APMRESUMEAUTOMATIC = 0x0012
	PBT_APMRESUMESUSPEND   = 0x0007
)

// startHDRWatchdog spawns a hidden window that listens for monitor
// wake events and calls ChangeDisplaySettings to re-initialize HDR.
func startHDRWatchdog() {
	go func() {
		// Use HWND_MESSAGE (-3) for a message-only window
		hwnd, _, _ := procCreateWindowEx.Call(
			0, uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("STATIC"))),
			uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("JCHDRWatch"))),
			0, 0, 0, 0, 0,
			^uintptr(2), // HWND_MESSAGE (-3)
			0, 0, 0,
		)
		if hwnd == 0 {
			return
		}

		type MSG struct {
			Hwnd    uintptr
			Message uint32
			WParam  uintptr
			LParam  uintptr
			Time    uint32
			Pt      [2]int32
		}

		var msg MSG
		for {
			ret, _, _ := procGetMessage.Call(
				uintptr(unsafe.Pointer(&msg)), hwnd, 0, 0,
			)
			if ret == 0 || ret == ^uintptr(0) {
				break
			}
			if msg.Message == WM_POWERBROADCAST {
				if msg.WParam == PBT_APMRESUMEAUTOMATIC || msg.WParam == PBT_APMRESUMESUSPEND {
					// Small delay to let Windows finish restoring the session
					time.Sleep(2 * time.Second)
					// Reset display mode — forces GPU driver to re-apply HDR
					procChangeDisplay.Call(uintptr(unsafe.Pointer(nil)), 0)
				}
			}
			procDispatchMsg.Call(uintptr(unsafe.Pointer(&msg)))
		}
	}()
}
