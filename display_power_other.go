//go:build !windows

package main

func watchDisplayPowerTransitions() {}
func shouldExitOnDisplayTransition() bool { return false }
