//go:build windows

package main

import (
	"unsafe"

	rl "github.com/gen2brain/raylib-go/raylib"
	"golang.org/x/sys/windows"
)

// Why this file exists: this screensaver previously ran with
// FlagWindowTopmost and kept presenting an OpenGL frame continuously, even
// while Windows had powered the monitor off via the automatic idle
// timeout. On this user's NVIDIA setup, that combination reliably left HDR
// in a wrong state after later logging back in through the lock screen —
// but only when the *automatic* idle timeout triggered the monitor
// power-off, not a manual Win+L or monitor power button press. That
// distinction points at the OS-driven idle/display-power path specifically,
// not just "the monitor turned off" in general.
//
// GUID_CONSOLE_DISPLAY_STATE is the modern (Windows 8+) recommended
// power-setting notification for exactly this: components that render to
// the display are expected to register for it and stop rendering while the
// display is off. We go further than just pausing presentation (which did
// not fix the issue on its own) by minimizing the window, which releases
// its presentation surface back to the desktop the same way any minimized
// windowed app would, without unloading any GPU resources -- so nothing
// needs to be reloaded when we restore.

// GUID_CONSOLE_DISPLAY_STATE: 6FE69556-704A-47A0-8F24-C28D936FDA47
var guidConsoleDisplayState = windows.GUID{
	Data1: 0x6fe69556,
	Data2: 0x704a,
	Data3: 0x47a0,
	Data4: [8]byte{0x8f, 0x24, 0xc2, 0x8d, 0x93, 0x6f, 0xda, 0x47},
}

const (
	wmPowerBroadcast      = 0x0218
	pbtPowerSettingChange = 0x8013
	deviceNotifyWindowH   = 0x00000000
	gwlpWndProc           = -4
)

// POWERBROADCAST_SETTING layout we care about:
//   GUID PowerSetting (16 bytes)
//   DWORD DataLength   (4 bytes)
//   BYTE  Data[1]      (variable, a DWORD in this case: 0=off,1=on,2=dimmed)
type powerBroadcastSettingHeader struct {
	PowerSetting windows.GUID
	DataLength   uint32
	// Data follows immediately after this struct in memory.
}

var (
	origWndProc uintptr
	subclassed  bool
)

func wndProc(hwnd windows.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	if msg == wmPowerBroadcast && wParam == pbtPowerSettingChange && lParam != 0 {
		hdr := (*powerBroadcastSettingHeader)(unsafe.Pointer(lParam))
		if hdr.PowerSetting == guidConsoleDisplayState && hdr.DataLength >= 4 {
			dataPtr := lParam + unsafe.Sizeof(powerBroadcastSettingHeader{})
			state := *(*uint32)(unsafe.Pointer(dataPtr))
			if state == 0 {
				// Display is off: release the presentation surface.
				if !rl.IsWindowMinimized() {
					rl.MinimizeWindow()
				}
			} else {
				// Display is on (1) or dimmed (2): bring the surface back.
				if rl.IsWindowMinimized() {
					rl.RestoreWindow()
				}
			}
		}
	}
	return callWindowProc(origWndProc, hwnd, msg, wParam, lParam)
}

func installMonitorPowerWatch() {
	if subclassed {
		return
	}
	hwndPtr := rl.GetWindowHandle()
	if hwndPtr == nil {
		return
	}
	hwnd := windows.HWND(uintptr(hwndPtr))

	if registerPowerSettingNotification(hwnd, &guidConsoleDisplayState, deviceNotifyWindowH) == 0 {
		return
	}

	prev := setWindowLongPtr(hwnd, gwlpWndProc, windowsCallback())
	if prev == 0 {
		return
	}
	origWndProc = prev
	subclassed = true
}

// isMonitorOff is kept for callers that want to skip extra per-frame work
// (e.g. animation bookkeeping) while minimized; the actual presentation
// pause now happens via MinimizeWindow/RestoreWindow above, not by reading
// this flag, since polling once a message has already been handled is too
// late to matter for that part.
func isMonitorOff() bool {
	return rl.IsWindowMinimized()
}

// --- thin syscall wrappers (user32.dll) ---

var (
	user32                        = windows.NewLazySystemDLL("user32.dll")
	procSetWindowLongPtrW         = user32.NewProc("SetWindowLongPtrW")
	procCallWindowProcW           = user32.NewProc("CallWindowProcW")
	procRegisterPowerSettingNotif = user32.NewProc("RegisterPowerSettingNotification")
)

func setWindowLongPtr(hwnd windows.HWND, index int32, newProc uintptr) uintptr {
	ret, _, _ := procSetWindowLongPtrW.Call(uintptr(hwnd), uintptr(index), newProc)
	return ret
}

func callWindowProc(prevProc uintptr, hwnd windows.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	ret, _, _ := procCallWindowProcW.Call(prevProc, uintptr(hwnd), uintptr(msg), wParam, lParam)
	return ret
}

func registerPowerSettingNotification(hwnd windows.HWND, guid *windows.GUID, flags uint32) uintptr {
	ret, _, _ := procRegisterPowerSettingNotif.Call(
		uintptr(hwnd),
		uintptr(unsafe.Pointer(guid)),
		uintptr(flags),
	)
	return ret
}

func windowsCallback() uintptr {
	return windows.NewCallback(func(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
		return wndProc(windows.HWND(hwnd), msg, wParam, lParam)
	})
}
