package main

import (
	"fmt"
	"log"
	"os"
)

var (
	debugEnabled = false
	debugLogFile *os.File
)

// debugLog writes to debug.log next to the executable. Needed because
// build.bat links with "-H windowsgui" (see commit 4072475, "Hide console
// window when running as screensaver"), which produces a GUI-subsystem
// binary with no console attached — fmt.Print*/os.Stdout writes go nowhere,
// even when launched from an existing terminal, so plain stdout debug prints
// are invisible. A file works regardless of subsystem.
func debugLog() *os.File {
	if debugLogFile == nil {
		f, err := os.OpenFile("debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil
		}
		debugLogFile = f
		log.SetOutput(f)
		log.SetFlags(0)
	}
	return debugLogFile
}

func debugPrintln(a ...any) {
	if debugEnabled {
		fmt.Println(a...)
		if debugLog() != nil {
			log.Println(a...)
		}
	}
}

func debugPrintf(format string, a ...any) {
	if debugEnabled {
		fmt.Printf(format, a...)
		if debugLog() != nil {
			log.Printf(format, a...)
		}
	}
}

// debugMarker always writes, regardless of debugEnabled, so we can confirm
// from debug.log which build actually produced a given run without relying
// on the Shift-key toggle timing.
func debugMarker(a ...any) {
	if debugLog() != nil {
		log.Println(a...)
	}
}
