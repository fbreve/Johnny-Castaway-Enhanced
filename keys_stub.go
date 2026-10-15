//go:build !windows

package main

import rl "github.com/gen2brain/raylib-go/raylib"

// isKeyDownGlobally mirrors the small set of Windows virtual-key queries used
// by the shared hotkey code. Outside Windows, Raylib can only report input
// delivered to this window; Wayland intentionally provides no focus-free
// global keyboard state API.
func isKeyDownGlobally(vk int) bool {
	switch vk {
	case 0x10: // VK_SHIFT: either physical Shift key
		return rl.IsKeyDown(rl.KeyLeftShift) || rl.IsKeyDown(rl.KeyRightShift)
	case 0x20: // VK_SPACE
		return rl.IsKeyDown(rl.KeySpace)
	case 0x0D: // VK_RETURN
		return rl.IsKeyDown(rl.KeyEnter)
	case 0x1B: // VK_ESCAPE
		return rl.IsKeyDown(rl.KeyEscape)
	case 0x4D: // VK_M
		return rl.IsKeyDown(rl.KeyM)
	}

	// Printable letter virtual-key codes (0x41..0x5A) map directly.
	if vk >= 0x41 && vk <= 0x5A {
		return rl.IsKeyDown(rl.KeyA + int32(vk-0x41))
	}
	return false
}

// isAnyKeyPressedGlobally reports whether Raylib received a key press for this
// window. GetKeyPressed drains Raylib's key queue and avoids probing arbitrary
// integer values as if every value in 0..511 were a valid KeyboardKey.
func isAnyKeyPressedGlobally() bool {
	return rl.GetKeyPressed() != 0
}
