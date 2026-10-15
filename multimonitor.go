package main

import (

	rl "github.com/gen2brain/raylib-go/raylib"
)

// getMonitorCount returns the number of connected monitors.
// On Windows it queries GetSystemMetrics(SM_CMONITORS) directly, avoiding
// needing to initialize and destroy a dummy Raylib window beforehand.
func getMonitorCount() int {
	if ret := getMonitorCountWin32(); ret > 0 {
		return ret
	}
	// On every other platform GLFW can only enumerate monitors while it
	// is initialized, i.e. while a window exists. runStory() calls this
	// BEFORE opening the real window (it must know the monitor count to
	// decide between spawning per-monitor children and opening a single
	// spanning window), so open a tiny throwaway window to initialize
	// GLFW, count the monitors, and tear it down again. Without this the
	// count silently comes back as <= 1 on Linux/BSD and multi-instance
	// mode never spawns its children.
	if !rl.IsWindowReady() {
		rl.InitWindow(64, 64, "")
		defer rl.CloseWindow()
	}
	return rl.GetMonitorCount()
}

// TMonitorRect describes one connected monitor's region within the single
// application window, in window-local coordinates (i.e. already offset so
// that (0,0) is the window's own top-left corner, not the OS desktop's).
type TMonitorRect struct {
	X, Y, W, H float32
}

// monitorRects holds one entry per connected monitor, computed once by
// setupMonitors(). The renderer draws a separate, correctly letterboxed
// copy of the scene into each entry's rectangle.
var monitorRects []TMonitorRect

// isWaylandSession checks the window system actually selected by GLFW after
// rl.InitWindow. Session environment variables are not reliable here: XWayland
// clients can inherit WAYLAND_DISPLAY and XDG_SESSION_TYPE=wayland.
func isWaylandSession() bool {
	return isWaylandPlatform(glfwPlatform())
}

func isWaylandPlatform(platform int) bool {
	return platform == glfwPlatformWayland
}

func validMonitorIndex(index, count int) bool {
	return index >= 0 && index < count
}

func selectedMonitorIndex(requested int, hasRequested bool, current int, count int) int {
	if count < 1 {
		return 0
	}
	if hasRequested && validMonitorIndex(requested, count) {
		return requested
	}
	if validMonitorIndex(current, count) {
		return current
	}
	return 0
}

// setupMonitors sizes and positions the application window to span every
// connected monitor (the combined virtual desktop bounding box, which
// correctly handles monitors of different sizes and irregular/offset
// arrangements, not just a simple side-by-side layout), and records each
// monitor's own rectangle in window-local coordinates.
//
// On a single-monitor system this reduces to exactly the previous
// behavior (one window sized to that one monitor, with a single
// full-window rectangle), so this is always safe to call instead of the
// old single-monitor sizing code, not just an opt-in extra mode.
func setupMonitors() {
	// Wayland compositors ignore programmatic window positioning entirely
	// and usually refuse the monitor-sized manual resize done below, which
	// leaves the screensaver stuck in its initial 640x480 window at a
	// compositor-chosen spot while the renderer letterboxes for the full
	// monitor size (showing up as a tiny zoomed scene in a screen corner).
	// A Wayland client can only reliably request fullscreen, so switch to a
	// fullscreen window on the target monitor and leave monitorRects empty:
	// the renderer then letterboxes against the actual window size on every
	// frame, which also absorbs the asynchronous compositor resize.
	if isWaylandSession() {
		monitorCount := rl.GetMonitorCount()
		monitor := selectedMonitorIndex(runOnMonitorIndex, hasMonitorIndex, rl.GetCurrentMonitor(), monitorCount)

		pumpFrame := func() {
			rl.BeginDrawing()
			rl.ClearBackground(rl.Black)
			rl.EndDrawing()
		}

		// Under Wayland the xdg_toplevel fullscreen request only works
		// once the surface has been mapped: sending it before the first
		// buffer swap results in a configure event the compositor never
		// delivers, so the window (and its framebuffer) stay at the
		// initial 640x480 while the renderer letterboxes for the full
		// monitor (a tiny zoomed scene in a corner). Pump a few frames
		// first to map the window, then request fullscreen - the only
		// window placement a Wayland client can rely on.
		for i := 0; i < 10; i++ {
			pumpFrame()
		}
		// Select the requested output while the window is still windowed. On
		// Wayland the subsequent fullscreen request is then associated with
		// that output instead of the compositor-chosen initial one.
		if hasMonitorIndex && validMonitorIndex(runOnMonitorIndex, monitorCount) && rl.GetCurrentMonitor() != monitor {
			rl.SetWindowMonitor(monitor)
			for i := 0; i < 3; i++ {
				pumpFrame()
			}
		}
		rl.ToggleFullscreen()

		// Wait until the compositor-applied fullscreen resize reaches the
		// framebuffer, so size-dependent setup done later (e.g. the
		// widescreen virtual canvas width) sees the final size and not the
		// initial 640x480 one.
		targetW := rl.GetMonitorWidth(monitor)
		targetH := rl.GetMonitorHeight(monitor)
		for i := 0; i < 150; i++ {
			pumpFrame()
			if rl.GetRenderWidth() >= targetW-8 && rl.GetRenderHeight() >= targetH-8 {
				break
			}
		}

		// raylib's logical screen size is not updated by the fullscreen
		// configure, so resync it with the actual framebuffer size, then
		// leave monitorRects empty: the renderer letterboxes against the
		// actual window size on every frame.
		if rw, rh := rl.GetRenderWidth(), rl.GetRenderHeight(); rw > 0 && rh > 0 {
			rl.SetWindowSize(rw, rh)
		}
		monitorRects = nil
		return
	}
	if hasMonitorIndex {
		count := rl.GetMonitorCount()
		if !validMonitorIndex(runOnMonitorIndex, count) {
			runOnMonitorIndex = selectedMonitorIndex(runOnMonitorIndex, true, rl.GetCurrentMonitor(), count)
		}
		pos := rl.GetMonitorPosition(runOnMonitorIndex)
		w := float32(rl.GetMonitorWidth(runOnMonitorIndex))
		h := float32(rl.GetMonitorHeight(runOnMonitorIndex))
		if w <= 0 || h <= 0 {
			w, h = 1920, 1080
		}
		rl.SetWindowSize(int(w), int(h))
		rl.SetWindowPosition(int(pos.X), int(pos.Y))
		monitorRects = []TMonitorRect{{X: 0, Y: 0, W: w, H: h}}
		return
	}

	count := rl.GetMonitorCount()
	if count < 1 {
		count = 1
	}

	type rawMonitor struct {
		x, y, w, h float32
	}
	raw := make([]rawMonitor, 0, count)

	haveBounds := false
	var minX, minY, maxX, maxY float32

	for i := 0; i < count; i++ {
		pos := rl.GetMonitorPosition(i)
		w := float32(rl.GetMonitorWidth(i))
		h := float32(rl.GetMonitorHeight(i))
		if w <= 0 || h <= 0 {
			// Fallback for a monitor that fails to report geometry.
			w, h = 1920, 1080
		}

		raw = append(raw, rawMonitor{pos.X, pos.Y, w, h})

		left, top := pos.X, pos.Y
		right, bottom := pos.X+w, pos.Y+h
		if !haveBounds {
			minX, minY, maxX, maxY = left, top, right, bottom
			haveBounds = true
		} else {
			if left < minX {
				minX = left
			}
			if top < minY {
				minY = top
			}
			if right > maxX {
				maxX = right
			}
			if bottom > maxY {
				maxY = bottom
			}
		}
	}

	totalW := maxX - minX
	totalH := maxY - minY

	rl.SetWindowSize(int(totalW), int(totalH))
	rl.SetWindowPosition(int(minX), int(minY))

	monitorRects = monitorRects[:0]
	for _, m := range raw {
		monitorRects = append(monitorRects, TMonitorRect{
			X: m.x - minX,
			Y: m.y - minY,
			W: m.w,
			H: m.h,
		})
	}
}