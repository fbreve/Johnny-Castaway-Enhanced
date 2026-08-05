//go:build !windows

package main

// startHDRWatchdog is a no-op on non-Windows operating systems.
func startHDRWatchdog() {}
