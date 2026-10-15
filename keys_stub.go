//go:build !windows

package main

import rl "github.com/gen2brain/raylib-go/raylib"

// vkToRaylib maps the Windows virtual-key codes used by the hotkey and
// screensaver exit logic to the equivalent Raylib keys.
var vkToRaylib = map[int]int32{
	0x10: rl.KeyLeftShift, // VK_SHIFT
	0x20: rl.KeySpace,     // VK_SPACE
	0x0D: rl.KeyEnter,     // VK_RETURN
	0x1B: rl.KeyEscape,    // VK_ESCAPE
	0x4D: rl.KeyM,         // VK_M
}

// isKeyDownGlobally has no focus-free global key state API outside Windows,
// so on other platforms it falls back to Raylib's window-focused input.
func isKeyDownGlobally(vk int) bool {
	if key, ok := vkToRaylib[vk]; ok {
		return rl.IsKeyDown(key)
	}
	// Printable letter virtual-key codes (0x41..0x5A) map directly.
	if vk >= 0x41 && vk <= 0x5A {
		return rl.IsKeyDown(rl.KeyA + int32(vk-0x41))
	}
	return false
}

func isAnyKeyPressedGlobally() bool {
	for key := int32(0); key < 512; key++ {
		if rl.IsKeyPressed(key) {
			return true
		}
	}
	return false
}
