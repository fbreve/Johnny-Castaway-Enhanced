package main

import "C"
import (
	"fmt"
	rl "github.com/gen2brain/raylib-go/raylib"
	"image/color"
	"os"
	"time"
)

const (
	ScreenWidth  = 640
	ScreenHeight = 480

	MaxBMPSlots      = 6
	MaxSpritesPerBMP = 120
	MaxTTMSlots      = 10
	MaxTTMThreads    = 10
)

var (
	ttmPalette = [16][4]uint8{}
)

var (
	grDx = 0
	grDy = 0
	//int grWindowed    = 0

	grUpdateDelay     int = 0
	grBackgroundSur   *rl.RenderTexture2D
	grSavedZonesLayer *rl.RenderTexture2D
)

type TAdsScene struct {
	slot     uint16
	tag      uint16
	numPlays uint16
}

type TTtmSlot struct {
	data       []byte
	dataSize   uint32
	tags       []TTtmTag
	numTags    int
	numSprites [MaxBMPSlots]int
	sprites    [MaxBMPSlots][MaxSpritesPerBMP]*rl.Texture2D
}

type TTtmTag struct { // TODO : rename, used for ADS too
	id     uint16
	offset uint32
}

type TTtmThread struct {
	ttmSlot         *TTtmSlot
	isRunning       int
	sceneSlot       uint16
	sceneTag        uint16
	sceneTimer      int16
	sceneIterations uint16
	ip              uint32
	delay           uint16
	timer           uint16
	nextGotoOffset  uint32
	selectedBmpSlot uint8
	fgColor         uint8
	bgColor         uint8
	ttmLayer        *rl.RenderTexture2D
}

func grReleaseScreen() {
	// Note: Deallocate is an ebiten specific thing, not sure if it's entirely necessary to invoke it.
	grBackgroundSur = nil
}

func grReleaseSavedLayer() {
	grSavedZonesLayer = nil
}

func grPutPixel(sur *rl.RenderTexture2D, x, y uint16, c uint8) {
	clr := color.RGBA{
		R: ttmPalette[c][0],
		G: ttmPalette[c][1],
		B: ttmPalette[c][2],
		A: 0,
	}

	rl.BeginTextureMode(*sur)
	defer rl.EndTextureMode()

	if x < 640 && y < 480 {
		rl.DrawPixel(int32(x), int32(y), clr)
	}
}

func grLoadPalette(palResource *TPALResource) {
	if palResource == nil {
		panic("nil palette")
	}

	for i := 0; i < 16; i++ {
		ttmPalette[i][0] = palResource.Colors[i].B << 2
		ttmPalette[i][1] = palResource.Colors[i].G << 2
		ttmPalette[i][2] = palResource.Colors[i].R << 2
		ttmPalette[i][3] = 0
	}
}

func graphicsInit() {
	// todo more stuff
	grLoadPalette(&palResources[0])
}

func graphicsEnd() {
	//SDL_DestroyWindow(sdl_window)
	//SDL_Quit()
}

func grToggleFullscreen() {

}

func grUpdateDisplay(
	ttmBGThread *TTtmThread,
	ttmThreads []TTtmThread,
	ttmHolidayThread *TTtmThread,
	ttmCloudsThread *TTtmThread,
) {
	// NOTE: it seems in the C code, ttmBackgroundThread is not used in this func. - r.c.
	// NOTE: Original has all args as *TTtmThread, but second arg is actually a multi-pointer, so I made it a slice. - r.c.
	// clear the screen.
	draw := func() {

		if rl.WindowShouldClose() {
			fmt.Println("exiting...")
			os.Exit(0)
		}

		rl.BeginDrawing()
		defer rl.EndDrawing()

		rl.ClearBackground(rl.Blank)

		// Scale and draw to actual window
		scale := min(
			float32(rl.GetScreenWidth())/float32(screenWidth),
			float32(rl.GetScreenHeight())/float32(screenHeight),
		)

		drawTexture := func(rt *rl.RenderTexture2D) {
			if rt == nil {
				return
			}

			w := float32(rt.Texture.Width)
			h := float32(rt.Texture.Height)
			// Note: This draws render textures back to right side up!
			src := rl.NewRectangle(0, 0, w, -h)
			dst := rl.NewRectangle(
				// Centers the game screens when aspect ratio doesn't match
				float32(rl.GetScreenWidth())/2-float32(screenWidth)*scale/2,
				float32(rl.GetScreenHeight())/2-float32(screenHeight)*scale/2,
				// Sets the scale of the screen for width and height
				w*scale,
				h*scale)
			rl.DrawTexturePro(rt.Texture, src, dst, rl.Vector2Zero(), 0, rl.White)
		}

		// Blit the background
		drawTexture(grBackgroundSur)

		// Blit the clouds
		if ttmCloudsThread != nil {
			if ttmCloudsThread.isRunning != 0 {
				drawTexture(ttmCloudsThread.ttmLayer)
			}
		}

		// Blit the saved zones layer
		drawTexture(grSavedZonesLayer)

		// Blit each threads layer
		for i := 0; i < MaxTTMThreads; i++ {
			if ttmThreads[i].isRunning != 0 {
				txt := ttmThreads[i].ttmLayer
				drawTexture(txt)
			}
		}

		// Finally, blit the holiday layer
		if ttmHolidayThread != nil {
			if ttmHolidayThread.isRunning != 0 {
				drawTexture(ttmHolidayThread.ttmLayer)
			}
		}
	}

	// TODO: Wait for the tick ...
	// r.c. (this is not like original C code which uses SDL, Raylib still requires calls to Begin/End draw
	// in addition to checking for WindowClose
	start := rl.GetTime()
	for {
		draw()
		const fps = 30
		const frameDelayMS = 1000 / fps
		time.Sleep(time.Millisecond * time.Duration(frameDelayMS))

		end := rl.GetTime()
		if grUpdateDelay == 0 ||
			(end-start <= float64(grUpdateDelay*20)) {
			break
		}
	}

	// Original C code is below.
	// eventsWaitTick(grUpdateDelay)

	// ... and refresh the display
	// SDL_UpdateWindowSurface(sdl_window)
}

func grNewLayer() *rl.RenderTexture2D {
	rt := rl.LoadRenderTexture(screenWidth, screenHeight)
	return &rt
}

func grFreeLayer(sur *rl.RenderTexture2D) {
	rl.UnloadRenderTexture(*sur)
}

func grSetClipZone(sur *rl.RenderTexture2D, x1, y1, x2, y2 int16) {
	x1 += int16(grDx)
	y1 += int16(grDy)
	x2 += int16(grDx)
	y2 += int16(grDy)

	// SDL2 code
	//SDL_Rect rect = { x1, y1, x2-x1, y2-y1 };
	//SDL_SetClipRect(sur, &rect);

	// Equivalent Ebiten code?? Not sure, I need to prove this out.
	// NOTE: according to docs, SubImage returns the image.Image interface but it's always
	// a *ebiten.Image so I should be able to cast and save it.
	//rect := image.Rect(int(x1), int(y1), int(x2), int(y2))
	//grClippedImage = sur.SubImage(rect).(*ebiten.Image)
}

func grCopyZoneToBg(sur *rl.RenderTexture2D, x, y, width, height uint16) {
	x += uint16(grDx)
	y += uint16(grDy)

	//SDL_Rect rect = { (short) x, (short) y, width + 2, height };
	//adjustedWidth := width + 2
	//
	//if grSavedZonesLayer == nil {
	//	grSavedZonesLayer = grNewLayer()
	//}

	// r.c. NOTE: this block is just to document SDL2 which is the source vs dst surface.
	// int SDL_BlitSurface(SDL_Surface *src,
	//                    const SDL_Rect *srcrect,
	//                    SDL_Surface *dst,
	//                    SDL_Rect *dstrect);

	// original SDL code
	//SDL_BlitSurface(sfc, &rect, grSavedZonesLayer, &rect);

	// Note : without the +2 in width+2 above, there would be a graphical
	// glitch (2 unfilled pixels) on the hull of the cargo, caused by an
	// error in coordinates in GJIVS6.TTM
	// Obviously, the original soft rounds the SAVE_IMAGE boundaries on
	// one way or another.

	// ported Ebiten code
	// Define source rectangle
	//srcRect := image.Rect(int(int16(x)), int(int16(y)), int(x+adjustedWidth), int(y+height))

	// Extract the sub-image from source
	//subImg := sur.SubImage(srcRect).(*ebiten.Image)
	//
	//// Set up draw options to position at the same coordinates in destination
	//opts := &ebiten.DrawImageOptions{}
	//opts.GeoM.Translate(float64(x), float64(y))
	//
	//// Draw to saved zones layer
	//grSavedZonesLayer.DrawImage(subImg, opts)
}

func grRestoreZone(sur *rl.RenderTexture2D, x, y, width, height uint16) {
	// In Johnny's TTMs, we never have RESTORE_ZONE called
	// while several zones are saved. So we simply free the
	// whole saved zones layer
	grReleaseSavedLayer()
}

func grDrawPixel(sur *rl.RenderTexture2D, x, y int16, clr uint8) {
	x += int16(grDx)
	y += int16(grDy)
	grPutPixel(sur, uint16(x), uint16(y), clr)
}

func grDrawLine(sur *rl.RenderTexture2D, x1, y1, x2, y2 int16, colorIdx uint8) {
	x1 += int16(grDx)
	y1 += int16(grDy)
	x2 += int16(grDx)
	y2 += int16(grDy)

	clr := ttmPalette[colorIdx]
	c := color.RGBA{
		// Note color order -> this matches what's in the C implementation.
		R: clr[2],
		G: clr[1],
		B: clr[0],
		A: 0xff,
	}

	rl.BeginTextureMode(*sur)
	defer rl.EndTextureMode()

	rl.DrawLine(int32(x1), int32(y1), int32(x2), int32(y2), c)
}

func grDrawHorizontalLine(sur *rl.RenderTexture2D, x1, x2, y int16, color uint8) {
	if y < 0 || y > 479 {
		return
	}

	if x1 < 0 {
		x1 = 0
	}
	if x2 > 639 {
		x2 = 639
	}

	for x := x1; x < x2; x++ {
		grPutPixel(sur, uint16(x), uint16(y), color)
	}
}

func grDrawRect(sur *rl.RenderTexture2D, x, y int16, width, height uint16, colorIdx uint8) {
	x += int16(grDx)
	y += int16(grDy)

	// r.c. testing this out, not ready yet.

	clr := ttmPalette[colorIdx]
	c := color.RGBA{
		// Note color order -> this matches what's in the C implementation.
		R: clr[2],
		G: clr[1],
		B: clr[0],
		A: 0xff,
	}

	rl.BeginTextureMode(*sur)
	defer rl.EndTextureMode()

	rl.DrawRectangle(int32(x), int32(y), int32(width), int32(height), c)
}

func grDrawCircle(sur *rl.RenderTexture2D, x1, y1 int16, width, height uint16, fgColor, bgColor uint8) {
	x1 += int16(grDx)
	y1 += int16(grDy)

	// We can only draw regular circles
	if width != height {
		fmt.Println("Warning : grDrawCircle() : unable to draw ellipse")
		return
	}

	// In original data, every width is even
	if width%2 != 0 {
		fmt.Println("Warning : grDrawCircle() : unable to process odd diameters")
		return
	}

	// Note: Original uses fully manual pixel drawing, we will just chat with Raylib's circle drawing facilities
	// Bresenham's circle drawing algorithm
	// Note : the code below intends to be pixel-perfect
	rl.BeginTextureMode(*sur)
	defer rl.EndTextureMode()

	// Note, currently only using fgColor and bgColor is ignored!
	colorIdx := fgColor
	clr := ttmPalette[colorIdx]

	c := color.RGBA{
		// Note color order -> this matches what's in the C implementation.
		R: clr[2],
		G: clr[1],
		B: clr[0],
		A: 0xff,
	}

	rl.DrawCircle(int32(x1), int32(y1), float32(width), c)
}

func grDrawSprite(sur *rl.RenderTexture2D, ttmSlot *TTtmSlot, x, y int16, spriteNo, imageNo uint16) {
	if int(spriteNo) >= ttmSlot.numSprites[imageNo] {
		fmt.Printf("Warning : grDrawSprite(): less than %d sprites loaded in slot %d\n", imageNo, spriteNo)
		return
	}

	x += int16(grDx)
	y += int16(grDy)

	srcSurface := ttmSlot.sprites[imageNo][spriteNo]

	rl.BeginTextureMode(*sur)
	defer rl.EndTextureMode()

	// NOTE: this clears the layer, and only the instruction-set should clear it when it deems necessary.
	//rl.ClearBackground(rl.Blank)

	// Use rl.Red for troubleshooting to render Red colored flipped sprites.
	xx := float32(x)
	yy := float32(y)
	w := float32(srcSurface.Width)
	h := float32(srcSurface.Height)

	// debugging bounding box.
	if rl.IsKeyDown(rl.KeyLeftShift) {
		rl.DrawRectangleLines(int32(xx), int32(yy), int32(w), int32(h), rl.Red)
	}

	src := rl.NewRectangle(0, 0, w, h)
	dst := rl.NewRectangle(xx, yy, w, h)
	rl.DrawTexturePro(*srcSurface, src, dst, rl.Vector2Zero(), 0.0, rl.White)
}

func grDrawSpriteFlip(sur *rl.RenderTexture2D, ttmSlot *TTtmSlot, x, y int16, spriteNo, imageNo uint16) {
	if int(spriteNo) >= ttmSlot.numSprites[imageNo] {
		fmt.Printf("Warning : grDrawSprite(): less than %d sprites loaded in slot %d\n", imageNo, spriteNo)
		return
	}

	x += int16(grDx)
	y += int16(grDy)

	srcSurface := ttmSlot.sprites[imageNo][spriteNo]
	//x += int16(srcSurface.Width) - 1 // In original C, but NOT NEEDED, in Raylib.

	rl.BeginTextureMode(*sur)
	defer rl.EndTextureMode()

	// NOTE: this clears the layer, and only the instruction-set should clear it when it deems necessary.
	//rl.ClearBackground(rl.Blank)

	// Use rl.Red for troubleshooting to render Red colored flipped sprites.
	xx := float32(x)
	yy := float32(y)
	w := float32(srcSurface.Width)
	h := float32(srcSurface.Height)

	// For debugging purposes.
	if rl.IsKeyDown(rl.KeyLeftShift) {
		rl.DrawRectangleLines(int32(xx), int32(yy), int32(w), int32(h), rl.Red)
	}

	src := rl.NewRectangle(0, 0, -w, h)
	dst := rl.NewRectangle(xx, yy, w, h)
	rl.DrawTexturePro(*srcSurface, src, dst, rl.Vector2Zero(), 0.0, rl.White) //rl.Red)
}

func grClearScreen(sur *rl.RenderTexture2D) {
	// NOTE: original game colors the key color, but when it renders does it show up? I doubt it.
	//keyKnockoutColor := color.RGBA{R: 0xa8, G: 0x00, B: 0xa8, A: 0xff}
	//keyKnockoutColor := color.RGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x00}
	rl.BeginTextureMode(*sur)
	defer rl.EndTextureMode()

	rl.ClearBackground(rl.Blank)
}

func grLoadScreen(screenName string) {
	if grBackgroundSur != nil {
		grReleaseScreen()
	}

	if grSavedZonesLayer != nil {
		grReleaseSavedLayer()
	}

	scrResource := findSCRResource(screenName)

	if (scrResource.Width % 2) == 1 {
		panic("Warning: grLoadScreen(): can't manage odd widths")
	}

	if scrResource.Width > 640 || scrResource.Height > 480 {
		panic("grLoadScreen(): can't manage more than 640x480 resolutions")
	}

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

	rt := rl.LoadRenderTexture(int32(width), int32(height))
	grBackgroundSur = &rt

	rl.BeginTextureMode(rt)
	defer rl.EndTextureMode()

	// Draw flipped (-height)
	src := rl.NewRectangle(
		0,
		0,
		float32(spriteTexture.Width),
		float32(spriteTexture.Height), //float32(-spriteTexture.Height), // Always negative
	)
	dst := rl.NewRectangle(0, 0, float32(spriteTexture.Width), float32(spriteTexture.Height))
	rl.DrawTexturePro(spriteTexture, src, dst, rl.Vector2Zero(), 0.0, rl.White)
}

func grInitEmptyBackground() {
	if grBackgroundSur != nil {
		grReleaseScreen()
	}

	if grSavedZonesLayer != nil {
		grReleaseSavedLayer()
	}

	rt := rl.LoadRenderTexture(640, 480)
	grBackgroundSur = &rt

	rl.BeginTextureMode(*grBackgroundSur)
	rl.ClearBackground(rl.Black)
	rl.EndTextureMode()
}

func grLoadBmp(ttmSlot *TTtmSlot, slotNo uint16, name string) {
	if ttmSlot.numSprites[slotNo] != 0 {
		grReleaseBmp(ttmSlot, slotNo)
	}

	bmpResource := findBMPResource(name)
	ttmSlot.numSprites[slotNo] = int(bmpResource.NumImages)
	data := bmpResource.UncompressedData
	dataOffset := 0 // dataOffset is where each bmp sprites data begins

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
		// segments the data to be the next cel of the sprite.
		data = data[dataOffset+1:]
		spriteImg := rl.NewImage(pixelData, int32(width), int32(height), 1, rl.UncompressedR8g8b8a8)
		spriteTexture := rl.LoadTextureFromImage(spriteImg)
		ttmSlot.sprites[slotNo][img] = &spriteTexture
	}
}

func grReleaseBmp(ttmSlot *TTtmSlot, bmpSlotNo uint16) {
	for i := 0; i < ttmSlot.numSprites[bmpSlotNo]; i++ {
		spr := ttmSlot.sprites[bmpSlotNo][i]
		rl.UnloadTexture(*spr)
	}

	ttmSlot.numSprites[bmpSlotNo] = 0
}

func grFadeOut() {
	// Does screen transitions like iris, rect sliding
	// Don't necessarily need this day 1
	// May be able to fake it with just simple assets
}
