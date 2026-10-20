//go:build !windows

package main

func hdrWindowsEventsStart() {}

func hdrShouldPausePresentation() bool { return false }
