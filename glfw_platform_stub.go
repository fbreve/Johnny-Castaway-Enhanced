//go:build !linux || !cgo || sdl || sdl3 || rgfw || drm

package main

const glfwPlatformWayland = 0x00060003

// Non-GLFW builds must never enter the GLFW-specific Wayland window path.
func glfwPlatform() int {
	return 0
}
