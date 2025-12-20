package main

import (
	"fmt"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"image"
	"image/color"
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
	grUpdateDelay   int = 0
	grBackgroundSur *ebiten.Image

	grSavedZonesLayer *ebiten.Image
	// Note: original C code doesn't have this field, but I think this needed to be added
	// in order to make the Ebiten code equivalent - r.c.
	grClippedImage *ebiten.Image
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
	sprites    [MaxBMPSlots][MaxSpritesPerBMP]*ebiten.Image
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
	ttmLayer        *ebiten.Image
}

func grReleaseScreen() {
	// Note: Deallocate is an ebiten specific thing, not sure if it's entirely necessary to invoke it.
	grBackgroundSur.Deallocate()
	grBackgroundSur = nil
}

func grReleaseSavedLayer() {
	grSavedZonesLayer.Deallocate()
	grSavedZonesLayer = nil
}

func grPutPixel(sur *ebiten.Image, x, y uint16, c uint8) {
	// TODO: Implement Cohen-Sutherland clipping algorithm or such for
	// grDrawLine(), and another ad hoc algorithm for grDrawCircle()

	if x < 640 && y < 480 {
		//pixelIdx := int(y) + (int(x) * sur.Rect.Dx())
		//sur.Pix[4*pixelIdx] = ttmPalette[color][0]
		//sur.Pix[4*pixelIdx+1] = ttmPalette[color][1]
		//sur.Pix[4*pixelIdx+2] = ttmPalette[color][2]
		//sur.Pix[4*pixelIdx+3] = 0

		// first try naive way, using the built-in Set method.
		clr := color.RGBA{
			R: ttmPalette[c][0],
			G: ttmPalette[c][1],
			B: ttmPalette[c][2],
			A: 0,
		}
		sur.Set(int(x), int(y), clr)
		//uint8 *pixel = (uint8*) sfc->pixels;
		//
		//pixel += (y * sfc->pitch) + (x * sfc->format->BytesPerPixel);
		//
		//pixel[0] = ttmPalette[color][0];
		//pixel[1] = ttmPalette[color][1];
		//pixel[2] = ttmPalette[color][2];
		//pixel[3] = 0;
	}
}

func grDrawHorizontalLine(sur *ebiten.Image, x1, x2, y int16, color uint8) {
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

func grUpdateDisplay(ttmBGThread *TTtmThread, ttmThreads []TTtmThread, ttmHolidayThread *TTtmThread) {
	// NOTE: it seems in the C code, ttmBackgroundThread is not used in this func. - r.c.
	// NOTE: Original has all args as *TTtmThread, but second arg is actually a multi-pointer, so I made it a slice. - r.c.
	// clear the screen.

	// Blit the background
	if grBackgroundSur != nil {
		gScreen.DrawImage(grBackgroundSur, &ebiten.DrawImageOptions{})
	}

	if grSavedZonesLayer != nil {
		gScreen.DrawImage(grSavedZonesLayer, &ebiten.DrawImageOptions{})
	}

	// Blit each threads layer
	for i := 0; i < MaxTTMThreads; i++ {
		if ttmThreads[i].isRunning != 0 {
			gScreen.DrawImage(ttmThreads[i].ttmLayer, &ebiten.DrawImageOptions{})
		}
	}

	// TODO: Finally, blit the holiday layer
	if ttmHolidayThread != nil {
		if ttmHolidayThread.isRunning != 0 {
			gScreen.DrawImage(ttmHolidayThread.ttmLayer, &ebiten.DrawImageOptions{})
		}
	}

	// TODO: Wait for the tick ...
	// eventsWaitTick(grUpdateDelay)

	// ... and refresh the display
	// SDL_UpdateWindowSurface(sdl_window)
}

func grNewLayer() *ebiten.Image {
	img := ebiten.NewImage(screenWidth, screenHeight)
	return img
}

func grFreeLayer(sur *ebiten.Image) {
	// r.c. in ebiten, I think Deallocate is just helper for tighter memory control.
	// but not truly necessary.
	sur.Deallocate()
}

func grSetClipZone(sur *ebiten.Image, x1, y1, x2, y2 int16) {
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
	rect := image.Rect(int(x1), int(y1), int(x2), int(y2))
	grClippedImage = sur.SubImage(rect).(*ebiten.Image)
}

func grCopyZoneToBg(sur *ebiten.Image, x, y, width, height uint16) {
	x += uint16(grDx)
	y += uint16(grDy)

	//SDL_Rect rect = { (short) x, (short) y, width + 2, height };
	adjustedWidth := width + 2

	if grSavedZonesLayer == nil {
		grSavedZonesLayer = grNewLayer()
	}

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
	srcRect := image.Rect(int(int16(x)), int(int16(y)), int(x+adjustedWidth), int(y+height))

	// Extract the sub-image from source
	subImg := sur.SubImage(srcRect).(*ebiten.Image)

	// Set up draw options to position at the same coordinates in destination
	opts := &ebiten.DrawImageOptions{}
	opts.GeoM.Translate(float64(x), float64(y))

	// Draw to saved zones layer
	grSavedZonesLayer.DrawImage(subImg, opts)
}

// zone stuff

func grDrawPixel(sur *ebiten.Image, x, y int16, clr uint8) {
	x += int16(grDx)
	y += int16(grDy)
	grPutPixel(sur, uint16(x), uint16(y), clr)
}

func grDrawLine() {
	fmt.Println("grDrawLine(...)")
}

func grDrawRect(sur *ebiten.Image, x, y int16, width, height uint16, colorIdx uint8) {
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

	vector.FillRect(
		sur,
		float32(x),
		float32(y),
		float32(width),
		float32(height),
		c,
		false,
	)
}

func grDrawCircle() {
	//x1 += int16(grDx)
	//y1 += int16(grDy)
	//
	//// We can only draw regular circles
	//if width != height {
	//	fmt.Println("Warning : grDrawCircle() : unable to draw ellipse")
	//	return
	//}
	//
	//// In original data, every width is even
	//if width%2 != 0 {
	//	fmt.Println("Warning : grDrawCircle() : unable to process odd diameters")
	//	return
	//}
	//
	//// Bresenham's circle drawing algorithm
	//// Note : the code below intends to be pixel-perfect
	//
	//r := (width >> 1) - 1
	//xc := uint16(x1) + r
	//yc := uint16(y1) + r
	//x := int16(0)
	//y := int16(r)
	//var d int = 1 - int(r)
	//
	//for {
	//
	//	grDrawHorizontalLine(sur, xc-x, xc+x+1, yc+y+1, bgColor)
	//	grDrawHorizontalLine(sur, xc-x, xc+x+1, yc-y, bgColor)
	//
	//	grDrawHorizontalLine(sur, xc-y, xc+y+1, yc+x+1, bgColor)
	//	grDrawHorizontalLine(sur, xc-y, xc+y+1, yc-x, bgColor)
	//
	//	if y-x <= 1 {
	//		break
	//	}
	//
	//	if d < 0 {
	//		d += (x << 1) + 3
	//	} else {
	//		d += ((x - y) << 1) + 5
	//		y--
	//	}
	//	x++
	//}
}

func grDrawSprite(sur *ebiten.Image, ttmSlot *TTtmSlot, x, y int16, spriteNo, imageNo uint16) {
	if int(spriteNo) >= ttmSlot.numSprites[imageNo] {
		fmt.Printf("Warning : grDrawSprite(): less than %d sprites loaded in slot %d\n", imageNo, spriteNo)
		return
	}

	x += int16(grDx)
	y += int16(grDy)

	srcSurface := ttmSlot.sprites[imageNo][spriteNo]

	// NOTE: I think I have the source and dest surfaces correct!

	opts := &ebiten.DrawImageOptions{}
	opts.GeoM.Translate(float64(x), float64(y))
	sur.DrawImage(srcSurface, opts)
}

func grDrawSpriteFlip(sur *ebiten.Image, ttmSlot *TTtmSlot, x, y int16, spriteNo, imageNo uint16) {
	if int(spriteNo) >= ttmSlot.numSprites[imageNo] {
		fmt.Printf("Warning : grDrawSprite(): less than %d sprites loaded in slot %d\n", imageNo, spriteNo)
		return
	}

	x += int16(grDx)
	y += int16(grDy)

	srcSurface := ttmSlot.sprites[imageNo][spriteNo]
	x += int16(srcSurface.Bounds().Dx()) - 1

	opts := &ebiten.DrawImageOptions{}
	// Color scale of red, shows those frames which are playing "flipped" - for troubleshooting.
	//opts.ColorScale.Scale(1.0, 0, 0, 1.0)
	opts.GeoM.Scale(-1, 1)
	opts.GeoM.Translate(float64(x), float64(y))
	sur.DrawImage(srcSurface, opts)
}

func grClearScreen(sur *ebiten.Image) {
	// NOTE: original game colors the key color, but when it renders does it show up? I doubt it.
	//keyKnockoutColor := color.RGBA{R: 0xa8, G: 0x00, B: 0xa8, A: 0xff}
	keyKnockoutColor := color.RGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x00}
	sur.Fill(keyKnockoutColor)
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

	// NOTE: code below is working, even if not performant yet!

	width := scrResource.Width
	height := scrResource.Height
	bytesPerRow := int(width) / 2

	grBackgroundSur = ebiten.NewImage(int(width), int(height))
	data := scrResource.UncompressedData

	for y := 0; y < int(height); y++ {
		for x := 0; x < int(width); x++ {
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
			grBackgroundSur.Set(x, y, c)
		}
	}
}

func grInitEmptyBackground() {
	if grBackgroundSur != nil {
		grReleaseScreen()
	}

	if grSavedZonesLayer != nil {
		grReleaseSavedLayer()
	}

	grBackgroundSur = ebiten.NewImage(640, 480)
	grBackgroundSur.Fill(color.Black)
}

// used for temporary visualizing and testing of sprite data.
var tmpSprites []*ebiten.Image

func grLoadBmp(ttmSlot *TTtmSlot, slotNo uint16, name string) {
	if ttmSlot.numSprites[slotNo] != 0 {
		//grReleaseBmp(ttmSlot, slotNo)
	}

	var sprites []*ebiten.Image
	// loads bitmaps (which become sprite references)
	// eventually they get stored as ttmSlot-> sprite list

	bmpResource := findBMPResource(name)
	ttmSlot.numSprites[slotNo] = int(bmpResource.NumImages)
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

		spriteImg := ebiten.NewImage(width, height)

		for y := 0; y < int(height); y++ {
			for x := 0; x < int(width); x++ {
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

				spriteImg.Set(x, y, c)
				dataOffset = byteIdx
			}
		}
		// segments the data to be the next cel of the sprite.
		data = data[dataOffset+1:]
		sprites = append(sprites, spriteImg)

		ttmSlot.sprites[slotNo][img] = spriteImg
	}
	tmpSprites = sprites
}

func grReleaseBmp() {

}

func grFadeOut() {
	// Does screen transitions like iris, rect sliding
	// Don't necessarily need this day 1
	// May be able to fake it with just simple assets
}
