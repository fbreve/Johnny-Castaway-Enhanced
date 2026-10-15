//go:build linux && cgo && !sdl && !sdl3 && !rgfw && !drm

package main

/*
extern int glfwInit(void);
extern void glfwTerminate(void);
extern void **glfwGetMonitors(int *count);
*/
import "C"

// getMonitorCountNative initializes GLFW without creating a window. This is
// enough for monitor enumeration and avoids flashing a throwaway Raylib window
// before the real screensaver window is created.
func getMonitorCountNative() int {
	if C.glfwInit() == 0 {
		return 0
	}
	defer C.glfwTerminate()

	var count C.int
	C.glfwGetMonitors(&count)
	return int(count)
}
