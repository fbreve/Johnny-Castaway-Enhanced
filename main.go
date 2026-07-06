package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	screenWidth  = 640
	screenHeight = 480

	// The game's logical scene coordinate space. Scripts use 0–639 × 0–349.
	// The bottom 130 rows of the 480-tall window are unused by game content.
	gameWidth  = 640
	gameHeight = 350
)

var (
	fadeInVal         = float32(255.0)
	runOnMonitorIndex int
	hasMonitorIndex   bool
	buildTime         = "Developer Build"
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
	rl.SetConfigFlags(rl.FlagWindowHighdpi)
	rl.InitWindow(600, 500, "ScreenAntics - Setup")
	defer rl.CloseWindow()
	rl.SetTargetFPS(60)

	var config TConfig
	cfgFileRead(&config)

	background := config.Background
	sounds := config.Sounds
	password := config.Password
	startTime := config.StartTime
	useMesa := config.UseMesa
	multiInstance := config.MultiInstance

	// Load Windows native fonts for gorgeous anti-aliased text
	var font rl.Font
	fontLoaded := false
	winDir := os.Getenv("SystemRoot")
	if winDir == "" {
		winDir = os.Getenv("windir")
	}
	if winDir == "" {
		winDir = "C:\\Windows"
	}
	fontPaths := []string{
		winDir + "\\Fonts\\segoeui.ttf",
		winDir + "\\Fonts\\arial.ttf",
		"C:\\Windows\\Fonts\\segoeui.ttf",
		"C:\\Windows\\Fonts\\arial.ttf",
	}
	for _, fontPath := range fontPaths {
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
		rl.DrawRectangleLines(15, 15, 570, 340, rl.Gray)
		rl.DrawRectangle(25, 5, 65, 20, rl.GetColor(0xf0f0f0ff))
		drawText("Setup", 30, 6, 16, rl.Black)

		// Start of Day Option
		drawText("Start of Day:", 30, 45, 16, rl.Black)

		// Time display box
		rl.DrawRectangle(140, 40, 110, 26, rl.White)
		rl.DrawRectangleLines(140, 40, 110, 26, rl.Gray)
		drawText(formatStartTime(startTime), 148, 44, 16, rl.Black)

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
		drawText("^", 260, 43, 14, rl.Black)

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
		drawText("v", 261, 51, 12, rl.Black)

		// Load Background Checkbox
		bgHover := mousePos.X >= 30 && mousePos.X <= 550 && mousePos.Y >= 90 && mousePos.Y <= 120
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
		passHover := mousePos.X >= 30 && mousePos.X <= 550 && mousePos.Y >= 140 && mousePos.Y <= 170
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
		sndHover := mousePos.X >= 30 && mousePos.X <= 550 && mousePos.Y >= 190 && mousePos.Y <= 220
		rl.DrawRectangle(30, 195, 18, 18, rl.White)
		rl.DrawRectangleLines(30, 195, 18, 18, rl.Gray)
		if sounds {
			rl.DrawRectangle(34, 199, 10, 10, rl.GetColor(0x0078d7ff))
		}
		drawText("Sounds", 60, 196, 16, rl.Black)
		if sndHover && click {
			sounds = !sounds
		}

		// Software OpenGL Checkbox
		swHover := mousePos.X >= 30 && mousePos.X <= 550 && mousePos.Y >= 240 && mousePos.Y <= 270
		rl.DrawRectangle(30, 245, 18, 18, rl.White)
		rl.DrawRectangleLines(30, 245, 18, 18, rl.Gray)
		if useMesa {
			rl.DrawRectangle(34, 249, 10, 10, rl.GetColor(0x0078d7ff))
		}
		drawText("Use Software OpenGL (Mesa)", 60, 246, 16, rl.Black)
		if swHover && click {
			useMesa = !useMesa
		}

		// Independent instances checkbox
		miHover := mousePos.X >= 30 && mousePos.X <= 550 && mousePos.Y >= 290 && mousePos.Y <= 320
		rl.DrawRectangle(30, 295, 18, 18, rl.White)
		rl.DrawRectangleLines(30, 295, 18, 18, rl.Gray)
		if multiInstance {
			rl.DrawRectangle(34, 299, 10, 10, rl.GetColor(0x0078d7ff))
		}
		drawText("Independent instances per monitor", 60, 296, 16, rl.Black)
		if miHover && click {
			multiInstance = !multiInstance
		}

		// Skooter Blog branding link
		brandText := "Visite o Skooter Blog: www.skooterblog.com"
		brandSize := float32(16)
		brandWidth := measureText(brandText, brandSize)
		brandX := int32((600 - brandWidth) / 2)
		brandY := int32(375)

		brandHover := mousePos.X >= float32(brandX) && mousePos.X <= float32(brandX)+brandWidth &&
			mousePos.Y >= float32(brandY-4) && mousePos.Y <= float32(brandY+18)

		brandCol := rl.GetColor(0x555555ff)
		if brandHover {
			brandCol = rl.GetColor(0x0066ccff)
			rl.SetMouseCursor(rl.MouseCursorPointingHand)
			if click {
				openURL("https://www.skooterblog.com/")
			}
		} else {
			rl.SetMouseCursor(rl.MouseCursorDefault)
		}

		drawText(brandText, brandX, brandY, brandSize, brandCol)
		if brandHover {
			rl.DrawLine(brandX, brandY+15, brandX+int32(brandWidth), brandY+15, brandCol)
		}

		// OK Button
		okHover := mousePos.X >= 180 && mousePos.X <= 280 && mousePos.Y >= 410 && mousePos.Y <= 450
		okCol := rl.GetColor(0xe1e1e1ff)
		if okHover {
			okCol = rl.GetColor(0xd1d1d1ff)
			if click {
				config.Background = background
				config.Sounds = sounds
				config.Password = password
				config.StartTime = startTime
				config.UseMesa = useMesa
				config.MultiInstance = multiInstance
				cfgFileWrite(&config)
				break
			}
		}
		rl.DrawRectangle(180, 410, 100, 40, okCol)
		rl.DrawRectangleLines(180, 410, 100, 40, rl.Gray)
		drawText("OK", 218, 419, 16, rl.Black)

		// Cancel Button
		cancelHover := mousePos.X >= 320 && mousePos.X <= 420 && mousePos.Y >= 410 && mousePos.Y <= 450
		cancelCol := rl.GetColor(0xe1e1e1ff)
		if cancelHover {
			cancelCol = rl.GetColor(0xd1d1d1ff)
			if click {
				break
			}
		}
		rl.DrawRectangle(320, 410, 100, 40, cancelCol)
		rl.DrawRectangleLines(320, 410, 100, 40, rl.Gray)
		drawText("Cancel", 342, 419, 16, rl.Black)

		// Build Time stamp
		buildText := "Build: " + buildTime
		buildSize := float32(14)
		buildWidth := measureText(buildText, buildSize)
		buildX := int32((600 - buildWidth) / 2)
		buildY := int32(465)
		drawText(buildText, buildX, buildY, buildSize, rl.GetColor(0x444444ff))
		rl.EndDrawing()
	}
}

func main() {
	var isSettings = false
	var isPreview = false
	var isRun = false
	var isTest = false
	var testAdsName = ""
	var testTagNo = 0

	for i, arg := range os.Args {
		argLower := strings.ToLower(arg)
		if strings.HasPrefix(argLower, "/c") || strings.HasPrefix(argLower, "-c") {
			isSettings = true
		} else if strings.HasPrefix(argLower, "/p") || strings.HasPrefix(argLower, "-p") {
			isPreview = true
		} else if strings.HasPrefix(argLower, "/s") || strings.HasPrefix(argLower, "-s") {
			isRun = true
		} else if strings.HasPrefix(argLower, "/t") || strings.HasPrefix(argLower, "-t") {
			isTest = true
			if i+1 < len(os.Args) {
				testAdsName = os.Args[i+1]
			}
			if i+2 < len(os.Args) {
				fmt.Sscanf(os.Args[i+2], "%d", &testTagNo)
			}
		} else if strings.HasPrefix(argLower, "/m") || strings.HasPrefix(argLower, "-m") {
			if i+1 < len(os.Args) {
				fmt.Sscanf(os.Args[i+1], "%d", &runOnMonitorIndex)
				hasMonitorIndex = true
			}
		}
	}

	var initialConfig TConfig
	cfgFileRead(&initialConfig)
	if !isSettings && !initialConfig.UseMesa {
		preloadNativeOpenGL()
	}

	if isSettings {
		runOptionsWindow()
		os.Exit(0)
	}
	if isPreview {
		os.Exit(0)
	}
	if isTest {
		runTestMode(testAdsName, testTagNo)
		os.Exit(0)
	}
	if isRun {
		isScreensaverMode = true
	}
	runStory()
}

func setupApp() {
	cfgFileRead(&activeConfig)
	if hasMonitorIndex && runOnMonitorIndex != 0 {
		activeConfig.Sounds = false
	}
	// Enable 4x MSAA for smoother, anti-aliased graphics
	rl.SetConfigFlags(rl.FlagMsaa4xHint)
	// Initialize with default standard window flags to ensure 100% OpenGL context creation compatibility on all hardware/drivers.
	rl.InitWindow(screenWidth, screenHeight, "Johnny Castaway")

	// Apply Undecorated and Resizable states dynamically after the window is successfully initialized.
	rl.SetWindowState(rl.FlagWindowUndecorated | rl.FlagWindowResizable)

	if !rl.IsWindowReady() {
		panic("Fatal: Failed to initialize window. Please check your OpenGL/graphics drivers.")
	}

	// r.c. - spans the window across every connected monitor (not just the
	// current one) and records each monitor's own rectangle for the
	// renderer to draw a separate copy of the scene into. On a
	// single-monitor system this behaves exactly like the previous code.
	setupMonitors()
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
		// r.c. - use the actual current window size, not the fixed 640x480
		// game-resolution constants. After setupMonitors() the window can
		// span multiple monitors and be much larger than 640x480; filling
		// only that fixed corner would leave the rest of the window
		// showing through as blank during this initial fade-in.
		rl.DrawRectangle(0, 0, int32(rl.GetScreenWidth()), int32(rl.GetScreenHeight()), rl.Fade(rl.Black, alpha))
		fadeInVal -= 10

		if fadeInVal <= 0 {
			return
		}

		rl.EndDrawing()
	}
}

func runStory() {
	var config TConfig
	cfgFileRead(&config)

	if config.MultiInstance && !hasMonitorIndex {
		// Initialize a tiny hidden window to query monitors
		rl.SetConfigFlags(rl.FlagWindowHidden)
		rl.InitWindow(1, 1, "Johnny Parent")
		monitorCount := rl.GetMonitorCount()
		rl.CloseWindow()

		if monitorCount > 1 {
			// Spawn child processes for each monitor
			var wg sync.WaitGroup
			cmds := make([]*exec.Cmd, monitorCount)
			shouldExitChan := make(chan struct{}, monitorCount)

			for i := 0; i < monitorCount; i++ {
				args := []string{"-s", "-m", fmt.Sprintf("%d", i)}
				cmd := exec.Command(os.Args[0], args...)
				cmds[i] = cmd

				wg.Add(1)
				go func(index int, c *exec.Cmd) {
					defer wg.Done()
					err := c.Run()
					if err != nil {
						fmt.Printf("Instance %d exited: %v\n", index, err)
					}
					select {
					case shouldExitChan <- struct{}{}:
					default:
					}
				}(i, cmd)
			}

			// Wait for any child process to exit
			<-shouldExitChan

			// Terminate all other child processes
			for _, c := range cmds {
				if c != nil && c.Process != nil {
					_ = c.Process.Kill()
				}
			}

			wg.Wait()
			return
		}
	}

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

func runTestMode(testAdsName string, testTagNo int) {
	setupApp()
	defer rl.CloseWindow()
	defer rl.CloseAudioDevice()
	defer graphicsEnd()
	defer unloadSfx()

	adsInit()
	islandState.xPos = 0
	islandState.yPos = 0
	islandState.lowTide = 1 // Ensure low tide so they match the walk paths
	adsInitIsland()

	if testAdsName != "" && testTagNo > 0 {
		testAdsName = strings.ToUpper(testAdsName)
		if !strings.HasSuffix(testAdsName, ".ADS") {
			testAdsName += ".ADS"
		}
		fmt.Printf("Running custom test mode for scene: %s tag %d\n", testAdsName, testTagNo)
		for !shouldExitApp {
			adsPlay(testAdsName, uint16(testTagNo))
		}
	} else {
		for !shouldExitApp {
			// Play tree climb and dive (ACTIVITY.ADS tag 4)
			adsPlay("ACTIVITY.ADS", 4)
			if shouldExitApp {
				break
			}
			// Play water return (JOHNNY.ADS tag 3)
			adsPlay("JOHNNY.ADS", 3)
		}
	}
}
