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

type Mode int

const (
	None               Mode = 0
	TTMSingleModeStart Mode = 1
	TTMSingleModePoll  Mode = 2
	TTMSingleModeEnd   Mode = 3
	Delay              Mode = 4
)

var (
// screenBuffer       = image.NewRGBA(image.Rect(0, 0, screenWidth, screenHeight))
//
//	gScreen = func() *ebiten.Image {
//		img := ebiten.NewImage(ScreenWidth, ScreenHeight)
//		img.Fill(color.White)
//		return img
//	}()
//
// gGame *Game = nil
)

type Game struct {
	mode            Mode
	delayReturnMode Mode
	delayTicks      int
}

//func NewGame() ebiten.Game {
//	g := &Game{}
//	gGame = g
//	g.Init()
//	return g
//}

//func (g *Game) Init() {
//	graphicsInit()
//	//grLoadScreen("OCEAN00.SCR")
//	//grLoadScreen("ISLAND2.SCR")
//	//grLoadScreen("JOFFICE.SCR")
//	//grLoadScreen("NIGHT.SCR")
//	//grPutPixel(screenBuffer, 320, 240, 2)
//	//for i := 0; i < 16; i++ {
//	//	for j := 0; j < 4; j++ {
//	//		grDrawHorizontalLine(screenBuffer, 20, 600, 10+((4*int16(i))+int16(j)), uint8(i))
//	//
//	//	}
//	//}
//}

//func (g *Game) ChangeState(mode Mode) {
//	prevMode := g.mode
//	g.mode = mode
//
//	fmt.Printf("ChangeState prev => %v, new => %v\n", prevMode, mode)
//}

//func (g *Game) Update() error {
//	if g.IsKeyJustPressed() && g.mode == None {
//		//return ebiten.Termination
//		g.mode = TTMSingleModeStart
//	}
//	switch g.mode {
//	case None:
//		// Use this mode for testing things
//		storyPlay()
//	case TTMSingleModeStart:
//		//inverAdsPlaySingleTtmStart("MJFIRE.TTM")
//		// This one shows the big red ship pulling up, and the copy zone logic needs to paint a sliver
//		// of the red ship over the screen to obscure everything (at the end of the segment)
//		inverAdsPlaySingleTtmStart("GJVIS6.TTM") // scene 55 (in itch.io wasm version)
//	case TTMSingleModePoll:
//		inverAdsPlaySingleTtmPoll()
//		if grUpdateDelay != 0 {
//			g.delayReturnMode = TTMSingleModePoll
//			g.delayTicks = grUpdateDelay * 1
//			g.mode = Delay
//		}
//	case TTMSingleModeEnd:
//		inverAdsPlaySingleTtmEnd()
//	case Delay:
//		if g.delayTicks > 0 {
//			g.delayTicks -= 2
//		} else {
//			g.mode = g.delayReturnMode
//		}
//	default:
//		panic("unknown game mode!!!")
//	}
//
//	return nil
//}

//func (g *Game) Draw(screen *ebiten.Image) {
//screen.DrawImage(gScreen, &ebiten.DrawImageOptions{})
//screen.WritePixels(screenBuffer.Pix)

//if grBackgroundSur != nil {
//	screen.DrawImage(grBackgroundSur, &ebiten.DrawImageOptions{})
//}

// This was just for testing...it shows the sprites currently loaded, but it's not real code (debugging only)
// But I'm disabling this, as it's obscuring the real game visuals for now.
//if tmpSprites != nil {
//	opts := &ebiten.DrawImageOptions{}
//	for _, spr := range tmpSprites {
//		screen.DrawImage(spr, opts)
//		opts.GeoM.Translate(float64(spr.Bounds().Dx()), 0)
//	}
//}

//const (
//	xLoc = 10
//	yLoc = 20
//)

//if g.mode == None {
//	ebitenutil.DebugPrintAt(screen, "State => None", xLoc, yLoc)
//} else if g.mode == TTMSingleModeStart {
//	ebitenutil.DebugPrintAt(screen, "State => TTMSingleModeStart", xLoc, yLoc)
//} else if g.mode == TTMSingleModePoll {
//	ebitenutil.DebugPrintAt(screen, "State => TTMSingleModePoll", xLoc, yLoc)
//} else if g.mode == TTMSingleModeEnd {
//	ebitenutil.DebugPrintAt(screen, "State => TTMSingleModeEnd", xLoc, yLoc)
//} else if g.mode == Delay {
//	ebitenutil.DebugPrintAt(screen, "State => DELAY", xLoc, yLoc)
//} else {
//	ebitenutil.DebugPrintAt(screen, "State => ???", xLoc, yLoc)
//}

// special case troubleshooting
//ebitenutil.DebugPrint(screen, fmt.Sprintf("ip:%d, dataSize:%d", ttmThreads[0].ip, ttmSlots[0].dataSize))
//}

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
	//rl.SetConfigFlags(rl.FlagWindowTransparent)

	rl.InitWindow(screenWidth, screenHeight, "Johnny Castaway - 34th Anniversary Edition")
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
