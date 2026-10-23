//go:build windows

package main

import (
 "runtime"
 "sync/atomic"
 "syscall"
 "unsafe"
)

var displayExitRequested atomic.Bool

func shouldExitOnDisplayTransition() bool { return displayExitRequested.Load() }

type displayGUID struct { A uint32; B, C uint16; D [8]byte }
type displayPowerSetting struct { GUID displayGUID; Length uint32; Data [1]byte }
type displayWindowClass struct {
 Size, Style uint32
 Proc, ClassExtra, WindowExtra, Instance, Icon, Cursor, Background, MenuName, ClassName, IconSmall uintptr
}
type displayMessage struct {
 Hwnd uintptr
 Message uint32
 WParam, LParam uintptr
 Time uint32
 X, Y int32
 Private uint32
}

func watchDisplayPowerTransitions() {
 go func() {
  runtime.LockOSThread()
  defer runtime.UnlockOSThread()
  user32 := syscall.NewLazyDLL("user32.dll")
  kernel32 := syscall.NewLazyDLL("kernel32.dll")
  registerClass := user32.NewProc("RegisterClassExW")
  createWindow := user32.NewProc("CreateWindowExW")
  defWindowProc := user32.NewProc("DefWindowProcW")
  registerPower := user32.NewProc("RegisterPowerSettingNotification")
  unregisterPower := user32.NewProc("UnregisterPowerSettingNotification")
  getMessage := user32.NewProc("GetMessageW")
  translateMessage := user32.NewProc("TranslateMessage")
  dispatchMessage := user32.NewProc("DispatchMessageW")
  destroyWindow := user32.NewProc("DestroyWindow")
  unregisterClass := user32.NewProc("UnregisterClassW")
  getModuleHandle := kernel32.NewProc("GetModuleHandleW")
  guid := displayGUID{0x6fe69556,0x704a,0x47a0,[8]byte{0x8f,0x24,0xc2,0x8d,0x93,0x6f,0xda,0x47}}
  callback := syscall.NewCallback(func(hwnd uintptr, msg uint32, wp, lp uintptr) uintptr {
   if msg == 0x0218 && wp == 0x8013 && lp != 0 {
    setting := (*displayPowerSetting)(unsafe.Pointer(lp))
    if setting.GUID == guid && setting.Length >= 4 {
     value := *(*uint32)(unsafe.Pointer(&setting.Data[0]))
     if value == 0 || value == 2 {
      displayExitRequested.Store(true)
     }
    }
   }
   result, _, _ := defWindowProc.Call(hwnd, uintptr(msg), wp, lp)
   return result
  })
  instance, _, _ := getModuleHandle.Call(0)
  name, _ := syscall.UTF16PtrFromString("JohnnyDisplayPowerExit")
  wc := displayWindowClass{Size:uint32(unsafe.Sizeof(displayWindowClass{})), Proc:callback, Instance:instance, ClassName:uintptr(unsafe.Pointer(name))}
  atom, _, _ := registerClass.Call(uintptr(unsafe.Pointer(&wc)))
  if atom == 0 { return }
  defer unregisterClass.Call(uintptr(unsafe.Pointer(name)),instance)
  hwnd, _, _ := createWindow.Call(0,uintptr(unsafe.Pointer(name)),uintptr(unsafe.Pointer(name)),0,0,0,0,0,0,0,instance,0)
  if hwnd == 0 { return }
  defer destroyWindow.Call(hwnd)
  handle, _, _ := registerPower.Call(hwnd,uintptr(unsafe.Pointer(&guid)),0)
  if handle == 0 { return }
  defer unregisterPower.Call(handle)
  var message displayMessage
  for {
   n, _, _ := getMessage.Call(uintptr(unsafe.Pointer(&message)),hwnd,0,0)
   if int32(n) <= 0 { return }
   translateMessage.Call(uintptr(unsafe.Pointer(&message)))
   dispatchMessage.Call(uintptr(unsafe.Pointer(&message)))
  }
 }()
}
