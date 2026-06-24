package main

import (
	"fmt"
	"os"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	screenWidth  = 640
	screenHeight = 480
)

var (
	fadeInVal   = float32(255.0)
)

func formatStartTime(val int) string {
	hour := val / 100
	min := val % 100
	period := "AM"
	if hour >= 12 {
		period = "PM"
	}
	displayHour := hour
	if hour == 0 {
		displayHour = 12
	} else if hour > 12 {
		displayHour = hour - 12
	}
	return fmt.Sprintf("%02d:%02d %s", displayHour, min, period)
}

func adjustStartTime(val int, up bool) int {
	hour := val / 100
	min := val % 100

	totalMin := hour*60 + min
	if up {
		totalMin += 30
	} else {
		totalMin -= 30
	}

	// Wrap around 24 hours
	if totalMin >= 24*60 {
		totalMin = 0
	} else if totalMin < 0 {
		totalMin = 23*60 + 30
	}

	newHour := totalMin / 60
	newMin := totalMin % 60
	return newHour*100 + newMin
}

func runOptionsWindow() {
	rl.InitWindow(400, 350, "ScreenAntics - Setup")
	defer rl.CloseWindow()
	rl.SetTargetFPS(60)

	var config TConfig
	cfgFileRead(&config)

	background := config.Background
	sounds := config.Sounds
	password := config.Password
	startTime := config.StartTime

	// Load Windows native fonts for gorgeous anti-aliased text
	var font rl.Font
	fontLoaded := false
	for _, fontPath := range []string{"C:\\Windows\\Fonts\\segoeui.ttf", "C:\\Windows\\Fonts\\arial.ttf"} {
		if _, err := os.Stat(fontPath); err == nil {
			font = rl.LoadFontEx(fontPath, 36, nil, 0)
			fontLoaded = true
			break
		}
	}
	if fontLoaded {
		defer rl.UnloadFont(font)
	}

	drawText := func(text string, x, y int32, size float32, col rl.Color) {
		if fontLoaded {
			rl.DrawTextEx(font, text, rl.Vector2{X: float32(x), Y: float32(y)}, size, 1, col)
		} else {
			rl.DrawText(text, x, y, int32(size), col)
		}
	}

	for !rl.WindowShouldClose() {
		// Update inputs
		mousePos := rl.GetMousePosition()
		click := rl.IsMouseButtonPressed(rl.MouseLeftButton)

		// Draw
		rl.BeginDrawing()
		rl.ClearBackground(rl.GetColor(0xf0f0f0ff)) // Standard Win32 light gray background

		// Groupbox "Setup"
		rl.DrawRectangleLines(15, 15, 370, 250, rl.Gray)
		rl.DrawRectangle(25, 5, 55, 20, rl.GetColor(0xf0f0f0ff))
		drawText("Setup", 30, 6, 16, rl.Black)

		// Start of Day Option
		drawText("Start of Day:", 30, 45, 16, rl.Black)

		// Time display box
		rl.DrawRectangle(140, 40, 110, 26, rl.White)
		rl.DrawRectangleLines(140, 40, 110, 26, rl.Gray)
		drawText(formatStartTime(startTime), 148, 44, 15, rl.Black)

		// Time Up/Down Arrow buttons
		// Up button
		upHover := mousePos.X >= 255 && mousePos.X <= 275 && mousePos.Y >= 40 && mousePos.Y <= 52
		upCol := rl.GetColor(0xe1e1e1ff)
		if upHover {
			upCol = rl.GetColor(0xd1d1d1ff)
			if click {
				startTime = adjustStartTime(startTime, true)
			}
		}
		rl.DrawRectangle(255, 40, 20, 12, upCol)
		rl.DrawRectangleLines(255, 40, 20, 12, rl.Gray)
		drawText("▲", 260, 41, 10, rl.Black)

		// Down button
		downHover := mousePos.X >= 255 && mousePos.X <= 275 && mousePos.Y >= 54 && mousePos.Y <= 66
		downCol := rl.GetColor(0xe1e1e1ff)
		if downHover {
			downCol = rl.GetColor(0xd1d1d1ff)
			if click {
				startTime = adjustStartTime(startTime, false)
			}
		}
		rl.DrawRectangle(255, 54, 20, 12, downCol)
		rl.DrawRectangleLines(255, 54, 20, 12, rl.Gray)
		drawText("▼", 260, 54, 10, rl.Black)

		// Load Background Checkbox
		bgHover := mousePos.X >= 30 && mousePos.X <= 350 && mousePos.Y >= 90 && mousePos.Y <= 120
		rl.DrawRectangle(30, 95, 18, 18, rl.White)
		rl.DrawRectangleLines(30, 95, 18, 18, rl.Gray)
		if background {
			rl.DrawRectangle(34, 99, 10, 10, rl.GetColor(0x0078d7ff))
		}
		drawText("Load Background", 60, 96, 16, rl.Black)
		if bgHover && click {
			background = !background
		}

		// Password Checkbox
		passHover := mousePos.X >= 30 && mousePos.X <= 350 && mousePos.Y >= 140 && mousePos.Y <= 170
		rl.DrawRectangle(30, 145, 18, 18, rl.White)
		rl.DrawRectangleLines(30, 145, 18, 18, rl.Gray)
		if password {
			rl.DrawRectangle(34, 149, 10, 10, rl.GetColor(0x0078d7ff))
		}
		drawText("Password Protection", 60, 146, 16, rl.Black)
		if passHover && click {
			password = !password
		}

		// Sounds Checkbox
		sndHover := mousePos.X >= 30 && mousePos.X <= 350 && mousePos.Y >= 190 && mousePos.Y <= 220
		rl.DrawRectangle(30, 195, 18, 18, rl.White)
		rl.DrawRectangleLines(30, 195, 18, 18, rl.Gray)
		if sounds {
			rl.DrawRectangle(34, 199, 10, 10, rl.GetColor(0x0078d7ff))
		}
		drawText("Sounds", 60, 196, 16, rl.Black)
		if sndHover && click {
			sounds = !sounds
		}

		// OK Button
		okHover := mousePos.X >= 80 && mousePos.X <= 180 && mousePos.Y >= 285 && mousePos.Y <= 325
		okCol := rl.GetColor(0xe1e1e1ff)
		if okHover {
			okCol = rl.GetColor(0xd1d1d1ff)
			if click {
				config.Background = background
				config.Sounds = sounds
				config.Password = password
				config.StartTime = startTime
				cfgFileWrite(&config)
				break
			}
		}
		rl.DrawRectangle(80, 285, 100, 40, okCol)
		rl.DrawRectangleLines(80, 285, 100, 40, rl.Gray)
		drawText("OK", 118, 294, 16, rl.Black)

		// Cancel Button
		cancelHover := mousePos.X >= 220 && mousePos.X <= 320 && mousePos.Y >= 285 && mousePos.Y <= 325
		cancelCol := rl.GetColor(0xe1e1e1ff)
		if cancelHover {
			cancelCol = rl.GetColor(0xd1d1d1ff)
			if click {
				break
			}
		}
		rl.DrawRectangle(220, 285, 100, 40, cancelCol)
		rl.DrawRectangleLines(220, 285, 100, 40, rl.Gray)
		drawText("Cancel", 246, 294, 16, rl.Black)

		rl.EndDrawing()
	}
}

func main() {
	var isSettings = false
	var isPreview = false
	var isRun = false

	for _, arg := range os.Args {
		argLower := strings.ToLower(arg)
		if strings.HasPrefix(argLower, "/c") || strings.HasPrefix(argLower, "-c") {
			isSettings = true
		} else if strings.HasPrefix(argLower, "/p") || strings.HasPrefix(argLower, "-p") {
			isPreview = true
		} else if strings.HasPrefix(argLower, "/s") || strings.HasPrefix(argLower, "-s") {
			isRun = true
		}
	}

	if isSettings {
		runOptionsWindow()
		os.Exit(0)
	}
	if isPreview {
		os.Exit(0)
	}
	if isRun {
		isScreensaverMode = true
	}
	runStory()
}

func setupApp() {
	cfgFileRead(&activeConfig)
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

	for !rl.WindowShouldClose() && !shouldExitApp {
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
