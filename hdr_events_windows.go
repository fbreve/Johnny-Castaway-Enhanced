//go:build windows

package main

import (
 "runtime"
 "syscall"
 "unsafe"
)

const (
 hdrWMClose = 0x0010
 hdrWMDestroy = 0x0002
 hdrWMQueryEndSession = 0x0011
 hdrWMEndSession = 0x0016
 hdrWMDisplayChange = 0x007E
 hdrWMPowerBroadcast = 0x0218
 hdrWMWTSSessionChange = 0x02B1
 hdrWTSLock = 0x7
 hdrWTSUnlock = 0x8
 hdrWTSNotifyThisSession = 0
)

type hdrWndClass struct {
 size uint32
 style uint32
 proc uintptr
 clsExtra int32
 wndExtra int32
 instance uintptr
 icon uintptr
 cursor uintptr
 background uintptr
 menuName *uint16
 className *uint16
 iconSmall uintptr
}
type hdrMsg struct {
 hwnd uintptr
 message uint32
 wparam uintptr
 lparam uintptr
 time uint32
 ptX int32
 ptY int32
 private uint32
}
var (
 hdrUser32 = syscall.NewLazyDLL("user32.dll")
 hdrKernel32 = syscall.NewLazyDLL("kernel32.dll")
 hdrWTS = syscall.NewLazyDLL("wtsapi32.dll")
 hdrRegisterClass = hdrUser32.NewProc("RegisterClassExW")
 hdrCreateWindow = hdrUser32.NewProc("CreateWindowExW")
 hdrGetMessage = hdrUser32.NewProc("GetMessageW")
 hdrTranslateMessage = hdrUser32.NewProc("TranslateMessage")
 hdrDispatchMessage = hdrUser32.NewProc("DispatchMessageW")
 hdrDefWindowProc = hdrUser32.NewProc("DefWindowProcW")
 hdrDestroyWindow = hdrUser32.NewProc("DestroyWindow")
 hdrGetModuleHandle = hdrKernel32.NewProc("GetModuleHandleW")
 hdrWTSRegister = hdrWTS.NewProc("WTSRegisterSessionNotification")
 hdrWTSUnregister = hdrWTS.NewProc("WTSUnRegisterSessionNotification")
)

func hdrWindowsEventsStart() {
 if hdrTraceEnabled != "1" { return }
 go hdrWindowsEventsLoop()
}
func hdrWindowsEventsLoop() {
 runtime.LockOSThread()
 defer runtime.UnlockOSThread()
 className := syscall.StringToUTF16Ptr("JohnnyHDRTraceObserver")
 instance, _, _ := hdrGetModuleHandle.Call(0)
 callback := syscall.NewCallback(func(hwnd uintptr, msg uint32, wp, lp uintptr) uintptr {
  switch msg {
  case hdrWMDisplayChange:
   hdrTrace("windows.display.change bits=%d size=%dx%d", wp, uint16(lp), uint16(lp>>16))
  case hdrWMPowerBroadcast:
   hdrTrace("windows.power.broadcast event=0x%x", wp)
  case hdrWMWTSSessionChange:
   hdrTrace("windows.session.change event=0x%x session=%d", wp, lp)
  case hdrWMQueryEndSession:
   hdrTrace("windows.query.end.session flags=0x%x", lp)
  case hdrWMEndSession:
   hdrTrace("windows.end.session ending=%d flags=0x%x", wp, lp)
  case hdrWMClose:
   hdrTrace("windows.observer.close")
  case hdrWMDestroy:
   hdrTrace("windows.observer.destroy")
  }
  result, _, _ := hdrDefWindowProc.Call(hwnd, uintptr(msg), wp, lp)
  return result
 })
 wc := hdrWndClass{size:uint32(unsafe.Sizeof(hdrWndClass{})),proc:callback,instance:instance,className:className}
 atom, _, err := hdrRegisterClass.Call(uintptr(unsafe.Pointer(&wc)))
 if atom == 0 { hdrTrace("windows.observer.register.failed err=%v",err); return }
 // A hidden top-level window receives broadcast messages; message-only windows do not.
 hwnd, _, err := hdrCreateWindow.Call(0,uintptr(unsafe.Pointer(className)),uintptr(unsafe.Pointer(className)),0,0,0,0,0,0,0,instance,0)
 if hwnd == 0 { hdrTrace("windows.observer.create.failed err=%v",err); return }
 registered, _, wtsErr := hdrWTSRegister.Call(hwnd,hdrWTSNotifyThisSession)
 hdrTrace("windows.observer.ready hwnd=0x%x session_notifications=%v err=%v",hwnd,registered!=0,wtsErr)
 defer func() {
  if registered != 0 { hdrWTSUnregister.Call(hwnd) }
  hdrDestroyWindow.Call(hwnd)
  hdrTrace("windows.observer.stopped")
 }()
 var msg hdrMsg
 for {
  n, _, getErr := hdrGetMessage.Call(uintptr(unsafe.Pointer(&msg)),0,0,0)
  if int32(n) <= 0 { hdrTrace("windows.observer.message.loop.end status=%d err=%v",int32(n),getErr); return }
  hdrTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
  hdrDispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
 }
}
