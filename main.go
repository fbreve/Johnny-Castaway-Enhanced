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
		if args[1] == "browser" {
			assetBrowser()
			return
		}
	}

	runStory()
}

func runStory() {
	baseWindowScaleFactor := float32(1.5)
	rl.InitWindow(
		int32(float32(screenWidth)*baseWindowScaleFactor),
		int32(float32(screenHeight)*baseWindowScaleFactor),
		"Johnny Castaway - 34th Anniversary Edition",
	)
	defer rl.CloseWindow()
	rl.SetWindowState(rl.FlagWindowResizable)
	rl.SetTargetFPS(30)

	start := time.Now()
	parseResourceFiles("assets/RESOURCE.MAP")
	fmt.Println("elapsed => ", time.Now().Sub(start))

	graphicsInit()
	defer graphicsEnd()

	for {
		adsPlaySingleTtm("MJFIRE.TTM")
	}

	//for !rl.WindowShouldClose() {
	// WARNING:!!!
	// Ok, apparently I need to do all drawing in grUpdateDisplay so it's called at the right times
	// Which means, I can't allow Draw calls to nest
	// And additionally, I shouldn't allow draw calls to occur in multiple batches, as Raylib wants a single
	// Begin/End pair ultimately.

	//rl.BeginDrawing()

	//rl.ClearBackground(rl.SkyBlue)
	//rl.DrawText("Congrats! You created your first window!", 24, screenHeight-24, 20, rl.Black)

	//rl.EndDrawing()
	//}
}
