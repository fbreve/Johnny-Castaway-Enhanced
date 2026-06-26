package main

import "C"
import (
	"fmt"
	"image/color"
	"math"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	MaxBMPSlots      = 6
	MaxSpritesPerBMP = 120
	MaxTTMSlots      = 10
	MaxTTMThreads    = 10
)

const (
	MaxFadeOutRadius = 800
)

var (
	// added by r.c. to mimic screen saver behavior.
	screenSaverPos           rl.Vector2 = rl.Vector2Zero()
	isScreensaverMode                   = false
	frameCount                          = 0
	isScreenSaverPosCaptured            = false
	shouldExitApp                       = false
)

var (
	ttmPalette = [16][4]uint8{}

	grDx = 0
	grDy = 0
	//int grWindowed    = 0

	isFadingOut   = false
	fadeOutRadius = 0
	isFadingIn    = false
	fadeInRadius  = 0

	grUpdateDelay     int = 0
	grBackgroundSur   *rl.RenderTexture2D
	grSavedZonesLayer *rl.RenderTexture2D
	grFinalRenderSur  *rl.RenderTexture2D
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

	// Mouse position is captured after a few frames in grUpdateDisplay to avoid startup fluctuations
	screenSaverPos = rl.Vector2Zero()

	rt := rl.LoadRenderTexture(640, 480)
	grFinalRenderSur = &rt
}

func graphicsEnd() {
	if grFinalRenderSur != nil {
		rl.UnloadRenderTexture(*grFinalRenderSur)
		grFinalRenderSur = nil
	}
}

func grToggleFullscreen() {

}

func grUpdateDisplay(
	ttmBGThread *TTtmThread,
	ttmThreads []TTtmThread,
	ttmHolidayThread *TTtmThread,
	ttmCloudsThread *TTtmThread,
) {
	// r.c. - compute one letterboxed (4:3) destination rect per connected
	// monitor instead of one for the whole (now possibly multi-monitor-
	// spanning) window, so the same scene is drawn correctly centered on
	// each monitor individually rather than once across all of them.
	// Falls back to a single full-window rect if monitorRects hasn't been
	// populated (other window paths, like the settings/asset-browser
	// windows, don't call setupMonitors()).
	type monitorDrawRect struct {
		offsetX, offsetY, renderW, renderH float32
	}

	targetAspect := float32(4.0) / 3.0

	computeLetterbox := func(w, h float32) (rw, rh, ox, oy float32) {
		aspect := w / h
		if aspect > targetAspect {
			rh = h
			rw = rh * targetAspect
			ox = (w - rw) / 2.0
			oy = 0
		} else {
			rw = w
			rh = rw / targetAspect
			ox = 0
			oy = (h - rh) / 2.0
		}
		return
	}

	var monitorDrawRects []monitorDrawRect
	if len(monitorRects) > 0 {
		for _, m := range monitorRects {
			rw, rh, ox, oy := computeLetterbox(m.W, m.H)
			monitorDrawRects = append(monitorDrawRects, monitorDrawRect{
				offsetX: m.X + ox,
				offsetY: m.Y + oy,
				renderW: rw,
				renderH: rh,
			})
		}
	} else {
		screenWidthFloat := float32(rl.GetScreenWidth())
		screenHeightFloat := float32(rl.GetScreenHeight())
		rw, rh, ox, oy := computeLetterbox(screenWidthFloat, screenHeightFloat)
		monitorDrawRects = append(monitorDrawRects, monitorDrawRect{
			offsetX: ox,
			offsetY: oy,
			renderW: rw,
			renderH: rh,
		})
	}

	draw := func() {
		if rl.IsKeyReleased(rl.KeyLeftShift) {
			debugEnabled = !debugEnabled
		}

		if rl.WindowShouldClose() || shouldExitApp {
			shouldExitApp = true
			fmt.Println("exiting...")
			return
		}

		// r.c. - while the window is minimized, skip the render work below.
		// Presentation is already stopped by being minimized; this just
		// avoids wasted per-frame work in that state.
		if rl.IsWindowMinimized() {
			return
		}

		type OrientationMode int
		const (
			ModeNormal  OrientationMode = 0
			ModeFlipped OrientationMode = 1
		)

		if !isFadingOut && grFinalRenderSur != nil {
			rl.BeginTextureMode(*grFinalRenderSur)
			rl.ClearBackground(rl.Blank)

			drawTextureToFinal := func(rt *rl.RenderTexture2D, orientation OrientationMode) {
				if rt == nil {
					return
				}

				w := float32(rt.Texture.Width)
				h := float32(rt.Texture.Height)

				if orientation == ModeFlipped {
					h = -h
				}

				src := rl.NewRectangle(0, 0, w, h)
				dst := rl.NewRectangle(0, 0, float32(rt.Texture.Width), float32(rt.Texture.Height))
				rl.DrawTexturePro(rt.Texture, src, dst, rl.Vector2Zero(), 0, rl.White)
			}

			// Blit the background
			drawTextureToFinal(grBackgroundSur, ModeFlipped)

			// Blit the clouds
			if ttmCloudsThread != nil {
				if ttmCloudsThread.isRunning != 0 {
					drawTextureToFinal(ttmCloudsThread.ttmLayer, ModeFlipped)
				}
			}

			// Blit the saved zones layer
			drawTextureToFinal(grSavedZonesLayer, ModeFlipped)

			// Blit each threads layer
			for i := 0; i < MaxTTMThreads; i++ {
				if ttmThreads[i].isRunning != 0 {
					txt := ttmThreads[i].ttmLayer
					drawTextureToFinal(txt, ModeFlipped)
				}
			}

			// Finally, blit the holiday layer
			if ttmHolidayThread != nil {
				if ttmHolidayThread.isRunning != 0 {
					drawTextureToFinal(ttmHolidayThread.ttmLayer, ModeFlipped)
				}
			}
			rl.EndTextureMode()
		}

		rl.BeginDrawing()
		defer rl.EndDrawing()

		rl.ClearBackground(rl.Black)

		drawTextureToScreen := func(rt *rl.RenderTexture2D, orientation OrientationMode, destX, destY, destW, destH float32) {
			if rt == nil {
				return
			}

			w := float32(rt.Texture.Width)
			h := float32(rt.Texture.Height)

			if orientation == ModeFlipped {
				h = -h
			}

			src := rl.NewRectangle(0, 0, w, h)
			dst := rl.NewRectangle(destX, destY, destW, destH)
			rl.DrawTexturePro(rt.Texture, src, dst, rl.Vector2Zero(), 0, rl.White)
		}

		if grFinalRenderSur != nil {
			for _, r := range monitorDrawRects {
				drawTextureToScreen(grFinalRenderSur, ModeFlipped, r.offsetX, r.offsetY, r.renderW, r.renderH)
			}
		}

		if isFadingOut {
			for _, r := range monitorDrawRects {
				drawCircularIris(fadeOutRadius, r.offsetX, r.offsetY, r.renderW, r.renderH)
			}
		} else if isFadingIn {
			for _, r := range monitorDrawRects {
				drawCircularIris(fadeInRadius, r.offsetX, r.offsetY, r.renderW, r.renderH)
			}
		}

		// Debug stuff
		if debugEnabled {
			fontSize := int32(35)
			yPos := int32(rl.GetScreenHeight()) - (fontSize * 2)
			offset := int32(3)
			rl.DrawText(fmt.Sprintf("Story: %d", storyCurrentDay), fontSize, yPos, fontSize, rl.Black)
			rl.DrawText(fmt.Sprintf("Story: %d", storyCurrentDay), fontSize-offset, yPos-offset, fontSize, rl.White)

			rl.DrawFPS(10, 10)
		}

		// If screensaver mode is enabled, exit on mouse movement (after settling) or key/mouse press.
		if isScreensaverMode {
			// Check for keyboard or mouse clicks
			if rl.GetKeyPressed() != 0 || rl.IsMouseButtonPressed(rl.MouseLeftButton) || rl.IsMouseButtonPressed(rl.MouseRightButton) {
				rl.SetMasterVolume(0)
				shouldExitApp = true
				return
			}

			frameCount++
			if frameCount > 10 { // Wait 10 frames for mouse events/focus to settle
				mousePos := rl.GetMousePosition()
				if !isScreenSaverPosCaptured {
					screenSaverPos = mousePos
					isScreenSaverPosCaptured = true
				} else {
					dx := mousePos.X - screenSaverPos.X
					dy := mousePos.Y - screenSaverPos.Y
					if (dx*dx + dy*dy) > 100 { // 10 pixels threshold squared
						rl.SetMasterVolume(0)
						shouldExitApp = true
						return
					}
				}
			}
		}
	}

	start := rl.GetTime()
	for {
		draw()
		if shouldExitApp {
			break
		}
		if isFadingOut {
			break
		}
		const fps = 30
		const frameDelayMS = 1000 / fps
		time.Sleep(time.Millisecond * time.Duration(frameDelayMS))

		if isFadingIn {
			fadeInRadius += 25
			if fadeInRadius >= 800 {
				fadeInRadius = 800
				isFadingIn = false
			}
		}

		end := rl.GetTime()
		if isFadingOut || grUpdateDelay == 0 ||
			(end-start) >= (float64(grUpdateDelay)*0.02) {
			break
		}
	}
}

func grNewLayer() *rl.RenderTexture2D {
	rt := rl.LoadRenderTexture(screenWidth, screenHeight)
	rl.BeginTextureMode(rt)
	rl.ClearBackground(rl.Blank)
	rl.EndTextureMode()
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

	// Equivalent Raylib code?? Not sure, I need to prove this out.

	//rect := image.Rect(int(x1), int(y1), int(x2), int(y2))
	//grClippedImage = sur.SubImage(rect).(*ebiten.Image)
}

func grCopyZoneToBg(sur *rl.RenderTexture2D, x, y, width, height uint16) {
	x += uint16(grDx)
	y += uint16(grDy)

	// Invert Y for the source rectangle since RenderTexture is flipped vertically in memory.
	srcRect := rl.NewRectangle(float32(x), float32(screenHeight-int(y)), float32(width+2), -float32(height))
	dstRect := rl.NewRectangle(float32(x), float32(y), float32(width+2), float32(height))

	if grSavedZonesLayer == nil {
		grSavedZonesLayer = grNewLayer()
	}

	rl.BeginTextureMode(*grSavedZonesLayer)
	defer rl.EndTextureMode()

	rl.DrawTexturePro(sur.Texture, srcRect, dstRect, rl.Vector2Zero(), 0.0, rl.White)

	// BELOW IS ORIGINAL C Code

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
}

func grSaveImage1(sur *rl.RenderTexture2D, arg0, arg1, arg2, arg3 uint16) { // // TODO : rename ?
	// r.c. in the original C code, these are NOT implemented!

	//    ttmSetColors(4,4);
	//    ttmDrawRect(arg0,arg1,arg2,arg3);
	//    ttmSaveImage0(arg0,arg1,arg2,arg3);
	//    ttmUpdate();
}

func grSaveZone(sur *rl.RenderTexture2D, x, y, width, height uint16) {
	// r.c. in the original C code, these are NOT implemented!

	// Minimalistic implementation: we don't really save the zone,
	// and let grRestoreZone() simply erase the 'saved zones' layer
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

	clr := ttmPalette[colorIdx&0x0f]
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

	clr := ttmPalette[colorIdx&0x0f]
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

	rl.BeginTextureMode(*sur)
	defer rl.EndTextureMode()

	grabColor := func(idx uint8) color.RGBA {
		clr := ttmPalette[idx&0x0f]
		return color.RGBA{
			// Note color order -> this matches what's in the C implementation.
			R: clr[2],
			G: clr[1],
			B: clr[0],
			A: 0xff,
		}
	}

	radius := float32(width) / 2.0
	centerX := int32(float32(x1) + radius)
	centerY := int32(float32(y1) + radius)

	// Draw filled circle with background/fill color
	bgClr := grabColor(bgColor)
	rl.DrawCircle(centerX, centerY, radius, bgClr)

	// Draw border circle with foreground color
	if fgColor != bgColor {
		fgClr := grabColor(fgColor)
		rl.DrawCircleLines(centerX, centerY, radius, fgClr)
	}
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
	//if debugEnabled {
	//	rl.DrawRectangleLines(int32(xx), int32(yy), int32(w), int32(h), rl.Red)
	//}

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
	//if debugEnabled {
	//	rl.DrawRectangleLines(int32(xx), int32(yy), int32(w), int32(h), rl.Red)
	//}

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

	rl.DrawTexture(spriteTexture, 0, 0, rl.White)
}

func grInitEmptyBackground() {
	if grBackgroundSur != nil {
		grReleaseScreen()
	}

	if grSavedZonesLayer != nil {
		grReleaseSavedLayer()
	}

	rt := rl.LoadRenderTexture(screenWidth, screenHeight)
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
	isFadingOut = true
	fadeOutRadius = 800

	for isFadingOut && !shouldExitApp {
		grUpdateDisplay(&ttmBackgroundThread, ttmThreads[:], &ttmHolidayThread, &ttmCloudsThread)

		fadeOutRadius -= 25
		if fadeOutRadius <= 0 {
			fadeOutRadius = 0
			isFadingOut = false
		}

		time.Sleep(time.Millisecond * 33)
	}
}

func grFadeIn() {
	isFadingIn = true
	fadeInRadius = 0
}

func drawCircularIris(radiusVal int, regionX, regionY, regionW, regionH float32) {
	cx := regionX + regionW/2.0
	cy := regionY + regionH/2.0

	targetAspect := float32(4.0) / 3.0
	currentAspect := regionW / regionH

	var renderH float32
	if currentAspect > targetAspect {
		renderH = regionH
	} else {
		renderH = regionW / targetAspect
	}

	actualRadius := float32(radiusVal) * (renderH / 480.0)

	left := int32(regionX)
	top := int32(regionY)
	right := int32(regionX + regionW)
	bottom := int32(regionY + regionH)

	if actualRadius <= 0 {
		rl.DrawRectangle(left, top, right-left, bottom-top, rl.Black)
		return
	}

	rInt := int32(actualRadius)
	cxInt := int32(cx)
	cyInt := int32(cy)

	// Top area (above circle vertical span), bounded to this monitor's region
	topHeight := cyInt - rInt
	if topHeight > top {
		rl.DrawRectangle(left, top, right-left, topHeight-top, rl.Black)
	}

	// Bottom area (below circle vertical span), bounded to this monitor's region
	bottomStart := cyInt + rInt
	if bottomStart < bottom {
		rl.DrawRectangle(left, bottomStart, right-left, bottom-bottomStart, rl.Black)
	}

	// Rows intersecting the circle, bounded to this monitor's region
	r2 := actualRadius * actualRadius
	startY := cyInt - rInt
	if startY < top {
		startY = top
	}
	endY := cyInt + rInt
	if endY > bottom {
		endY = bottom
	}

	for y := startY; y < endY; y++ {
		dy := float32(y) - cy
		val := r2 - dy*dy
		if val < 0 {
			val = 0
		}
		dx := float32(math.Sqrt(float64(val)))

		xStart := cxInt - int32(dx)
		xEnd := cxInt + int32(dx)

		if xStart > left {
			rl.DrawRectangle(left, y, xStart-left, 1, rl.Black)
		}
		if xEnd < right {
			rl.DrawRectangle(xEnd, y, right-xEnd, 1, rl.Black)
		}
	}
}
