//go:build !windows

package main

func hdrWindowsEventsStart() {}

func hdrShouldExitOnDisplayOff() bool { return false }
