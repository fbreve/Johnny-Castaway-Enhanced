package main

import (
	"os"
	"strconv"
)

// Wayland layer-shell mode, implemented by the patched GLFW backend in
// patches/glfw-wayland-layer-shell.
//
// A normal Wayland toplevel window can never render above layer-shell
// surfaces, whatever its z-order: the status bar, launcher menus and
// notifications live on the "top" layer, which always stacks above regular
// windows. Mapping the screensaver itself as an "overlay" layer surface
// (the layer lock screens use) puts it above everything: floating and
// pinned windows, the bar, open menus and popups.
//
// The mode is enabled via environment variables read by GLFW at window
// creation, set here right before InitWindow:
//   - GLFW_WLR_LAYER_SHELL=1        map the window as an overlay surface
//   - GLFW_WLR_LAYER_OUTPUT=<idx>    anchor it to monitor <idx>
//
// If the compositor has no wlr-layer-shell support, GLFW falls back to the
// regular xdg toplevel path, i.e. the previous fullscreen behavior.

// waylandSessionEnv reports a Wayland session from the environment alone.
// Unlike isWaylandSession() - which queries GLFW's platform and therefore
// requires an initialized GLFW - it is safe to call before rl.InitWindow,
// where the layer-shell mode has to be armed.
func waylandSessionEnv() bool {
	return os.Getenv("WAYLAND_DISPLAY") != "" || os.Getenv("XDG_SESSION_TYPE") == "wayland"
}

// enableLayerShellMode arms the layer-shell window mode for the screensaver
// window. Must run before rl.InitWindow; only the screensaver mode uses it,
// and the throwaway monitor-count window opened by getMonitorCount() in the
// multi-instance parent never sees it.
func enableLayerShellMode() {
	if !isScreensaverMode || !waylandSessionEnv() {
		return
	}
	_ = os.Setenv("GLFW_WLR_LAYER_SHELL", "1")
	if hasMonitorIndex {
		_ = os.Setenv("GLFW_WLR_LAYER_OUTPUT", strconv.Itoa(runOnMonitorIndex))
	}
}

// layerShellMode reports whether this process runs with the layer-shell
// window mode armed.
func layerShellMode() bool {
	return os.Getenv("GLFW_WLR_LAYER_SHELL") == "1"
}
