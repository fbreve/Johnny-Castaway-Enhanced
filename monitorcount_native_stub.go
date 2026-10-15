//go:build !linux || !cgo || sdl || sdl3 || rgfw || drm

package main

func getMonitorCountNative() int {
	return 0
}
