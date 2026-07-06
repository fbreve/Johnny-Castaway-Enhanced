package main

import (
	rl "github.com/gen2brain/raylib-go/raylib"
)

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
	if hasMonitorIndex {
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