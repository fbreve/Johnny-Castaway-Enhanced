package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
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
	isRun             = false
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
	rl.InitWindow(600, 520, "ScreenAntics - Setup")
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
	widescreen := config.Widescreen
	filterMode := config.FilterMode
	filterDropdownOpen := false
	scanlines := config.Scanlines
	silentTime := config.SilentTime
	silentStart := config.SilentStart
	silentEnd := config.SilentEnd

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
			font = rl.LoadFontEx(fontPath, 64, nil, 0)
			rl.SetTextureFilter(font.Texture, rl.FilterBilinear)
			fontLoaded = true
			break
		}
	}
	if fontLoaded {
		defer rl.UnloadFont(font)
	}

	drawText := func(text string, x, y int32, size float32, col rl.Color) {
		if fontLoaded {
			rl.DrawTextEx(font, text, rl.Vector2{X: float32(x), Y: float32(y)}, size, 0, col)
		} else {
			rl.DrawText(text, x, y, int32(size), col)
		}
	}

	measureText := func(text string, size float32) float32 {
		if fontLoaded {
			vec := rl.MeasureTextEx(font, text, size, 0)
			return vec.X
		}
		return float32(rl.MeasureText(text, int32(size)))
	}

	for !rl.WindowShouldClose() {
		// Update inputs
		mousePos := rl.GetMousePosition()
		click := rl.IsMouseButtonPressed(rl.MouseLeftButton)

		// Intercept clicks if the dropdown is open
		if filterDropdownOpen {
			optionsHover := mousePos.X >= 140 && mousePos.X <= 310 && mousePos.Y >= 316 && mousePos.Y <= 316+7*26
			if click {
				if optionsHover {
					clickedIdx := int(mousePos.Y-316) / 26
					if clickedIdx >= 0 && clickedIdx < 7 {
						filterMode = clickedIdx
					}
				}
				filterDropdownOpen = false
				click = false // Consume the click
			}
		}

		// Draw
		rl.BeginDrawing()
		rl.ClearBackground(rl.GetColor(0xf0f0f0ff)) // Standard Win32 light gray background

		// Groupbox "Setup"
		rl.DrawRectangleLines(15, 15, 570, 360, rl.Gray)
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

		// --- COLUMN 1 (x=30) ---

		// Load Background Checkbox
		bgHover := mousePos.X >= 30 && mousePos.X <= 280 && mousePos.Y >= 90 && mousePos.Y <= 125
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
		passHover := mousePos.X >= 30 && mousePos.X <= 280 && mousePos.Y >= 140 && mousePos.Y <= 175
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
		sndHover := mousePos.X >= 30 && mousePos.X <= 280 && mousePos.Y >= 190 && mousePos.Y <= 225
		rl.DrawRectangle(30, 195, 18, 18, rl.White)
		rl.DrawRectangleLines(30, 195, 18, 18, rl.Gray)
		if sounds {
			rl.DrawRectangle(34, 199, 10, 10, rl.GetColor(0x0078d7ff))
		}
		drawText("Sounds", 60, 196, 16, rl.Black)
		if sndHover && click {
			sounds = !sounds
		}

		// --- COLUMN 2 (x=320) ---

		// Widescreen Checkbox
		wsHover := mousePos.X >= 320 && mousePos.X <= 570 && mousePos.Y >= 90 && mousePos.Y <= 125
		rl.DrawRectangle(320, 95, 18, 18, rl.White)
		rl.DrawRectangleLines(320, 95, 18, 18, rl.Gray)
		if widescreen {
			rl.DrawRectangle(324, 99, 10, 10, rl.GetColor(0x0078d7ff))
		}
		drawText("Widescreen", 350, 96, 16, rl.Black)
		if wsHover && click {
			widescreen = !widescreen
		}

		// Software OpenGL Checkbox
		swHover := mousePos.X >= 320 && mousePos.X <= 570 && mousePos.Y >= 140 && mousePos.Y <= 175
		rl.DrawRectangle(320, 145, 18, 18, rl.White)
		rl.DrawRectangleLines(320, 145, 18, 18, rl.Gray)
		if useMesa {
			rl.DrawRectangle(324, 149, 10, 10, rl.GetColor(0x0078d7ff))
		}
		drawText("Use Software OpenGL (Mesa)", 350, 146, 16, rl.Black)
		if swHover && click {
			useMesa = !useMesa
		}

		// Independent instances checkbox
		miHover := mousePos.X >= 320 && mousePos.X <= 570 && mousePos.Y >= 190 && mousePos.Y <= 225
		rl.DrawRectangle(320, 195, 18, 18, rl.White)
		rl.DrawRectangleLines(320, 195, 18, 18, rl.Gray)
		if multiInstance {
			rl.DrawRectangle(324, 199, 10, 10, rl.GetColor(0x0078d7ff))
		}
		drawText("Independent instances", 350, 196, 16, rl.Black)
		if miHover && click {
			multiInstance = !multiInstance
		}

		// --- SILENT TIME ROW (y=240) ---
		stHover := mousePos.X >= 30 && mousePos.X <= 145 && mousePos.Y >= 235 && mousePos.Y <= 265
		rl.DrawRectangle(30, 240, 18, 18, rl.White)
		rl.DrawRectangleLines(30, 240, 18, 18, rl.Gray)
		if silentTime {
			rl.DrawRectangle(34, 244, 10, 10, rl.GetColor(0x0078d7ff))
		}
		drawText("Silent Time", 60, 241, 16, rl.Black)
		if stHover && click {
			silentTime = !silentTime
		}

		// Silent Time From/To interval selectors
		drawText("From:", 155, 241, 16, rl.Black)
		rl.DrawRectangle(200, 236, 95, 26, rl.White)
		rl.DrawRectangleLines(200, 236, 95, 26, rl.Gray)
		drawText(formatStartTime(silentStart), 205, 240, 15, rl.Black)

		// SilentStart Up button
		ssUpHover := mousePos.X >= 295 && mousePos.X <= 313 && mousePos.Y >= 236 && mousePos.Y <= 248
		ssUpCol := rl.GetColor(0xe1e1e1ff)
		if ssUpHover {
			ssUpCol = rl.GetColor(0xd1d1d1ff)
			if click {
				silentStart = adjustStartTime(silentStart, true)
			}
		}
		rl.DrawRectangle(295, 236, 18, 12, ssUpCol)
		rl.DrawRectangleLines(295, 236, 18, 12, rl.Gray)
		drawText("^", 300, 239, 14, rl.Black)

		// SilentStart Down button
		ssDownHover := mousePos.X >= 295 && mousePos.X <= 313 && mousePos.Y >= 250 && mousePos.Y <= 262
		ssDownCol := rl.GetColor(0xe1e1e1ff)
		if ssDownHover {
			ssDownCol = rl.GetColor(0xd1d1d1ff)
			if click {
				silentStart = adjustStartTime(silentStart, false)
			}
		}
		rl.DrawRectangle(295, 250, 18, 12, ssDownCol)
		rl.DrawRectangleLines(295, 250, 18, 12, rl.Gray)
		drawText("v", 301, 247, 12, rl.Black)

		drawText("To:", 325, 241, 16, rl.Black)
		rl.DrawRectangle(355, 236, 95, 26, rl.White)
		rl.DrawRectangleLines(355, 236, 95, 26, rl.Gray)
		drawText(formatStartTime(silentEnd), 360, 240, 15, rl.Black)

		// SilentEnd Up button
		seUpHover := mousePos.X >= 450 && mousePos.X <= 468 && mousePos.Y >= 236 && mousePos.Y <= 248
		seUpCol := rl.GetColor(0xe1e1e1ff)
		if seUpHover {
			seUpCol = rl.GetColor(0xd1d1d1ff)
			if click {
				silentEnd = adjustStartTime(silentEnd, true)
			}
		}
		rl.DrawRectangle(450, 236, 18, 12, seUpCol)
		rl.DrawRectangleLines(450, 236, 18, 12, rl.Gray)
		drawText("^", 455, 239, 14, rl.Black)

		// SilentEnd Down button
		seDownHover := mousePos.X >= 450 && mousePos.X <= 468 && mousePos.Y >= 250 && mousePos.Y <= 262
		seDownCol := rl.GetColor(0xe1e1e1ff)
		if seDownHover {
			seDownCol = rl.GetColor(0xd1d1d1ff)
			if click {
				silentEnd = adjustStartTime(silentEnd, false)
			}
		}
		rl.DrawRectangle(450, 250, 18, 12, seDownCol)
		rl.DrawRectangleLines(450, 250, 18, 12, rl.Gray)
		drawText("v", 456, 247, 12, rl.Black)

		// Scaling Filter Option (y=290)
		drawText("Scaling Filter:", 30, 295, 16, rl.Black)

		// Filter display box
		filterNames := []string{
			"Nearest",
			"Bilinear",
			"Sharp Bilinear",
			"CRT Dither",
			"Smart Dither",
			"Aperture Grille",
			"CRT Simulator",
		}

		// Draw header box
		rl.DrawRectangle(140, 290, 170, 26, rl.White)
		rl.DrawRectangleLines(140, 290, 170, 26, rl.Gray)
		drawText(filterNames[filterMode], 148, 294, 16, rl.Black)

		// Draw small down arrow box on the right
		rl.DrawRectangle(290, 291, 19, 24, rl.GetColor(0xe1e1e1ff))
		rl.DrawLine(290, 290, 290, 316, rl.Gray)
		drawText("v", 296, 297, 12, rl.Black)

		headerHover := mousePos.X >= 140 && mousePos.X <= 310 && mousePos.Y >= 290 && mousePos.Y <= 316
		if headerHover && click {
			filterDropdownOpen = !filterDropdownOpen
			click = false // Consume the click
		}

		// Scanlines Checkbox (Column 2, aligned with Scaling Filter)
		slHover := mousePos.X >= 320 && mousePos.X <= 570 && mousePos.Y >= 290 && mousePos.Y <= 325
		rl.DrawRectangle(320, 295, 18, 18, rl.White)
		rl.DrawRectangleLines(320, 295, 18, 18, rl.Gray)
		if scanlines {
			rl.DrawRectangle(324, 299, 10, 10, rl.GetColor(0x0078d7ff))
		}
		drawText("Scanlines", 350, 296, 16, rl.Black)
		if slHover && click {
			scanlines = !scanlines
		}

		// Skooter Blog branding link
		brandText := "Visite o Skooter Blog: www.skooterblog.com"
		brandSize := float32(16)
		brandWidth := measureText(brandText, brandSize)
		brandX := int32((600 - brandWidth) / 2)
		brandY := int32(395)

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
		okHover := mousePos.X >= 180 && mousePos.X <= 280 && mousePos.Y >= 430 && mousePos.Y <= 470
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
				config.Widescreen = widescreen
				config.FilterMode = filterMode
				config.Scanlines = scanlines
				config.SilentTime = silentTime
				config.SilentStart = silentStart
				config.SilentEnd = silentEnd
				cfgFileWrite(&config)
				break
			}
		}
		rl.DrawRectangle(180, 430, 100, 40, okCol)
		rl.DrawRectangleLines(180, 430, 100, 40, rl.Gray)
		drawText("OK", 218, 439, 16, rl.Black)

		// Cancel Button
		cancelHover := mousePos.X >= 320 && mousePos.X <= 420 && mousePos.Y >= 430 && mousePos.Y <= 470
		cancelCol := rl.GetColor(0xe1e1e1ff)
		if cancelHover {
			cancelCol = rl.GetColor(0xd1d1d1ff)
			if click {
				break
			}
		}
		rl.DrawRectangle(320, 430, 100, 40, cancelCol)
		rl.DrawRectangleLines(320, 430, 100, 40, rl.Gray)
		drawText("Cancel", 342, 439, 16, rl.Black)

		// Build Time stamp
		buildText := "Build: " + buildTime
		buildSize := float32(14)
		buildWidth := measureText(buildText, buildSize)
		buildX := int32((600 - buildWidth) / 2)
		buildY := int32(485)
		drawText(buildText, buildX, buildY, buildSize, rl.GetColor(0x444444ff))

		// Draw dropdown options overlay if open
		if filterDropdownOpen {
			// Draw dropdown background shadow/borders
			rl.DrawRectangle(140, 316, 170, 7*26, rl.White)
			rl.DrawRectangleLines(140, 316, 170, 7*26, rl.Gray)

			for i := 0; i < 7; i++ {
				optY := int32(316 + i*26)
				optHover := mousePos.X >= 140 && mousePos.X <= 310 && mousePos.Y >= float32(optY) && mousePos.Y <= float32(optY+26)

				if optHover {
					rl.DrawRectangle(141, optY, 168, 25, rl.GetColor(0x0078d7ff)) // Windows blue highlight
					drawText(filterNames[i], 148, optY+4, 16, rl.White)
				} else {
					drawText(filterNames[i], 148, optY+4, 16, rl.Black)
				}

				// Draw subtle separator lines between options
				if i < 6 {
					rl.DrawLine(140, optY+26, 310, optY+26, rl.GetColor(0xe0e0e0ff))
				}
			}
		}

		rl.EndDrawing()
	}
}

func main() {
	var isSettings = false
	var isPreview = false
	var isTest = false
	var isBench = false
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
		} else if strings.HasPrefix(argLower, "/b") || strings.HasPrefix(argLower, "-b") {
			isBench = true
		} else if strings.HasPrefix(argLower, "/k") || strings.HasPrefix(argLower, "-k") {
			// -k enables debug hotkeys: Space=pause, M=max-speed, Enter=advance, Esc=quit
			hotKeysEnabled = true
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
	if isBench {
		runBenchMode()
		os.Exit(0)
	}
	if isTest {
		runTestMode(testAdsName, testTagNo)
		os.Exit(0)
	}
	if isRun || (!isSettings && !isPreview && !isBench && !isTest) {
		isScreensaverMode = true
	}
	runStory()
}

func setupApp() {
	cfgFileRead(&activeConfig)
	startHDRWatchdog()

	// Enable 4x MSAA, undecorated, and resizable window flags before initialization to ensure window focus
	rl.SetConfigFlags(rl.FlagMsaa4xHint | rl.FlagWindowUndecorated | rl.FlagWindowResizable)
	rl.InitWindow(screenWidth, screenHeight, "Johnny Castaway")

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
	rl.SetMasterVolume(1.0)
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

	if config.MultiInstance && !hasMonitorIndex && isScreensaverMode {
		monitorCount := getMonitorCount()

		if monitorCount > 1 {
			// Spawn child processes for each monitor
			var wg sync.WaitGroup
			cmds := make([]*exec.Cmd, monitorCount)
			stdinPipes := make([]io.WriteCloser, monitorCount)
			shouldExitChan := make(chan struct{}, monitorCount)

			for i := 0; i < monitorCount; i++ {
				args := []string{"-m", fmt.Sprintf("%d", i)}
				if isRun {
					args = append(args, "-s")
				}
				if hotKeysEnabled {
					args = append(args, "-k")
				}
				cmd := exec.Command(os.Args[0], args...)
				
				pipe, err := cmd.StdinPipe()
				if err == nil {
					stdinPipes[i] = pipe
				}
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

			// Clean exit: signal all other child processes to terminate gracefully
			// by closing their stdin pipes. This causes their stdin reader loop to unblock
			// and trigger a standard Raylib/GLFW teardown to cleanly restore HDR and display settings.
			for _, pipe := range stdinPipes {
				if pipe != nil {
					_ = pipe.Close()
				}
			}

			wg.Wait()
			return
		}
	}

	if hasMonitorIndex {
		// Child process: listen to standard input to receive the exit signal from the parent.
		// When the parent closes the stdin pipe, Read returns immediately, triggering clean exit.
		go func() {
			buf := make([]byte, 1)
			_, _ = os.Stdin.Read(buf)
			shouldExitApp = true
		}()
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

func runBenchMode() {
	// Mirrors jc_reborn bench mode: loads the full app, runs adsPlayBench()
	// which measures rendering throughput for 1, 4, and 8 simultaneous
	// sprite layers. The results are printed to stdout, saved to bench.log,
	// and rendered directly on screen.
	fmt.Println("\nJohnny Castaway - Render Benchmark")
	fmt.Println("-----------------------------------")
	setupApp()
	defer rl.CloseWindow()
	defer rl.CloseAudioDevice()
	defer graphicsEnd()
	defer unloadSfx()

	results := adsPlayBench()

	// Output to console and log file
	var logOutput strings.Builder
	logOutput.WriteString("Johnny Castaway - Render Benchmark Results\n")
	logOutput.WriteString("-----------------------------------\n")
	for _, res := range results {
		fmt.Println(res)
		logOutput.WriteString(res + "\n")
	}
	logOutput.WriteString("-----------------------------------\n")
	_ = os.WriteFile("bench.log", []byte(logOutput.String()), 0644)

	fmt.Println("-----------------------------------")
	fmt.Println("Benchmark complete. Results saved to bench.log. Press any key to exit.")

	// Keep the window alive to show results on screen
	rects := monitorRects
	if len(rects) == 0 {
		rects = []TMonitorRect{{X: 0, Y: 0, W: float32(rl.GetScreenWidth()), H: float32(rl.GetScreenHeight())}}
	}

	for !rl.WindowShouldClose() && !shouldExitApp {
		rl.BeginDrawing()
		rl.ClearBackground(rl.Black)

		for _, m := range rects {
			rl.DrawText("Johnny Castaway - Render Benchmark Results", int32(m.X)+60, int32(m.Y)+80, 20, rl.RayWhite)
			rl.DrawText("--------------------------------------------------", int32(m.X)+60, int32(m.Y)+110, 20, rl.Gray)

			y := int32(140)
			for _, res := range results {
				rl.DrawText(res, int32(m.X)+60, int32(m.Y)+y, 20, rl.Green)
				y += 30
			}

			rl.DrawText("--------------------------------------------------", int32(m.X)+60, int32(m.Y)+y, 20, rl.Gray)
			rl.DrawText("Results saved to bench.log.", int32(m.X)+60, int32(m.Y)+y+30, 18, rl.LightGray)
			rl.DrawText("Press any key to exit.", int32(m.X)+60, int32(m.Y)+y+60, 18, rl.Yellow)
		}

		rl.EndDrawing()
		if rl.GetKeyPressed() != 0 {
			break
		}
	}
}

func runTestMode(testAdsName string, testTagNo int) {
	setupApp()
	defer rl.CloseWindow()
	defer rl.CloseAudioDevice()
	defer graphicsEnd()
	defer unloadSfx()

	adsInit()
	storyCurrentDay = activeConfig.CurrentDay
	islandState.xPos = 0
	islandState.yPos = 0
	islandState.lowTide = 0
	islandState.raft = 0

	// Find the scene in storyScenes so test mode can mirror its actual story-day,
	// raft stage, tide eligibility, and island positioning instead of using a
	// generic hardcoded island setup.
	var scene TStoryScene
	found := false
	if testAdsName != "" && testTagNo > 0 {
		testAdsName = strings.ToUpper(testAdsName)
		if !strings.HasSuffix(testAdsName, ".ADS") {
			testAdsName += ".ADS"
		}

		for _, s := range storyScenes {
			if strings.ToUpper(s.adsName) == testAdsName && int(s.adsTagNo) == testTagNo {
				scene = s
				found = true
				break
			}
		}

		if found {
			if scene.dayNo != 0 {
				storyCurrentDay = scene.dayNo
			}
			storyCalculateIslandFromScene(&scene)
			if scene.flags&ISLAND == ISLAND {
				xOffset := 0
				if scene.flags&LEFT_ISLAND == LEFT_ISLAND {
					xOffset = 272
				}
				ttmDx = islandState.xPos + xOffset
				ttmDy = islandState.yPos
			} else {
				ttmDx = 0
				ttmDy = 0
			}
		} else {
			islandState.xPos = 0
			islandState.yPos = 0
			islandState.lowTide = 0
			islandState.raft = 0
			ttmDx = 0
			ttmDy = 0
		}
	}

	// r.c. - previously called unconditionally, which meant testing any
	// non-ISLAND FINAL scene (JOHNNY.ADS tag 1 "The End", tag 6) via -t
	// always started the background wave thread and clouds thread even
	// though those scenes' own TTM scripts never draw either one, and
	// storyPlay() would never call adsInitIsland() for them either. Mirror
	// storyPlay()'s own choice here so test mode matches real playback.
	if !found || scene.flags&ISLAND == ISLAND {
		adsInitIsland()
	} else {
		adsNoIsland()
	}

	if testAdsName != "" && testTagNo > 0 {
		fmt.Printf("Running custom test mode for scene: %s tag %d (LEFT_ISLAND=%v, xPos=%d)\n", testAdsName, testTagNo, islandState.xPos == -272, islandState.xPos)
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

func openURL(url string) {
	var err error
	switch runtime.GOOS {
	case "windows":
		err = exec.Command("cmd", "/c", "start", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default: // "linux", etc.
		err = exec.Command("xdg-open", url).Start()
	}
	if err != nil {
		fmt.Println("failed to open URL: ", err)
	}
}
