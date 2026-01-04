package main

import (
	"fmt"
	"image/color"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
)

var ttmPaletteInternal = [16][4]uint8{}
var bmpIdx int
var spriteIdx float32 = 0

func unloadSprites(sprites []*rl.Texture2D) {
	for _, spr := range sprites {
		rl.UnloadTexture(*spr)
	}
}

func assetBrowser() {
	rl.SetConfigFlags(rl.FlagWindowTransparent)

	rl.InitWindow(screenWidth, screenHeight, "Johnny Castaway - 34th Anniversary Edition")
	defer rl.CloseWindow()
	rl.SetWindowState(rl.FlagWindowResizable) //| rl.FlagWindowUndecorated)
	rl.SetTargetFPS(30)

	start := time.Now()
	parseResourceFiles("assets/RESOURCE.MAP")
	fmt.Println("elapsed => ", time.Since(start))

	palResource := palResources[0]
	for i := 0; i < 16; i++ {
		ttmPaletteInternal[i][0] = palResource.Colors[i].B << 2
		ttmPaletteInternal[i][1] = palResource.Colors[i].G << 2
		ttmPaletteInternal[i][2] = palResource.Colors[i].R << 2
		ttmPaletteInternal[i][3] = 0
	}

	scrTexture := loadScrImg("ISLETEMP.SCR") // JOFFICE.SCR, ISLETEMP.SCR, NIGHT.SCR, SUZBEACH.SCR, INTRO.SCR, OCEAN0{0,1,2}.SCR
	tmpSprites := loadBitmapImg(bmpIdx)

	for !rl.WindowShouldClose() {
		rl.BeginDrawing()

		rl.ClearBackground(rl.Blank)
		rl.DrawText("Congrats! You created your first window!", 190, 200, 20, rl.LightGray)

		if rl.IsKeyReleased(rl.KeyRight) {
			fmt.Println("right")
			spriteIdx = 0
			bmpIdx++
			unloadSprites(tmpSprites)
			tmpSprites = loadBitmapImg(bmpIdx)
			fmt.Println("bmpIdx =>", bmpIdx)

		} else if rl.IsKeyReleased(rl.KeyLeft) {
			fmt.Println("left")
			spriteIdx = 0
			bmpIdx--
			unloadSprites(tmpSprites)
			tmpSprites = loadBitmapImg(bmpIdx)
			fmt.Println("bmpIdx =>", bmpIdx)
		}

		rl.DrawTexture(*scrTexture, 0, 0, rl.White)

		xOffset := int32(0)
		for i := 0; i < len(tmpSprites); i++ {
			txt := tmpSprites[i]
			rl.DrawTexture(*txt, xOffset, 10, rl.White)
			xOffset += txt.Width
		}

		txt := tmpSprites[int(spriteIdx)]
		src := rl.NewRectangle(0, 0, float32(txt.Width), float32(txt.Height))

		const scaleFactor = 4
		dst := rl.NewRectangle(20, 100, float32(txt.Width)*scaleFactor, float32(txt.Height)*scaleFactor)
		rl.DrawTexturePro(*txt, src, dst, rl.Vector2{X: 0, Y: 0}, 0, rl.White)
		spriteIdx += 0.2
		if int(spriteIdx) > len(tmpSprites)-1 {
			spriteIdx = 0
		}

		// Works, window moves dynamically
		//winPos := rl.GetWindowPosition()
		//rl.SetWindowPosition(int(winPos.X)+int(spriteIdx/4), int(winPos.Y))

		rl.EndDrawing()
	}
}

func loadBitmapImg(idx int) []*rl.Texture2D {
	var tmpSprites []*rl.Texture2D

	bmpResource := bmpResources[idx]
	data := bmpResource.UncompressedData
	// dataOffset is where each bmp sprites data begins
	dataOffset := 0

	for img := 0; img < int(bmpResource.NumImages); img++ {
		if (bmpResource.Widths[img] % 2) == 1 {
			panic("grLoadBmp(): can't manage odd widths")
		}

		width := int(bmpResource.Widths[img])
		height := int(bmpResource.Heights[img])
		bytesPerRow := int(width) / 2

		pixelData := make([]byte, 4*width*height)

		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				byteIdx := y*bytesPerRow + (x / 2)

				// NOTE: This is a 4bit/per pixel color index
				var colorIdx int
				if x%2 == 0 {
					colorIdx = int((data[byteIdx] >> 4) & 0x0f)
				} else {
					colorIdx = int(data[byteIdx] & 0x0f)
				}
				clr := ttmPalette[colorIdx]

				c := color.RGBA{
					// Note color order -> this matches what's in the C implementation.
					R: clr[2],
					G: clr[1],
					B: clr[0],
					A: 0xff,
				}

				// When Pink Key Color!!!! Knock it out!
				// if RGB => 0xa8, 0x00, 0xa8 it's the key color and must not be rendered,
				// hence alpha is set to 0x00.
				if clr[0] == 0xa8 && clr[1] == 0x00 && clr[2] == 0xa8 {
					c = color.RGBA{
						R: 0x0,
						G: 0x0,
						B: 0x0,
						A: 0x0,
					}
				}

				idx := (y*width + x) * 4
				pixelData[idx] = c.R
				pixelData[idx+1] = c.G
				pixelData[idx+2] = c.B
				pixelData[idx+3] = c.A

				dataOffset = byteIdx
			}
		}
		data = data[dataOffset+1:]
		spriteImg := rl.NewImage(pixelData, int32(width), int32(height), 1, rl.UncompressedR8g8b8a8)
		spriteTexture := rl.LoadTextureFromImage(spriteImg)
		tmpSprites = append(tmpSprites, &spriteTexture)

	}
	return tmpSprites
}

func loadScrImg(screenName string) *rl.Texture2D {
	scrResource := findSCRResource(screenName)

	if (scrResource.Width % 2) == 1 {
		panic("Warning: grLoadScreen(): can't manage odd widths")
	}

	if scrResource.Width > 640 || scrResource.Height > 480 {
		panic("grLoadScreen(): can't manage more than 640x480 resolutions")
	}

	// NOTE: code below is working, even if not performant yet!

	width := int(scrResource.Width)
	height := int(scrResource.Height)
	bytesPerRow := int(width) / 2

	data := scrResource.UncompressedData
	pixelData := make([]byte, 4*width*height)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			byteIdx := y*bytesPerRow + (x / 2)

			// NOTE: This is a 4bit/per pixel color index
			var colorIdx int
			if x%2 == 0 {
				colorIdx = int((data[byteIdx] >> 4) & 0x0f)
			} else {
				colorIdx = int(data[byteIdx] & 0x0f)
			}
			clr := ttmPalette[colorIdx]
			c := color.RGBA{
				// Note color order -> this matches what's in the C implementation.
				R: clr[2],
				G: clr[1],
				B: clr[0],
				A: 0xff,
			}

			idx := (y*width + x) * 4
			pixelData[idx] = c.R
			pixelData[idx+1] = c.G
			pixelData[idx+2] = c.B
			pixelData[idx+3] = c.A
		}
	}

	spriteImg := rl.NewImage(pixelData, int32(width), int32(height), 1, rl.UncompressedR8g8b8a8)
	spriteTexture := rl.LoadTextureFromImage(spriteImg)
	return &spriteTexture
}
