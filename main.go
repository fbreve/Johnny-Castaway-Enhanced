package main

import (
	"fmt"
	rl "github.com/gen2brain/raylib-go/raylib"
	"os"
	"time"
)

const (
	screenWidth  = 640
	screenHeight = 480
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
		}
	}

	runStory()
}

func setupApp() {
	baseWindowScaleFactor := float32(1.5)
	rl.InitWindow(
		int32(float32(screenWidth)*baseWindowScaleFactor),
		int32(float32(screenHeight)*baseWindowScaleFactor),
		"Johnny Castaway - 34th Anniversary Edition",
	)

	rl.SetWindowState(rl.FlagWindowResizable)
	rl.SetTargetFPS(30)

	start := time.Now()
	parseResourceFiles("assets/RESOURCE.MAP")
	fmt.Println("elapsed => ", time.Now().Sub(start))

	graphicsInit()
}

func runStory() {
	setupApp()
	defer rl.CloseWindow()
	defer graphicsEnd()

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
