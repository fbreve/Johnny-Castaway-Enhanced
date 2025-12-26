package main

import "fmt"

var (
	debugEnabled = false
)

func debugPrintln(a ...any) {
	if debugEnabled {
		fmt.Println(a...)
	}
}

func debugPrintf(format string, a ...any) {
	if debugEnabled {
		fmt.Printf(format, a...)
	}
}
