//go:build windows

package main

import (
 "runtime"
 "sync/atomic"
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
 hdrPBTSettingChange = 0x8013
)

type hdrGUID struct { a uint32; b,c uint16; d [8]byte }
type hdrPowerSetting struct { setting hdrGUID; length uint32; data [1]byte }
var hdrExitOnDisplayOff atomic.Bool

func hdrShouldExitOnDisplayOff() bool {
 return hdrExitOnDisplayOff.Load()
}

var hdrConsoleDisplay = hdrGUID{0x6fe69556,0x704a,0x47a0,[8]byte{0x8f,0x24,0xc2,0x8d,0x93,0x6f,0xda,0x47}}
var hdrSessionDisplay = hdrGUID{0x2b84c20e,0xad23,0x4ddf,[8]byte{0x93,0xdb,0x05,0xff,0xbd,0x7e,0xfc,0xa5}}

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
 hdrRegisterPower = hdrUser32.NewProc("RegisterPowerSettingNotification")
 hdrUnregisterPower = hdrUser32.NewProc("UnregisterPowerSettingNotification")
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
   if wp == hdrPBTSettingChange && lp != 0 {
    setting := (*hdrPowerSetting)(unsafe.Pointer(lp))
    if setting.length >= 4 && setting.length <= 256 {
     value := *(*uint32)(unsafe.Pointer(&setting.data[0]))
     hdrTrace("windows.power.setting guid=%08x value=%d",setting.setting.a,value)
     if setting.setting.a == hdrConsoleDisplay.a {
      if value == 2 && hdrExitOnDisplayOff.CompareAndSwap(false, true) {
       hdrTrace("windows.display.off.exit.requested power_value=%d", value)
      }
     }
    } else {
     hdrTrace("windows.power.setting.invalid length=%d",setting.length)
    }
   } else { hdrTrace("windows.power.broadcast event=0x%x", wp) }
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
 var powerHandles []uintptr
 for _, g := range []*hdrGUID{&hdrConsoleDisplay,&hdrSessionDisplay} {
  h, _, powerErr := hdrRegisterPower.Call(hwnd,uintptr(unsafe.Pointer(g)),0)
  hdrTrace("windows.power.register guid=%08x success=%v err=%v",g.a,h!=0,powerErr)
  if h != 0 {powerHandles=append(powerHandles,h)}
 }
 registered, _, wtsErr := hdrWTSRegister.Call(hwnd,hdrWTSNotifyThisSession)
 hdrTrace("windows.observer.ready hwnd=0x%x session_notifications=%v err=%v",hwnd,registered!=0,wtsErr)
 defer func() {
  for _, h := range powerHandles { hdrUnregisterPower.Call(h) }
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
