package main

import (
	"fmt"
	rl "github.com/gen2brain/raylib-go/raylib"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	screenWidth  = 640
	screenHeight = 480
)

var (
	fadeInVal   = float32(255.0)
	displayFlag = ""
)

func main() {
	args := os.Args
	if len(args) > 1 {
		arg := args[1]

		switch arg {
		case "browser":
			assetBrowser()
			return
		case "ttm":
			singleTTM()
			return
		case "display":
			displayFlag = args[2]
		}
	}

	runStory()
}

func setupApp() {
	var providedMonNum int64 = -1
	var providedWidthNum int64 = -1
	var providedHeightNum int64 = -1

	if displayFlag != "" {
		colonLoc := strings.IndexAny(displayFlag, ":")
		byIndexLoc := strings.IndexAny(displayFlag, "x")

		val1, err := strconv.ParseInt(displayFlag[0:colonLoc], 10, 64)
		if err != nil {
			panic(fmt.Errorf("failed to parse providedMonNum as int: %w", err))
		}
		providedMonNum = val1

		val2, err := strconv.ParseInt(displayFlag[colonLoc+1:byIndexLoc], 10, 64)
		if err != nil {
			panic(fmt.Errorf("failed to parse providedWidthNum as int: %w", err))
		}
		providedWidthNum = val2

		val3, err := strconv.ParseInt(displayFlag[byIndexLoc+1:], 10, 64)
		if err != nil {
			panic(fmt.Errorf("failed to parse providedHeightNum as int: %w", err))
		}
		providedHeightNum = val3

		fmt.Printf("monNum:%d, monWidth:%d, monHeight:%d\n", providedMonNum, providedWidthNum, providedHeightNum)
	}

	if providedMonNum != -1 {
		// full screen at startup!
		rl.SetConfigFlags(rl.FlagWindowUndecorated | rl.FlagWindowTopmost) //| rl.FlagWindowTransparent) //| rl.FlagFullscreenMode)

		rl.InitWindow(
			int32(providedWidthNum),
			int32(providedHeightNum),
			"Johnny Castaway - 34th Anniversary Edition",
		)

		//posX := (*providedWidthNum - screenWidth) / 2
		//posY := (*providedHeightNum - screenHeight) / 2
		//rl.SetWindowPosition(0, -22)
		rl.DisableCursor()
		rl.HideCursor()

		rl.SetWindowMonitor(int(providedMonNum))
		rl.SetWindowFocused()

		// NOTE: Important!!! After a small delay we ask for full screen, once we're set to the right monitor above
		// we shouldn't have vsync issues, because we enter full screen on the correct monitor!
		//time.Sleep(time.Millisecond * 100)
		//rl.ToggleFullscreen()
	} else {
		rl.SetConfigFlags(rl.FlagWindowResizable | rl.FlagWindowTransparent | rl.FlagWindowUndecorated)

		baseWindowScaleFactor := float32(1.0)
		rl.InitWindow(
			int32(float32(screenWidth)*baseWindowScaleFactor),
			int32(float32(screenHeight)*baseWindowScaleFactor),
			"Johnny Castaway - 34th Anniversary Edition",
		)
	}
	rl.InitAudioDevice()
	loadSfx()

	rl.SetTargetFPS(30)

	start := time.Now()
	parseResourceFiles("assets/RESOURCE.MAP")
	fmt.Println("elapsed => ", time.Now().Sub(start))

	doFadeIn()
	graphicsInit()
}

func doFadeIn() {
	fadeInVal = 255.0

	for !rl.WindowShouldClose() {
		rl.BeginDrawing()

		rl.ClearBackground(rl.Blank)

		alpha := 1.0 - fadeInVal/255.0
		fmt.Println(alpha)
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
