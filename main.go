package main

import (
	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	screenWidth  = 640
	screenHeight = 480
)

var (
	fadeInVal   = float32(255.0)
)

func main() {
	runStory()
}

func setupApp() {
	// Borderless fullscreen - covers entire screen without changing resolution
	rl.SetConfigFlags(rl.FlagWindowUndecorated | rl.FlagWindowTopmost | rl.FlagMsaa4xHint)
	rl.InitWindow(screenWidth, screenHeight, "Johnny Castaway")

	mon := rl.GetCurrentMonitor()
	monW := rl.GetMonitorWidth(mon)
	monH := rl.GetMonitorHeight(mon)
	if monW <= 0 { monW = 1920 }
	if monH <= 0 { monH = 1080 }

	rl.SetWindowSize(monW, monH)
	rl.SetWindowPosition(0, 0)
	rl.DisableCursor()
	rl.HideCursor()

	rl.InitAudioDevice()
	loadSfx()

	rl.SetTargetFPS(30)

	parseResourceFiles("assets/RESOURCE.MAP")

	doFadeIn()
	graphicsInit()
}

func doFadeIn() {
	fadeInVal = 255.0

	for !rl.WindowShouldClose() {
		rl.BeginDrawing()

		rl.ClearBackground(rl.Blank)

		alpha := 1.0 - fadeInVal/255.0
		rl.DrawRectangle(0, 0, screenWidth, screenHeight, rl.Fade(rl.Black, alpha))
		fadeInVal -= 10

		if fadeInVal <= 0 {
			return
		}

		rl.EndDrawing()
	}
}

func runStory() {
	setupApp()
	defer rl.CloseWindow()
	defer rl.CloseAudioDevice()
	defer graphicsEnd()
	defer unloadSfx()

	storyPlay()
}

func singleTTM() {
	setupApp()
	defer rl.CloseWindow()
	defer graphicsEnd()
	for {
		adsPlaySingleTtm("MJFIRE.TTM")
	}
}
