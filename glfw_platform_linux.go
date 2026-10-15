//go:build linux && cgo && !sdl && !sdl3 && !rgfw && !drm

package main

/*
 // glfwGetPlatform is provided by the GLFW 3.4 bundled into raylib-go.
 // Query it only after raylib has initialized the window/GLFW.
 extern int glfwGetPlatform(void);
*/
import "C"

const glfwPlatformWayland = 0x00060003

func glfwPlatform() int {
	return int(C.glfwGetPlatform())
}
