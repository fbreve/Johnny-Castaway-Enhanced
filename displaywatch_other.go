//go:build !windows

package main

// On non-Windows platforms there's no Windows-specific monitor power
// notification to watch, so these are no-ops.

func installMonitorPowerWatch() {}

func isMonitorOff() bool { return false }
