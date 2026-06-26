//go:build windows

package main

/*
#include <windows.h>

// Force high-performance GPU on hybrid graphics laptops
__declspec(dllexport) DWORD NvOptimusEnablement = 0x00000001;
__declspec(dllexport) int AmdPowerXpressRequestHighPerformance = 1;
*/
import "C"

import "syscall"

func preloadNativeOpenGL() {
	_, _ = syscall.LoadLibrary("C:\\Windows\\System32\\opengl32.dll")
}
