package main

import "C"
import (
	"encoding/json"
	"fmt"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"strings"

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

// hotkey state — only active when hotKeysEnabled is true (set via -k CLI flag).
var (
	hotKeysEnabled  = false
	isPaused        = false
	isMaxSpeed      = false
	advanceOneFrame = false // set by Enter while paused; consumed after one draw tick
)

// Shared state for multi-monitor synchronization of hotkeys (lock-free multi-file design)
var (
	myStatePath        string
	lastReadTimes      = make(map[string]time.Time)
	lastAdvanceTrigger int
)

var (
	modUser32            = syscall.NewLazyDLL("user32.dll")
	procGetAsyncKeyState = modUser32.NewProc("GetAsyncKeyState")

	prevSpaceDown  = false
	prevMDown      = false
	prevEnterDown  = false
	prevEscapeDown = false
	prevShiftDown  = false
)

func isKeyDownGlobally(vk int) bool {
	r, _, _ := procGetAsyncKeyState.Call(uintptr(vk))
	return (r & 0x8000) != 0
}

func isAnyKeyPressedGlobally() bool {
	for vk := 8; vk <= 255; vk++ {
		if isKeyDownGlobally(vk) {
			return true
		}
	}
	return false
}

type TSharedState struct {
	Paused         bool `json:"p"`
	MaxSpeed       bool `json:"m"`
	AdvanceTrigger int  `json:"a"`
}

func initSharedState() {
	myStatePath = filepath.Join(os.TempDir(), fmt.Sprintf("johnny_state_%d.json", os.Getpid()))
	_ = os.Remove(myStatePath)
}

func writeSharedState() {
	state := TSharedState{
		Paused:         isPaused,
		MaxSpeed:       isMaxSpeed,
		AdvanceTrigger: lastAdvanceTrigger,
	}
	data, err := json.Marshal(state)
	if err == nil {
		_ = os.WriteFile(myStatePath, data, 0644)
	}
}

func readSharedState() {
	pattern := filepath.Join(os.TempDir(), "johnny_state_*.json")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return
	}

	for _, file := range files {
		// Ignore our own state file
		if file == myStatePath {
			continue
		}

		fi, err := os.Stat(file)
		if err != nil {
			continue
		}

		lastTime, exists := lastReadTimes[file]
		if !exists || fi.ModTime().After(lastTime) {
			lastReadTimes[file] = fi.ModTime()
			data, err := os.ReadFile(file)
			if err == nil {
				var state TSharedState
				if json.Unmarshal(data, &state) == nil {
					if state.Paused != isPaused {
						isPaused = state.Paused
					}
					if state.MaxSpeed != isMaxSpeed {
						isMaxSpeed = state.MaxSpeed
					}
					if state.AdvanceTrigger != lastAdvanceTrigger {
						lastAdvanceTrigger = state.AdvanceTrigger
						advanceOneFrame = true
					}
				}
			}
		}
	}
}

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

	// widescreen variables
	virtualWidth      = 640
	virtualHeight     = 480
	widescreenOffsetX = int16(0)

	// r.c. debug instrumentation - tracks nil transitions of grSavedZonesLayer
	// for logging in grUpdateDisplay
	lastSavedZonesLayerWasNil = true
	grFinalRenderSur  *rl.RenderTexture2D
)

var activeClipZones = make(map[*rl.RenderTexture2D]rl.Rectangle)

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
	ResName    string
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

	// r.c. - widescreen scale anchor. Proportional scaling (x * virtualWidth/640)
	// applied independently to each sprite stretches the gaps *between*
	// sprites that are supposed to stay a fixed distance apart (e.g. a boat
	// hull, its passengers, and a towed water-skier) - the further a sprite's
	// x is from 0, the more it gets pushed, so a group that's tightly spaced
	// near the boat visibly spreads apart, on the order of 15-20px at a
	// typical 16:9 ratio. Confirmed empirically against WOULDBE.TTM tag 9
	// ("they drive off"): the boat/passenger gap grows by ~20px.
	//
	// The fix: compute the proportional scale ONCE from the FIRST
	// screen-spanning sprite drawn in a frame (the "anchor"), then apply
	// that SAME delta additively to every other screen-spanning sprite
	// drawn in that same frame, preserving their relative spacing. Reset
	// at every frame boundary (the UPDATE opcode) so the anchor is always
	// fresh - unlike the old version of this mechanism, which computed the
	// delta from whichever sprite happened to be widest in the *previous*
	// frame and reused it a frame late, causing the boat to visibly step
	// backward whenever its motion pattern changed (e.g. stop/restart).
	hasScaleOffset bool
	scaleOffsetX   int16

	// r.c. - tracks the bounding span of all DRAW_SPRITE/DRAW_SPRITE_FLIP
	// positions issued on this thread during its lifetime. Used to decide,
	// on STOP_SCENE, whether this thread was a stationary decoration (e.g.
	// a sandcastle or an anchored ship - worth freezing into the persistent
	// background) versus a moving actor (Johnny, planes, a sailor walking -
	// should just vanish, not leave a ghost behind).
	moveTracked bool
	moveMinX    int16
	moveMaxX    int16
	moveMinY    int16
	moveMaxY    int16
	drawCount   int

	// r.c. - tracks the most recent DRAW_SPRITE/DRAW_SPRITE_FLIP call on
	// this thread. COPY_ZONE_TO_BG is used by original scripts almost
	// always as "freeze the sprite I just drew" (drawn onto ttmLayer, then
	// immediately copied out to the persistent background in the same
	// tick). Reading ttmLayer back as a texture immediately after
	// rendering to it has proven unreliable in testing (confirmed via
	// repeated screenshots), while every other approach that draws the
	// original sprite texture directly works correctly. So when the
	// COPY_ZONE_TO_BG rect matches this last draw, redraw the sprite
	// directly instead of reading ttmLayer back.
	hasLastDraw      bool
	lastDrawX        int16
	lastDrawY        int16
	lastDrawW        int32
	lastDrawH        int32
	lastDrawSpriteNo uint16
	lastDrawImageNo  uint16
	lastDrawFlipped  bool


	// r.c. - same idea as lastDraw above, but for DRAW_RECT. GJVIS6.TTM
	// (VISITOR.ADS tag 3 - the red tanker passing close in front of the
	// island) draws its hull's flat midsection not as a sprite bitmap but
	// as a sequence of solid-color DRAW_RECT strips, each one immediately
	// frozen with its own COPY_ZONE_TO_BG using the exact same rect args.
	// That freeze hit the same same-tick-readback unreliability as sprite
	// freezes, except there was no lastDraw match to redraw from instead
	// (DRAW_RECT never populated it) - the raw copy fell through and
	// often froze a blank/transparent strip, matching the reported bug:
	// the ship's bow and top show fine (drawn via DRAW_SPRITE, covered by
	// lastDraw above), but its side is missing.
	hasLastRect   bool
	lastRectX     int16
	lastRectY     int16
	lastRectW     uint16
	lastRectH     uint16
	lastRectColor uint8

	// r.c. - whichever of DRAW_SPRITE/DRAW_SPRITE_FLIP or DRAW_RECT ran
	// most recently on this thread. GJVIS6.TTM's hull strips draw two
	// small bow-cap sprites *and* the wide hull rect in the same tick,
	// right before COPY_ZONE_TO_BG - and the bow sprite's bounding box
	// sometimes falls within the matching tolerance of the (much wider)
	// rect zone too. Checking sprite-match unconditionally before
	// rect-match then occasionally redraws just the small bow sprite
	// instead of the full hull strip, leaving a thin unpainted gap on the
	// persistent layer for that one strip - exactly the vertical seam
	// reported (island/palm tree visible through a hairline gap). Always
	// trying whichever was drawn last, first, avoids that.
	lastOpWasRect bool

	// r.c. - tracks the right edge of the last rect this thread actually
	// froze onto grSavedZonesLayer (as opposed to lastRect* above, which
	// just tracks the most recent DRAW_RECT call regardless of whether it
	// got frozen). Used to bridge tiny authoring gaps between consecutive
	// strips - see grTryRedrawLastRectToBg.
	hasFrozenRect    bool
	frozenRectRight  int16
	frozenRectTop    int16
	frozenRectBottom int16

	// r.c. - some animations settle into a final position and then cycle
	// between a few frames there (e.g. the ship: a one-time "sails
	// catching wind" frame on arrival, then loops on a "sails full" idle
	// frame forever after). Neither "first sprite ever drawn" (could be
	// mid-transit, off-screen) nor "last sprite drawn" (could be the
	// common loop frame, not the correct at-rest look) is reliable. So we
	// track, scoped to whatever position the thread is CURRENTLY holding
	// steady at, which distinct sprites have been drawn there and how
	// often - resetting whenever the position actually changes. The rarest
	// sprite at the final settled position is what we want.
	settledX          int16
	settledY          int16
	settledEntries     [maxSettledEntries]settledSpriteEntry
	settledEntryCount int
}

const maxSettledEntries = 8

type settledSpriteEntry struct {
	spriteNo uint16
	imageNo  uint16
	flipped  bool
	count    int
}

func grReleaseScreen() {
	grBackgroundSur = nil
}

func grReleaseSavedLayer() {
	debugPrintln("*** SAVED ZONES LAYER RELEASED ***")
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

	rect, hasClip := activeClipZones[sur]
	if hasClip {
		rl.BeginScissorMode(int32(rect.X), int32(rect.Y), int32(rect.Width), int32(rect.Height))
		defer rl.EndScissorMode()
	}

	if x < uint16(virtualWidth) && y < 480 {
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

const sharpBilinearFs = `#version 330
in vec2 fragTexCoord;
in vec4 fragColor;
uniform sampler2D texture0;
out vec4 finalColor;
uniform vec2 textureSize;
uniform vec2 renderSize;
uniform float scanlineWeight;
void main() {
    vec2 texelCoord = fragTexCoord * textureSize - vec2(0.5);
    vec2 integer = floor(texelCoord);
    vec2 scale = renderSize / textureSize;
    vec2 fractionalLocation = clamp((fract(texelCoord) - 0.5) * scale + 0.5, 0.0, 1.0);
    vec2 uv = (integer + fractionalLocation + vec2(0.5)) / textureSize;
    vec4 col = texture(texture0, uv);
    if (scanlineWeight > 0.0) {
        float scanline = 1.0 - scanlineWeight * 0.5 * (1.0 + cos(fract(fragTexCoord.y * textureSize.y) * 2.0 * 3.14159265));
        col.rgb *= scanline;
    }
    finalColor = col * fragColor;
}`

const ditherBlendFs = `#version 330
in vec2 fragTexCoord;
in vec4 fragColor;
uniform sampler2D texture0;
out vec4 finalColor;
uniform vec2 textureSize;
uniform vec2 renderSize;
uniform float scanlineWeight;
void main() {
    vec2 texelCoord = fragTexCoord * textureSize - vec2(0.5);
    vec2 integer = floor(texelCoord);
    vec2 scale = renderSize / textureSize;
    vec2 fractionalLocation = clamp((fract(texelCoord) - 0.5) * scale + 0.5, 0.0, 1.0);
    vec2 uv = (integer + fractionalLocation + vec2(0.5)) / textureSize;

    float texelWidth = 1.0 / textureSize.x;
    vec4 colorC = texture(texture0, uv);
    vec4 colorL = texture(texture0, uv + vec2(-1.0 * texelWidth, 0.0));
    vec4 colorR = texture(texture0, uv + vec2(1.0 * texelWidth, 0.0));
    vec4 col = (colorL + 2.0 * colorC + colorR) * 0.25;
    if (scanlineWeight > 0.0) {
        float scanline = 1.0 - scanlineWeight * 0.5 * (1.0 + cos(fract(fragTexCoord.y * textureSize.y) * 2.0 * 3.14159265));
        col.rgb *= scanline;
    }
    finalColor = col * fragColor;
}`

const smartDitherFs = `#version 330
in vec2 fragTexCoord;
in vec4 fragColor;
uniform sampler2D texture0;
out vec4 finalColor;
uniform vec2 textureSize;
uniform vec2 renderSize;
uniform float scanlineWeight;
void main() {
    vec2 texelCoord = fragTexCoord * textureSize - vec2(0.5);
    vec2 integer = floor(texelCoord);
    vec2 scale = renderSize / textureSize;
    vec2 fractionalLocation = clamp((fract(texelCoord) - 0.5) * scale + 0.5, 0.0, 1.0);
    vec2 uv = (integer + fractionalLocation + vec2(0.5)) / textureSize;

    float texelWidth = 1.0 / textureSize.x;
    float texelHeight = 1.0 / textureSize.y;

    vec4 colorC = texture(texture0, uv);
    vec4 colorL = texture(texture0, uv + vec2(-1.0 * texelWidth, 0.0));
    vec4 colorR = texture(texture0, uv + vec2(1.0 * texelWidth, 0.0));
    vec4 colorU = texture(texture0, uv + vec2(0.0, -1.0 * texelHeight));
    vec4 colorD = texture(texture0, uv + vec2(0.0, 1.0 * texelHeight));

    float diffL = distance(colorC.rgb, colorL.rgb);
    float diffR = distance(colorC.rgb, colorR.rgb);
    float diffLR = distance(colorL.rgb, colorR.rgb);

    float diffU = distance(colorC.rgb, colorU.rgb);
    float diffD = distance(colorC.rgb, colorD.rgb);
    float diffUD = distance(colorU.rgb, colorD.rgb);

    float ditherH = clamp((min(diffL, diffR) - diffLR) * 4.0, 0.0, 1.0);
    float ditherV = clamp((min(diffU, diffD) - diffUD) * 4.0, 0.0, 1.0);
    float ditherAmount = max(ditherH, ditherV);

    vec4 blendedColor = (colorL + 2.0 * colorC + colorR) * 0.25;
    vec4 col = mix(colorC, blendedColor, ditherAmount);
    if (scanlineWeight > 0.0) {
        float scanline = 1.0 - scanlineWeight * 0.5 * (1.0 + cos(fract(fragTexCoord.y * textureSize.y) * 2.0 * 3.14159265));
        col.rgb *= scanline;
    }
    finalColor = col * fragColor;
}`

const scanlineFs = `#version 330
in vec2 fragTexCoord;
in vec4 fragColor;
uniform sampler2D texture0;
out vec4 finalColor;
uniform vec2 textureSize;
uniform float scanlineWeight;
void main() {
    vec4 col = texture(texture0, fragTexCoord);
    if (scanlineWeight > 0.0) {
        float scanline = 1.0 - scanlineWeight * 0.5 * (1.0 + cos(fract(fragTexCoord.y * textureSize.y) * 2.0 * 3.14159265));
        col.rgb *= scanline;
    }
    finalColor = col * fragColor;
}`

const crtSimulatorFs = `#version 330
in vec2 fragTexCoord;
in vec4 fragColor;
uniform sampler2D texture0;
out vec4 finalColor;
uniform vec2 textureSize;
uniform vec2 renderSize;
uniform float scanlineWeight;
uniform float curvatureWeight;
uniform float vignetteWeight;

vec2 curve(vec2 uv) {
    uv = (uv - 0.5) * 2.0;
    uv.x *= 1.0 + (uv.y * uv.y) * 0.04;
    uv.y *= 1.0 + (uv.x * uv.x) * 0.04;
    uv = (uv / 2.0) + 0.5;
    return uv;
}

void main() {
    vec2 curvedUV = fragTexCoord;
    if (curvatureWeight > 0.0) {
        curvedUV = curve(fragTexCoord);
        if (curvedUV.x < 0.0 || curvedUV.x > 1.0 || curvedUV.y < 0.0 || curvedUV.y > 1.0) {
            finalColor = vec4(0.0, 0.0, 0.0, 1.0);
            return;
        }
    }
    vec2 texelCoord = curvedUV * textureSize - vec2(0.5);
    vec2 integer = floor(texelCoord);
    vec2 scale = renderSize / textureSize;
    vec2 fractionalLocation = clamp((fract(texelCoord) - 0.5) * scale + 0.5, 0.0, 1.0);
    vec2 uv = (integer + fractionalLocation + vec2(0.5)) / textureSize;
    vec4 col = texture(texture0, uv);

    float scanline = 1.0 - scanlineWeight * 0.5 * (1.0 + cos(fract(curvedUV.y * textureSize.y) * 2.0 * 3.14159265));
    col.rgb *= scanline;

    float xPixel = fragTexCoord.x * renderSize.x;
    int subpixel = int(mod(xPixel, 3.0));
    vec3 maskColor = vec3(0.9, 0.9, 0.9);
    if (subpixel == 0) maskColor = vec3(1.0, 0.85, 0.85);
    else if (subpixel == 1) maskColor = vec3(0.85, 1.0, 0.85);
    else maskColor = vec3(0.85, 0.85, 1.0);
    col.rgb *= maskColor;

    if (vignetteWeight > 0.0) {
        float vignette = curvedUV.x * curvedUV.y * (1.0 - curvedUV.x) * (1.0 - curvedUV.y);
        vignette = clamp(pow(16.0 * vignette, 0.25), 0.0, 1.0);
        col.rgb *= mix(0.75, 1.0, vignette);
    }

    finalColor = col * fragColor;
}`

var (
	sharpBilinearShader     rl.Shader
	ditherBlendShader       rl.Shader
	smartDitherShader       rl.Shader
	scanlineShader          rl.Shader
	crtSimulatorShader      rl.Shader
	sharpBilinearTexSizeLoc int32
	sharpBilinearRenSizeLoc int32
	sharpBilinearScanLoc    int32
	ditherBlendTexSizeLoc   int32
	ditherBlendRenSizeLoc   int32
	ditherBlendScanLoc      int32
	smartDitherTexSizeLoc   int32
	smartDitherRenSizeLoc   int32
	smartDitherScanLoc      int32
	scanlineTexSizeLoc      int32
	scanlineScanLoc         int32
	crtSimulatorTexSizeLoc  int32
	crtSimulatorRenSizeLoc  int32
	crtSimulatorScanLoc     int32
	crtSimulatorCurveLoc    int32
	crtSimulatorVigLoc      int32
)

func graphicsInit() {
	// todo more stuff
	grLoadPalette(&palResources[0])

	// Calculate virtual width and height
	if activeConfig.Widescreen {
		var aspect float32 = 4.0 / 3.0
		if len(monitorRects) > 0 {
			aspect = monitorRects[0].W / monitorRects[0].H
		} else {
			aspect = float32(rl.GetScreenWidth()) / float32(rl.GetScreenHeight())
		}
		if aspect > 4.0/3.0 {
			virtualHeight = 480
			virtualWidth = int(float32(virtualHeight) * aspect)
			if virtualWidth%2 == 1 {
				virtualWidth++
			}
			widescreenOffsetX = int16((virtualWidth - 640) / 2)
		} else {
			virtualHeight = 480
			virtualWidth = 640
			widescreenOffsetX = 0
		}
	} else {
		virtualHeight = 480
		virtualWidth = 640
		widescreenOffsetX = 0
	}

	// Initialize state path and clean up stale shared files
	initSharedState()
	if files, err := filepath.Glob(filepath.Join(os.TempDir(), "johnny_state_*.json")); err == nil {
		for _, f := range files {
			if f != myStatePath {
				if fi, err := os.Stat(f); err == nil {
					if time.Since(fi.ModTime()) > 30*time.Second {
						_ = os.Remove(f)
					}
				}
			}
		}
	}

	// Mouse position is captured after a few frames in grUpdateDisplay to avoid startup fluctuations
	screenSaverPos = rl.Vector2Zero()

	rt := rl.LoadRenderTexture(int32(virtualWidth), int32(virtualHeight))
	grFinalRenderSur = &rt

	// Load shaders from memory
	sharpBilinearShader = rl.LoadShaderFromMemory("", sharpBilinearFs)
	sharpBilinearTexSizeLoc = rl.GetShaderLocation(sharpBilinearShader, "textureSize")
	sharpBilinearRenSizeLoc = rl.GetShaderLocation(sharpBilinearShader, "renderSize")
	sharpBilinearScanLoc = rl.GetShaderLocation(sharpBilinearShader, "scanlineWeight")

	ditherBlendShader = rl.LoadShaderFromMemory("", ditherBlendFs)
	ditherBlendTexSizeLoc = rl.GetShaderLocation(ditherBlendShader, "textureSize")
	ditherBlendRenSizeLoc = rl.GetShaderLocation(ditherBlendShader, "renderSize")
	ditherBlendScanLoc = rl.GetShaderLocation(ditherBlendShader, "scanlineWeight")

	smartDitherShader = rl.LoadShaderFromMemory("", smartDitherFs)
	smartDitherTexSizeLoc = rl.GetShaderLocation(smartDitherShader, "textureSize")
	smartDitherRenSizeLoc = rl.GetShaderLocation(smartDitherShader, "renderSize")
	smartDitherScanLoc = rl.GetShaderLocation(smartDitherShader, "scanlineWeight")

	scanlineShader = rl.LoadShaderFromMemory("", scanlineFs)
	scanlineTexSizeLoc = rl.GetShaderLocation(scanlineShader, "textureSize")
	scanlineScanLoc = rl.GetShaderLocation(scanlineShader, "scanlineWeight")

	crtSimulatorShader = rl.LoadShaderFromMemory("", crtSimulatorFs)
	crtSimulatorTexSizeLoc = rl.GetShaderLocation(crtSimulatorShader, "textureSize")
	crtSimulatorRenSizeLoc = rl.GetShaderLocation(crtSimulatorShader, "renderSize")
	crtSimulatorScanLoc = rl.GetShaderLocation(crtSimulatorShader, "scanlineWeight")
	crtSimulatorCurveLoc = rl.GetShaderLocation(crtSimulatorShader, "curvatureWeight")
	crtSimulatorVigLoc = rl.GetShaderLocation(crtSimulatorShader, "vignetteWeight")
}

func graphicsEnd() {
	if grFinalRenderSur != nil {
		rl.UnloadRenderTexture(*grFinalRenderSur)
		grFinalRenderSur = nil
	}
	// Clean up our own state file on exit
	if myStatePath != "" {
		_ = os.Remove(myStatePath)
	}
	// Unload shaders
	if sharpBilinearShader.ID > 0 {
		rl.UnloadShader(sharpBilinearShader)
	}
	if ditherBlendShader.ID > 0 {
		rl.UnloadShader(ditherBlendShader)
	}
	if smartDitherShader.ID > 0 {
		rl.UnloadShader(smartDitherShader)
	}
	if scanlineShader.ID > 0 {
		rl.UnloadShader(scanlineShader)
	}
	if crtSimulatorShader.ID > 0 {
		rl.UnloadShader(crtSimulatorShader)
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

	targetAspect := float32(virtualWidth) / float32(virtualHeight)

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
		shiftDown := isKeyDownGlobally(0x10) // VK_SHIFT
		if shiftDown && !prevShiftDown {
			debugEnabled = !debugEnabled
		}
		prevShiftDown = shiftDown

		// --- Debug hot-keys (only when explicitly enabled) ---
		// Works in both normal and screensaver mode; when active, keys are
		// consumed here and the screensaver any-key-exits check below is skipped.
		if hotKeysEnabled {
			readSharedState()

			spaceDown := isKeyDownGlobally(0x20) // VK_SPACE
			if spaceDown && !prevSpaceDown {
				isPaused = !isPaused
				advanceOneFrame = false
				writeSharedState()
			}
			prevSpaceDown = spaceDown

			mDown := isKeyDownGlobally(0x4D) // VK_M
			if mDown && !prevMDown {
				isMaxSpeed = !isMaxSpeed
				writeSharedState()
			}
			prevMDown = mDown

			enterDown := isKeyDownGlobally(0x0D) // VK_RETURN
			if enterDown && !prevEnterDown {
				lastAdvanceTrigger++
				advanceOneFrame = true
				writeSharedState()
			}
			prevEnterDown = enterDown

			escapeDown := isKeyDownGlobally(0x1B) // VK_ESCAPE
			if escapeDown && !prevEscapeDown {
				shouldExitApp = true
				return
			}
			prevEscapeDown = escapeDown
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
			if (grSavedZonesLayer == nil) != lastSavedZonesLayerWasNil {
				lastSavedZonesLayerWasNil = grSavedZonesLayer == nil
				debugPrintf("*** SAVED ZONES LAYER composite state changed: nil=%v ***\n", lastSavedZonesLayerWasNil)
			}
			drawTextureToFinal(grSavedZonesLayer, ModeFlipped)

			// Blit each thread's layer.
			//
			// r.c. - Normally this is a simple array-index pass. But when
			// Johnny (identified explicitly via isJohnnyThread) is present
			// alongside other active threads (planes circling him), the
			// original game shows planes passing BEHIND Johnny when flying
			// right-to-left and IN FRONT when flying left-to-right - a depth
			// cue for orbiting around a fixed point. DRAW_SPRITE_FLIP
			// correlates exactly with right-to-left motion in this data.
			// Plain array-index order can't express "some threads behind,
			// some in front of a specific other thread", so when Johnny
			// is present, split into three passes: flipped/moving-left (right-
			// to-left) threads first (behind), all Johnny threads (middle),
			// then non-flipped/moving-right (left-to-right) threads last (in front).
			//
			// Movement-based heuristics (tight bounding box, relative span
			// comparison) were tried and both failed: Johnny still moves
			// somewhat while animating (fighting the planes), and other
			// unrelated threads (e.g. the anchored ship, also motionless)
			// kept getting misidentified as "Johnny" instead. Explicit
			// identity, confirmed from the disassembly/logs, is reliable
			// where behavioral inference wasn't.
			johnnyIdx := -1
			for i := 0; i < MaxTTMThreads; i++ {
				if ttmThreads[i].isRunning != 0 && isJohnnyThread(ttmThreads[i].sceneSlot, ttmThreads[i].sceneTag) {
					johnnyIdx = i
					break
				}
			}

			onTopIdx := -1
			for i := 0; i < MaxTTMThreads; i++ {
				if ttmThreads[i].isRunning != 0 && isAlwaysOnTopThread(ttmThreads[i].sceneSlot, ttmThreads[i].sceneTag) {
					onTopIdx = i
					break
				}
			}

			{
				activeSummary := ""
				for i := 0; i < MaxTTMThreads; i++ {
					if ttmThreads[i].isRunning != 0 {
						activeSummary += fmt.Sprintf("[#%d slot=%d tag=%d flip=%v] ", i, ttmThreads[i].sceneSlot, ttmThreads[i].sceneTag, ttmThreads[i].lastDrawFlipped)
					}
				}
				if activeSummary != "" {
					debugPrintf("*** compositing: johnnyIdx=%d onTopIdx=%d active=%s***\n", johnnyIdx, onTopIdx, activeSummary)
				}
			}

			if onTopIdx >= 0 {
				for i := 0; i < MaxTTMThreads; i++ {
					if ttmThreads[i].isRunning != 0 && !isAlwaysOnTopThread(ttmThreads[i].sceneSlot, ttmThreads[i].sceneTag) {
						drawTextureToFinal(ttmThreads[i].ttmLayer, ModeFlipped)
					}
				}
				for i := 0; i < MaxTTMThreads; i++ {
					if ttmThreads[i].isRunning != 0 && isAlwaysOnTopThread(ttmThreads[i].sceneSlot, ttmThreads[i].sceneTag) {
						drawTextureToFinal(ttmThreads[i].ttmLayer, ModeFlipped)
					}
				}
			} else if johnnyIdx >= 0 {
				for i := 0; i < MaxTTMThreads; i++ {
					if ttmThreads[i].isRunning != 0 && !isJohnnyThread(ttmThreads[i].sceneSlot, ttmThreads[i].sceneTag) && ttmThreads[i].lastDrawFlipped {
						drawTextureToFinal(ttmThreads[i].ttmLayer, ModeFlipped)
					}
				}
				for i := 0; i < MaxTTMThreads; i++ {
					if ttmThreads[i].isRunning != 0 && isJohnnyThread(ttmThreads[i].sceneSlot, ttmThreads[i].sceneTag) {
						drawTextureToFinal(ttmThreads[i].ttmLayer, ModeFlipped)
					}
				}
				for i := 0; i < MaxTTMThreads; i++ {
					if ttmThreads[i].isRunning != 0 && !isJohnnyThread(ttmThreads[i].sceneSlot, ttmThreads[i].sceneTag) && !ttmThreads[i].lastDrawFlipped {
						drawTextureToFinal(ttmThreads[i].ttmLayer, ModeFlipped)
					}
				}
			} else {
				for i := 0; i < MaxTTMThreads; i++ {
					if ttmThreads[i].isRunning != 0 {
						drawTextureToFinal(ttmThreads[i].ttmLayer, ModeFlipped)
					}
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

			scanlineWeightVal := []float32{0.0}
			if activeConfig.Scanlines {
				scanlineWeightVal[0] = 0.15 // 15% opacity scanlines (subtle retro grid)
			}

			// If scanlines are enabled, we must run a shader even for Nearest/Bilinear
			useShader := true
			if !activeConfig.Scanlines && (activeConfig.FilterMode == 0 || activeConfig.FilterMode == 1) {
				useShader = false
			}

			if useShader {
				switch activeConfig.FilterMode {
				case 1: // Bilinear + Scanlines
					rl.SetTextureFilter(rt.Texture, rl.FilterBilinear)
					texSizeVal := []float32{float32(rt.Texture.Width), float32(rt.Texture.Height)}
					rl.SetShaderValue(scanlineShader, scanlineTexSizeLoc, texSizeVal, rl.ShaderUniformVec2)
					rl.SetShaderValue(scanlineShader, scanlineScanLoc, scanlineWeightVal, rl.ShaderUniformFloat)
					rl.BeginShaderMode(scanlineShader)
					rl.DrawTexturePro(rt.Texture, src, dst, rl.Vector2Zero(), 0, rl.White)
					rl.EndShaderMode()
				case 2: // Sharp Bilinear
					rl.SetTextureFilter(rt.Texture, rl.FilterBilinear)
					texSizeVal := []float32{float32(rt.Texture.Width), float32(rt.Texture.Height)}
					renSizeVal := []float32{destW, destH}
					rl.SetShaderValue(sharpBilinearShader, sharpBilinearTexSizeLoc, texSizeVal, rl.ShaderUniformVec2)
					rl.SetShaderValue(sharpBilinearShader, sharpBilinearRenSizeLoc, renSizeVal, rl.ShaderUniformVec2)
					rl.SetShaderValue(sharpBilinearShader, sharpBilinearScanLoc, scanlineWeightVal, rl.ShaderUniformFloat)
					rl.BeginShaderMode(sharpBilinearShader)
					rl.DrawTexturePro(rt.Texture, src, dst, rl.Vector2Zero(), 0, rl.White)
					rl.EndShaderMode()
				case 3: // Dither Blend (CRT/NTSC horizontal low-pass)
					rl.SetTextureFilter(rt.Texture, rl.FilterBilinear)
					texSizeVal := []float32{float32(rt.Texture.Width), float32(rt.Texture.Height)}
					renSizeVal := []float32{destW, destH}
					rl.SetShaderValue(ditherBlendShader, ditherBlendTexSizeLoc, texSizeVal, rl.ShaderUniformVec2)
					rl.SetShaderValue(ditherBlendShader, ditherBlendRenSizeLoc, renSizeVal, rl.ShaderUniformVec2)
					rl.SetShaderValue(ditherBlendShader, ditherBlendScanLoc, scanlineWeightVal, rl.ShaderUniformFloat)
					rl.BeginShaderMode(ditherBlendShader)
					rl.DrawTexturePro(rt.Texture, src, dst, rl.Vector2Zero(), 0, rl.White)
					rl.EndShaderMode()
				case 4: // Smart Dither (Sharp + dither-only blend)
					rl.SetTextureFilter(rt.Texture, rl.FilterBilinear)
					texSizeVal := []float32{float32(rt.Texture.Width), float32(rt.Texture.Height)}
					renSizeVal := []float32{destW, destH}
					rl.SetShaderValue(smartDitherShader, smartDitherTexSizeLoc, texSizeVal, rl.ShaderUniformVec2)
					rl.SetShaderValue(smartDitherShader, smartDitherRenSizeLoc, renSizeVal, rl.ShaderUniformVec2)
					rl.SetShaderValue(smartDitherShader, smartDitherScanLoc, scanlineWeightVal, rl.ShaderUniformFloat)
					rl.BeginShaderMode(smartDitherShader)
					rl.DrawTexturePro(rt.Texture, src, dst, rl.Vector2Zero(), 0, rl.White)
					rl.EndShaderMode()
				case 5: // Aperture Grille (Flat CRT: Scanlines + RGB subpixels, no curve, no vignette)
					rl.SetTextureFilter(rt.Texture, rl.FilterBilinear)
					texSizeVal := []float32{float32(rt.Texture.Width), float32(rt.Texture.Height)}
					renSizeVal := []float32{destW, destH}
					rl.SetShaderValue(crtSimulatorShader, crtSimulatorTexSizeLoc, texSizeVal, rl.ShaderUniformVec2)
					rl.SetShaderValue(crtSimulatorShader, crtSimulatorRenSizeLoc, renSizeVal, rl.ShaderUniformVec2)
					rl.SetShaderValue(crtSimulatorShader, crtSimulatorScanLoc, scanlineWeightVal, rl.ShaderUniformFloat)
					rl.SetShaderValue(crtSimulatorShader, crtSimulatorCurveLoc, []float32{0.0}, rl.ShaderUniformFloat)
					rl.SetShaderValue(crtSimulatorShader, crtSimulatorVigLoc, []float32{0.0}, rl.ShaderUniformFloat)
					rl.BeginShaderMode(crtSimulatorShader)
					rl.DrawTexturePro(rt.Texture, src, dst, rl.Vector2Zero(), 0, rl.White)
					rl.EndShaderMode()
				case 6: // CRT Simulator (Curved CRT: Scanlines + RGB subpixels + Curvature + Vignette)
					rl.SetTextureFilter(rt.Texture, rl.FilterBilinear)
					texSizeVal := []float32{float32(rt.Texture.Width), float32(rt.Texture.Height)}
					renSizeVal := []float32{destW, destH}
					rl.SetShaderValue(crtSimulatorShader, crtSimulatorTexSizeLoc, texSizeVal, rl.ShaderUniformVec2)
					rl.SetShaderValue(crtSimulatorShader, crtSimulatorRenSizeLoc, renSizeVal, rl.ShaderUniformVec2)
					rl.SetShaderValue(crtSimulatorShader, crtSimulatorScanLoc, scanlineWeightVal, rl.ShaderUniformFloat)
					rl.SetShaderValue(crtSimulatorShader, crtSimulatorCurveLoc, []float32{1.0}, rl.ShaderUniformFloat)
					rl.SetShaderValue(crtSimulatorShader, crtSimulatorVigLoc, []float32{1.0}, rl.ShaderUniformFloat)
					rl.BeginShaderMode(crtSimulatorShader)
					rl.DrawTexturePro(rt.Texture, src, dst, rl.Vector2Zero(), 0, rl.White)
					rl.EndShaderMode()
				default: // 0 - Nearest + Scanlines
					rl.SetTextureFilter(rt.Texture, rl.FilterPoint)
					texSizeVal := []float32{float32(rt.Texture.Width), float32(rt.Texture.Height)}
					rl.SetShaderValue(scanlineShader, scanlineTexSizeLoc, texSizeVal, rl.ShaderUniformVec2)
					rl.SetShaderValue(scanlineShader, scanlineScanLoc, scanlineWeightVal, rl.ShaderUniformFloat)
					rl.BeginShaderMode(scanlineShader)
					rl.DrawTexturePro(rt.Texture, src, dst, rl.Vector2Zero(), 0, rl.White)
					rl.EndShaderMode()
				}
			} else {
				// No shaders (pure Nearest or pure Bilinear)
				if activeConfig.FilterMode == 1 {
					rl.SetTextureFilter(rt.Texture, rl.FilterBilinear)
				} else {
					rl.SetTextureFilter(rt.Texture, rl.FilterPoint)
				}
				rl.DrawTexturePro(rt.Texture, src, dst, rl.Vector2Zero(), 0, rl.White)
			}
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

		// Debug and status overlays — drawn on every active monitor
		rects := monitorRects
		if len(rects) == 0 {
			rects = []TMonitorRect{{X: 0, Y: 0, W: float32(rl.GetScreenWidth()), H: float32(rl.GetScreenHeight())}}
		}

		// Debug stuff
		if debugEnabled {
			fontSize := int32(35)
			offset := int32(3)
			for _, m := range rects {
				yPos := int32(m.Y) + int32(m.H) - (fontSize * 2)
				rl.DrawText(fmt.Sprintf("Story: %d", storyCurrentDay), int32(m.X)+int32(fontSize), yPos, fontSize, rl.Black)
				rl.DrawText(fmt.Sprintf("Story: %d", storyCurrentDay), int32(m.X)+int32(fontSize)-offset, yPos-offset, fontSize, rl.White)

				rl.DrawFPS(int32(m.X)+10, int32(m.Y)+10)
			}
		}

		// Hotkey status overlay — shown whenever hotkeys are active, independent of the debug overlay.
		if hotKeysEnabled {
			statusMsg := ""
			if isPaused {
				statusMsg += "[PAUSED] "
			}
			if isMaxSpeed {
				statusMsg += "[MAX SPEED] "
			}
			if statusMsg != "" {
				for _, m := range rects {
					rl.DrawText(statusMsg, int32(m.X)+10, int32(m.Y)+int32(m.H)/2, 24, rl.Yellow)
				}
			}
		}

		// If screensaver mode is enabled, exit on mouse movement (after settling) or key/mouse press.
		// When hotkeys are active, skip all exit checks so the user can interact and use Esc to exit.
		if isScreensaverMode && !hotKeysEnabled {
			shouldExit := false
			if isRun {
				shouldExit = isAnyKeyPressedGlobally()
			} else {
				shouldExit = isKeyDownGlobally(0x1B) // VK_ESCAPE
			}

			if shouldExit || rl.IsMouseButtonPressed(rl.MouseLeftButton) || rl.IsMouseButtonPressed(rl.MouseRightButton) {
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

		// When paused, spin-wait for either unpause or a single-frame advance.
		// advanceOneFrame is consumed here so the outer timing logic sees
		// grUpdateDelay == 0 and exits immediately after one tick.
		if isPaused && !advanceOneFrame {
			continue
		}
		if advanceOneFrame {
			advanceOneFrame = false
			break
		}

		end := rl.GetTime()
		if isFadingOut || grUpdateDelay == 0 || isMaxSpeed ||
			(end-start) >= (float64(grUpdateDelay)*0.02) {
			break
		}
	}
}

func grNewLayer() *rl.RenderTexture2D {
	rt := rl.LoadRenderTexture(int32(virtualWidth), int32(virtualHeight))
	rl.BeginTextureMode(rt)
	rl.ClearBackground(rl.Blank)
	rl.EndTextureMode()
	return &rt
}

func grFreeLayer(sur *rl.RenderTexture2D) {
	delete(activeClipZones, sur)
	rl.UnloadRenderTexture(*sur)
}

func grSetClipZone(sur *rl.RenderTexture2D, x1, y1, x2, y2 int16) {
	// The TTM args are (x1, y1, x2, y2) — absolute top-left and bottom-right
	// corners of the clip rectangle. The dump disassembler labels arg3/arg4 as
	// "w" and "h" but they are really x2 and y2.
	// Example: SET_CLIP_ZONE x=423 y=148 w=500 h=349 → rect (423,148)→(500,349).
	//
	// The "full screen" reset convention (0,0,639,479) is expressed by scripts
	// in their own unshifted 640x480 coordinate space, so that check MUST be
	// done on the raw args, before grDx/grDy (the island's randomized on-screen
	// position for VARPOS_OK scenes, e.g. BUILDING.ADS) are applied below.
	// Checking it post-offset meant x1<=0 could never be true again once grDx
	// was non-zero, so a real reset call was mistaken for a partial clip zone
	// and stuck around, scissoring away anything drawn outside it afterwards
	// (e.g. later stages of the BUILDING.ADS tag 2 sandcastle animation).
	isFullScreenReset := x1 <= 0 && y1 <= 0 && x2 >= int16(screenWidth-1) && y2 >= int16(screenHeight-1)

	x1 += int16(grDx)
	y1 += int16(grDy)
	x2 += int16(grDx)
	y2 += int16(grDy)

	if activeConfig.Widescreen && sur != ttmCloudsThread.ttmLayer {
		x1 += widescreenOffsetX
		x2 += widescreenOffsetX
	}

	w := x2 - x1
	h := y2 - y1

	if w <= 0 || h <= 0 {
		delete(activeClipZones, sur)
		return
	}

	// Reset clip only when the zone spans the full screen. Some scripts (e.g.
	// MJDIVE tag 2) intentionally use 0,0,639,279 to clip to the upper area;
	// treating any x2>=639 as full-screen wrongly disables that clip.
	if isFullScreenReset {
		delete(activeClipZones, sur)
		return
	}

	// Store the clip rect in GAME coordinates (top-left origin: y increases downward).
	// Raylib's BeginScissorMode(x, y, w, h) internally converts to OpenGL coords via
	// glScissor(x, framebufferHeight-(y+h), w, h), so we must NOT pre-flip y here.
	activeClipZones[sur] = rl.NewRectangle(float32(x1), float32(y1), float32(w), float32(h))
}

func grFreezeLayerToBg(sur *rl.RenderTexture2D) {
	debugPrintf("*** FREEZE LAYER TO BG: surface=%p ***\n", sur)
	if grSavedZonesLayer == nil {
		grSavedZonesLayer = grNewLayer()
	}

	rl.BeginTextureMode(*grSavedZonesLayer)
	defer rl.EndTextureMode()

	// Full-canvas copy, no vertical flip needed here since both sur and
	// grSavedZonesLayer are RenderTextures stored the same way, and we're
	// copying the whole canvas top-to-bottom (not reading a sub-rect that
	// needs the screenHeight-relative flip that grCopyZoneToBg does).
	srcRect := rl.NewRectangle(0, float32(screenHeight), float32(screenWidth), -float32(screenHeight))
	dstRect := rl.NewRectangle(0, 0, float32(screenWidth), float32(screenHeight))
	rl.DrawTexturePro(sur.Texture, srcRect, dstRect, rl.Vector2Zero(), 0.0, rl.White)
}

// grTryRedrawLastSpriteToBg checks whether the requested COPY_ZONE_TO_BG
// rect matches the sprite most recently drawn on this thread, and if so,
// redraws that original sprite texture directly onto the persistent layer
// instead of reading ttmThread.ttmLayer back as a texture (which testing
// has repeatedly shown to be unreliable immediately after rendering to it
// within the same tick). Returns true if it handled the freeze this way.
func grTryRedrawLastSpriteToBg(ttmThread *TTtmThread, x, y int16, width, height uint16) bool {
	if !ttmThread.hasLastDraw {
		return false
	}

	// Check if the last drawn sprite's bounding box is inside (or very close to) the copy zone rect.
	// We allow a small tolerance in case the copy zone is slightly smaller, but usually it is larger.
	const tolerance = 8

	spriteLeft := int(ttmThread.lastDrawX)
	spriteTop := int(ttmThread.lastDrawY)
	spriteRight := spriteLeft + int(ttmThread.lastDrawW)
	spriteBottom := spriteTop + int(ttmThread.lastDrawH)

	rectLeft := int(x)
	rectTop := int(y)
	rectRight := rectLeft + int(width)
	rectBottom := rectTop + int(height)

	matched := (spriteLeft >= rectLeft - tolerance) &&
		(spriteTop >= rectTop - tolerance) &&
		(spriteRight <= rectRight + tolerance) &&
		(spriteBottom <= rectBottom + tolerance)

	if !matched {
		return false
	}

	if grSavedZonesLayer == nil {
		grSavedZonesLayer = grNewLayer()
	}
	debugPrintf("*** COPY_ZONE_TO_BG matched last draw: redrawing sprtNo=%d imgNo=%d at (%d,%d) flipped=%v ***\n",
		ttmThread.lastDrawSpriteNo, ttmThread.lastDrawImageNo, ttmThread.lastDrawX, ttmThread.lastDrawY, ttmThread.lastDrawFlipped)
	if ttmThread.lastDrawFlipped {
		grDrawSpriteFlip(grSavedZonesLayer, ttmThread.ttmSlot, ttmThread.lastDrawX, ttmThread.lastDrawY, ttmThread.lastDrawSpriteNo, ttmThread.lastDrawImageNo)
	} else {
		grDrawSprite(grSavedZonesLayer, ttmThread.ttmSlot, ttmThread.lastDrawX, ttmThread.lastDrawY, ttmThread.lastDrawSpriteNo, ttmThread.lastDrawImageNo)
	}
	return true
}

// grTryRedrawLastRectToBg is the DRAW_RECT counterpart to
// grTryRedrawLastSpriteToBg above: if the requested COPY_ZONE_TO_BG rect
// matches the rect most recently filled with DRAW_RECT on this thread,
// redraw that same solid-color rect directly onto the persistent layer
// instead of reading ttmLayer back.
func grTryRedrawLastRectToBg(ttmThread *TTtmThread, x, y int16, width, height uint16) bool {
	if !ttmThread.hasLastRect {
		return false
	}

	const tolerance = 8

	rectLeft := int(ttmThread.lastRectX)
	rectTop := int(ttmThread.lastRectY)
	rectRight := rectLeft + int(ttmThread.lastRectW)
	rectBottom := rectTop + int(ttmThread.lastRectH)

	zoneLeft := int(x)
	zoneTop := int(y)
	zoneRight := zoneLeft + int(width)
	zoneBottom := zoneTop + int(height)

	matched := (rectLeft >= zoneLeft-tolerance) &&
		(rectTop >= zoneTop-tolerance) &&
		(rectRight <= zoneRight+tolerance) &&
		(rectBottom <= zoneBottom+tolerance)

	if !matched {
		return false
	}

	if grSavedZonesLayer == nil {
		grSavedZonesLayer = grNewLayer()
	}

	drawX := ttmThread.lastRectX
	drawWidth := ttmThread.lastRectW

	// r.c. - GJVIS6.TTM (the tanker passing in front of the island,
	// VISITOR.ADS tag 3) has a genuine 2px gap in its own original data
	// between two of its ~28 hull strips (one ends at x=477, the next
	// starts at x=479 - confirmed via debug log, every strip otherwise
	// tiles exactly edge-to-edge). Once each strip is correctly frozen
	// (as opposed to the previous same-tick-readback bug masking
	// everything), that authoring gap becomes a permanent hairline hole
	// in the persistent background showing whatever was behind it. Bridge
	// any such small gap by extending this strip leftward to butt up
	// against the previous one, as long as they're on the same row.
	const maxBridgeGap = 8
	if ttmThread.hasFrozenRect &&
		ttmThread.frozenRectTop == ttmThread.lastRectY &&
		ttmThread.frozenRectBottom == ttmThread.lastRectY+int16(ttmThread.lastRectH) {
		gap := int(drawX) - int(ttmThread.frozenRectRight)
		if gap > 0 && gap <= maxBridgeGap {
			debugPrintf("*** bridging %dpx gap before rect freeze (prev right edge %d, this strip started at %d) ***\n",
				gap, ttmThread.frozenRectRight, drawX)
			drawWidth += uint16(gap)
			drawX = ttmThread.frozenRectRight
		}
	}

	debugPrintf("*** COPY_ZONE_TO_BG matched last rect: redrawing rect at (%d,%d,%d,%d) color=%d ***\n",
		drawX, ttmThread.lastRectY, drawWidth, ttmThread.lastRectH, ttmThread.lastRectColor)
	grDrawRect(grSavedZonesLayer, ttmThread.ttmSlot, drawX, ttmThread.lastRectY, drawWidth, ttmThread.lastRectH, ttmThread.lastRectColor)

	ttmThread.hasFrozenRect = true
	ttmThread.frozenRectRight = drawX + int16(drawWidth)
	ttmThread.frozenRectTop = ttmThread.lastRectY
	ttmThread.frozenRectBottom = ttmThread.lastRectY + int16(ttmThread.lastRectH)
	return true
}

// grRedrawLastSpriteToBg unconditionally redraws whatever sprite this
// thread most recently drew, directly onto the persistent layer - no
// coordinate matching, no render-texture read-back. Used for the explicit
// STOP_SCENE decoration exceptions (e.g. the anchored ship), which have no
// script-driven COPY_ZONE_TO_BG of their own to trigger off of.
func grRedrawLastSpriteToBg(ttmThread *TTtmThread) {
	if !ttmThread.hasLastDraw {
		debugPrintln("*** STOP_SCENE exception: no last draw recorded, nothing to redraw ***")
		return
	}
	if grSavedZonesLayer == nil {
		grSavedZonesLayer = grNewLayer()
	}
	// Sanity check: has the underlying bitmap at this sprite index changed
	// since we recorded it? ttmThread.ttmSlot is a pointer to a SHARED
	// resource slot - if another scene reused the same resource slot number
	// and reloaded a different BMP into this image bank in the meantime,
	// the pixels here would no longer be what was actually drawn.
	if int(ttmThread.lastDrawImageNo) < len(ttmThread.ttmSlot.sprites) &&
		int(ttmThread.lastDrawSpriteNo) < len(ttmThread.ttmSlot.sprites[ttmThread.lastDrawImageNo]) {
		cur := ttmThread.ttmSlot.sprites[ttmThread.lastDrawImageNo][ttmThread.lastDrawSpriteNo]
		if cur == nil {
			debugPrintln("*** STOP_SCENE exception: WARNING sprite is now nil - underlying bitmap slot was reloaded/cleared since the original draw ***")
		} else if cur.Width != ttmThread.lastDrawW || cur.Height != ttmThread.lastDrawH {
			debugPrintf("*** STOP_SCENE exception: WARNING sprite dimensions changed since original draw (was %dx%d, now %dx%d) - underlying bitmap slot was reloaded ***\n",
				ttmThread.lastDrawW, ttmThread.lastDrawH, cur.Width, cur.Height)
		}
	}
	debugPrintf("*** STOP_SCENE exception: redrawing sprtNo=%d imgNo=%d at (%d,%d) flipped=%v ***\n",
		ttmThread.lastDrawSpriteNo, ttmThread.lastDrawImageNo, ttmThread.lastDrawX, ttmThread.lastDrawY, ttmThread.lastDrawFlipped)
	if ttmThread.lastDrawFlipped {
		grDrawSpriteFlip(grSavedZonesLayer, ttmThread.ttmSlot, ttmThread.lastDrawX, ttmThread.lastDrawY, ttmThread.lastDrawSpriteNo, ttmThread.lastDrawImageNo)
	} else {
		grDrawSprite(grSavedZonesLayer, ttmThread.ttmSlot, ttmThread.lastDrawX, ttmThread.lastDrawY, ttmThread.lastDrawSpriteNo, ttmThread.lastDrawImageNo)
	}
}

// grRedrawRarestSettledSpriteToBg picks, among the sprites drawn at the
// thread's final settled position, whichever one was drawn LEAST often, and
// redraws that directly onto the persistent layer. Confirmed necessary for
// the anchored ship: it draws a one-time "sails catching wind" arrival
// frame (sprtNo:8, drawn once) then loops on a "sails full" frame (sprtNo:9,
// drawn ~50x) at the same position for the rest of its life. Neither
// "literally first sprite ever" (could be mid-transit, off-screen, before
// the thread settles) nor "literally last" (the common loop frame) is
// correct - the rare one-off frame at the final resting spot is what the
// original game shows as the ship's at-rest appearance.
func grRedrawRarestSettledSpriteToBg(ttmThread *TTtmThread) {
	if ttmThread.settledEntryCount == 0 {
		return
	}
	for i := 0; i < ttmThread.settledEntryCount; i++ {
		e := ttmThread.settledEntries[i]
		debugPrintf("*** settled-position candidate: sprtNo=%d imgNo=%d flipped=%v seen=%dx ***\n", e.spriteNo, e.imageNo, e.flipped, e.count)
	}
	best := ttmThread.settledEntries[0]
	for i := 1; i < ttmThread.settledEntryCount; i++ {
		if ttmThread.settledEntries[i].count < best.count {
			best = ttmThread.settledEntries[i]
		}
	}
	if grSavedZonesLayer == nil {
		grSavedZonesLayer = grNewLayer()
	}
	debugPrintf("*** STOP_SCENE exception: redrawing rarest-at-rest sprtNo=%d imgNo=%d (seen %dx) at (%d,%d) flipped=%v ***\n",
		best.spriteNo, best.imageNo, best.count, ttmThread.settledX, ttmThread.settledY, best.flipped)
	if best.flipped {
		grDrawSpriteFlip(grSavedZonesLayer, ttmThread.ttmSlot, ttmThread.settledX, ttmThread.settledY, best.spriteNo, best.imageNo)
	} else {
		grDrawSprite(grSavedZonesLayer, ttmThread.ttmSlot, ttmThread.settledX, ttmThread.settledY, best.spriteNo, best.imageNo)
	}
}

// grRedrawMostCommonSettledSpriteToBg is the inverse of the rarest variant:
// picks whichever sprite was drawn MOST often at the thread's final settled
// position. Visual comparison against the original showed the rare one-off
// frame (sprtNo:8, a "wind gust catching the sail" transient) looks wrong
// frozen in place - the common steady-state loop frame (sprtNo:9) is more
// likely the correct at-rest appearance.
func grRedrawMostCommonSettledSpriteToBg(ttmThread *TTtmThread) {
	if ttmThread.settledEntryCount == 0 {
		return
	}
	for i := 0; i < ttmThread.settledEntryCount; i++ {
		e := ttmThread.settledEntries[i]
		debugPrintf("*** settled-position candidate: sprtNo=%d imgNo=%d flipped=%v seen=%dx ***\n", e.spriteNo, e.imageNo, e.flipped, e.count)
	}
	best := ttmThread.settledEntries[0]
	for i := 1; i < ttmThread.settledEntryCount; i++ {
		if ttmThread.settledEntries[i].count > best.count {
			best = ttmThread.settledEntries[i]
		}
	}
	if grSavedZonesLayer == nil {
		grSavedZonesLayer = grNewLayer()
	}
	debugPrintf("*** STOP_SCENE exception: redrawing most-common-at-rest sprtNo=%d imgNo=%d (seen %dx) at (%d,%d) flipped=%v ***\n",
		best.spriteNo, best.imageNo, best.count, ttmThread.settledX, ttmThread.settledY, best.flipped)
	if best.flipped {
		grDrawSpriteFlip(grSavedZonesLayer, ttmThread.ttmSlot, ttmThread.settledX, ttmThread.settledY, best.spriteNo, best.imageNo)
	} else {
		grDrawSprite(grSavedZonesLayer, ttmThread.ttmSlot, ttmThread.settledX, ttmThread.settledY, best.spriteNo, best.imageNo)
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func grCopyZoneToBg(sur *rl.RenderTexture2D, x, y, width, height uint16) {
	x += uint16(grDx)
	y += uint16(grDy)

	if activeConfig.Widescreen && sur != ttmCloudsThread.ttmLayer {
		x += uint16(widescreenOffsetX)
	}

	// Invert Y for the source rectangle since RenderTexture is flipped vertically in memory.
	srcRect := rl.NewRectangle(float32(x), float32(screenHeight-int(y)), float32(width+2), -float32(height))
	dstRect := rl.NewRectangle(float32(x), float32(y), float32(width+2), float32(height))

	// r.c. - NOT grBackgroundSur: the ambient tide/wave animation
	// (islandAnimate, in island.go) draws directly onto grBackgroundSur on
	// its own timer without ever clearing it, at fixed positions that can
	// overlap a copied zone (e.g. BUILDING.ADS's sandcastle). Since that's
	// the same physical texture rather than a separate composited layer,
	// whichever one draws last permanently overwrites the other's pixels.
	// grSavedZonesLayer is composited *after* background+clouds and *before*
	// the active per-thread layers (see grUpdateDisplay), so it sits above
	// the wave animation and is safe from it.
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

	if activeConfig.Widescreen && sur != ttmCloudsThread.ttmLayer {
		x += widescreenOffsetX
	}

	grPutPixel(sur, uint16(x), uint16(y), clr)
}

func grDrawLine(sur *rl.RenderTexture2D, x1, y1, x2, y2 int16, colorIdx uint8) {
	x1 += int16(grDx)
	y1 += int16(grDy)
	x2 += int16(grDx)
	y2 += int16(grDy)

	if activeConfig.Widescreen && sur != ttmCloudsThread.ttmLayer {
		thread := getThreadByLayer(sur)
		if thread != nil && isScreenSpanningDraw(sur, thread.ttmSlot) {
			// r.c. - see grDrawSprite() for the per-frame anchor rationale.
			// Both endpoints of this line get the SAME delta as every other
			// screen-spanning draw this frame (not each independently
			// proportionally scaled), so a line stays attached to whatever
			// sprite it's meant to connect to.
			if thread.hasScaleOffset {
				x1 += thread.scaleOffsetX
				x2 += thread.scaleOffsetX
			} else {
				scaledX1 := int16(float32(x1) * (float32(virtualWidth) / 640.0))
				thread.scaleOffsetX = scaledX1 - x1
				thread.hasScaleOffset = true
				x1 = scaledX1
				x2 += thread.scaleOffsetX
			}
		} else {
			x1 += widescreenOffsetX
			x2 += widescreenOffsetX
		}
	}

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

	rect, hasClip := activeClipZones[sur]
	if hasClip {
		rl.BeginScissorMode(int32(rect.X), int32(rect.Y), int32(rect.Width), int32(rect.Height))
		defer rl.EndScissorMode()
	}

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

	shift := int16(0)
	if activeConfig.Widescreen && sur != ttmCloudsThread.ttmLayer {
		shift = widescreenOffsetX
	}

	for x := x1; x < x2; x++ {
		grPutPixel(sur, uint16(x+shift), uint16(y), color)
	}
}

func grDrawRect(sur *rl.RenderTexture2D, ttmSlot *TTtmSlot, x, y int16, width, height uint16, colorIdx uint8) {
	if activeConfig.Widescreen && sur != ttmCloudsThread.ttmLayer {
		if isScreenSpanningDraw(sur, ttmSlot) {
			// r.c. - see grDrawSprite() for the per-frame anchor rationale.
			thread := getThreadByLayer(sur)
			if thread != nil {
				if thread.hasScaleOffset {
					x += thread.scaleOffsetX
				} else {
					scaledX := int16(float32(x) * (float32(virtualWidth) / 640.0))
					thread.scaleOffsetX = scaledX - x
					thread.hasScaleOffset = true
					x = scaledX
				}
			} else {
				x = int16(float32(x) * (float32(virtualWidth) / 640.0))
			}
		} else {
			x += widescreenOffsetX
		}
	}

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

	rect, hasClip := activeClipZones[sur]
	if hasClip {
		rl.BeginScissorMode(int32(rect.X), int32(rect.Y), int32(rect.Width), int32(rect.Height))
		defer rl.EndScissorMode()
	}

	rl.DrawRectangle(int32(x), int32(y), int32(width), int32(height), c)
}

func grDrawCircle(sur *rl.RenderTexture2D, x1, y1 int16, width, height uint16, fgColor, bgColor uint8) {
	x1 += int16(grDx)
	y1 += int16(grDy)

	if activeConfig.Widescreen && sur != ttmCloudsThread.ttmLayer {
		x1 += widescreenOffsetX
	}

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

	rect, hasClip := activeClipZones[sur]
	if hasClip {
		rl.BeginScissorMode(int32(rect.X), int32(rect.Y), int32(rect.Width), int32(rect.Height))
		defer rl.EndScissorMode()
	}

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


func trackLastDraw(ttmThread *TTtmThread, x, y int16, spriteNo, imageNo uint16, flipped bool) {
	if int(spriteNo) >= ttmThread.ttmSlot.numSprites[imageNo] {
		return
	}

	// Skip tracking for the sandcastle decoration sprite to prevent it
	// from overwriting the thread's lastDrawFlipped state (the plane's direction).
	if imageNo == 5 && spriteNo == 18 {
		return
	}

	const settleTolerance = 8
	dx := int(x) - int(ttmThread.settledX)
	dy := int(y) - int(ttmThread.settledY)
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	if ttmThread.settledEntryCount == 0 || dx > settleTolerance || dy > settleTolerance {
		// Position moved (or this is the very first draw) - start fresh.
		ttmThread.settledX = x
		ttmThread.settledY = y
		ttmThread.settledEntryCount = 0
	}
	found := false
	for i := 0; i < ttmThread.settledEntryCount; i++ {
		e := &ttmThread.settledEntries[i]
		if e.spriteNo == spriteNo && e.imageNo == imageNo && e.flipped == flipped {
			e.count++
			found = true
			break
		}
	}
	if !found && ttmThread.settledEntryCount < maxSettledEntries {
		ttmThread.settledEntries[ttmThread.settledEntryCount] = settledSpriteEntry{spriteNo: spriteNo, imageNo: imageNo, flipped: flipped, count: 1}
		ttmThread.settledEntryCount++
	}

	srcSurface := ttmThread.ttmSlot.sprites[imageNo][spriteNo]
	ttmThread.hasLastDraw = true
	ttmThread.lastDrawX = x
	ttmThread.lastDrawY = y
	ttmThread.lastDrawW = srcSurface.Width
	ttmThread.lastDrawH = srcSurface.Height
	ttmThread.lastDrawSpriteNo = spriteNo
	ttmThread.lastDrawImageNo = imageNo
	ttmThread.lastDrawFlipped = flipped
	ttmThread.lastOpWasRect = false
}

// trackLastRect is the DRAW_RECT counterpart to trackLastDraw above - see
// the comment on hasLastRect for why this is needed.
func trackLastRect(ttmThread *TTtmThread, x, y int16, width, height uint16, colorIdx uint8) {
	ttmThread.hasLastRect = true
	ttmThread.lastRectX = x
	ttmThread.lastRectY = y
	ttmThread.lastRectW = width
	ttmThread.lastRectH = height
	ttmThread.lastRectColor = colorIdx
	ttmThread.lastOpWasRect = true
}

func trackThreadMovement(ttmThread *TTtmThread, x, y int16) {
	ttmThread.drawCount++
	if !ttmThread.moveTracked {
		ttmThread.moveTracked = true
		ttmThread.moveMinX, ttmThread.moveMaxX = x, x
		ttmThread.moveMinY, ttmThread.moveMaxY = y, y
		return
	}
	if x < ttmThread.moveMinX {
		ttmThread.moveMinX = x
	}
	if x > ttmThread.moveMaxX {
		ttmThread.moveMaxX = x
	}
	if y < ttmThread.moveMinY {
		ttmThread.moveMinY = y
	}
	if y > ttmThread.moveMaxY {
		ttmThread.moveMaxY = y
	}
}

// threadWasStationary reports whether this thread was a long-lived, fixed
// decoration (an anchored ship, say) rather than a moving actor or a
// character briefly pausing. Requires BOTH a tight bounding box (every draw
// stayed close to the first position) AND a substantial number of draws -
// the duration check matters because a character can briefly hold still
// (sitting, perched) and look identical to a decoration by position alone;
// sustained repetition over many ticks is a much more specific signal.
func threadWasStationary(ttmThread *TTtmThread) bool {
	const tolerance = 20
	const minDrawsForDecoration = 80
	if !ttmThread.moveTracked || ttmThread.drawCount < minDrawsForDecoration {
		return false
	}
	return (ttmThread.moveMaxX-ttmThread.moveMinX) <= tolerance &&
		(ttmThread.moveMaxY-ttmThread.moveMinY) <= tolerance
}

func getThreadByLayer(sur *rl.RenderTexture2D) *TTtmThread {
	if sur == nil {
		return nil
	}
	if ttmCloudsThread.ttmLayer == sur {
		return &ttmCloudsThread
	}
	if ttmBackgroundThread.ttmLayer == sur {
		return &ttmBackgroundThread
	}
	if ttmHolidayThread.ttmLayer == sur {
		return &ttmHolidayThread
	}
	for i := 0; i < MaxTTMThreads; i++ {
		if ttmThreads[i].isRunning != 0 && ttmThreads[i].ttmLayer == sur {
			return &ttmThreads[i]
		}
	}
	return nil
}

func isScreenSpanningDraw(sur *rl.RenderTexture2D, ttmSlot *TTtmSlot) bool {
	if ttmSlot == nil {
		return false
	}
	name := strings.ToUpper(ttmSlot.ResName)
	if name == "GJVIS3.TTM" || name == "GJVIS6.TTM" || name == "WOULDBE.TTM" || name == "THEEND.TTM" {
		return true
	}
	if name == "GJVIS5.TTM" {
		// Only tag 9 is the screen-spanning plane flyby
		thread := getThreadByLayer(sur)
		if thread != nil && thread.sceneTag == 9 {
			return true
		}
	}
	return false
}

func shouldScaleSprite(ttmSlot *TTtmSlot, imageNo uint16) bool {
	if ttmSlot == nil {
		return false
	}
	name := strings.ToUpper(ttmSlot.ResName)
	switch name {
	case "GJVIS3.TTM":
		// Only the boat (GJVIS3.BMP, slot 2) spans the screen.
		// Johnny (slots 0 and 1) and the palm tree trunk (slot 3) are static.
		return imageNo == 2
	case "GJVIS6.TTM":
		// Tanker parts (slots 1, 2, 3, 4) span the screen.
		// Johnny (slot 0) is static on the island.
		return imageNo != 0
	case "WOULDBE.TTM":
		// Boat and passengers (slots 2, 4) span the screen.
		// Johnny (slots 0, 3), Trunk (slot 1), and Litebulb (slot 5) are static.
		return imageNo == 2 || imageNo == 4
	case "THEEND.TTM":
		// Credits cover the whole screen.
		return true
	case "GJVIS5.TTM":
		// Tag 9 plane is slot 2.
		return imageNo == 2
	default:
		return false
	}
}

func grDrawSprite(sur *rl.RenderTexture2D, ttmSlot *TTtmSlot, x, y int16, spriteNo, imageNo uint16) {
	if int(spriteNo) >= ttmSlot.numSprites[imageNo] {
		fmt.Printf("Warning : grDrawSprite(): less than %d sprites loaded in slot %d\n", imageNo, spriteNo)
		return
	}

	if activeConfig.Widescreen && sur != ttmCloudsThread.ttmLayer {
		if isScreenSpanningDraw(sur, ttmSlot) && shouldScaleSprite(ttmSlot, imageNo) {
			thread := getThreadByLayer(sur)
			if thread != nil {
				if thread.hasScaleOffset {
					x += thread.scaleOffsetX
				} else {
					scaledX := int16(float32(x) * (float32(virtualWidth) / 640.0))
					thread.scaleOffsetX = scaledX - x
					thread.hasScaleOffset = true
					x = scaledX
				}
			} else {
				x = int16(float32(x) * (float32(virtualWidth) / 640.0))
			}
		} else {
			x += widescreenOffsetX
		}
	}

	x += int16(grDx)
	y += int16(grDy)

	srcSurface := ttmSlot.sprites[imageNo][spriteNo]

	rl.BeginTextureMode(*sur)
	defer rl.EndTextureMode()

	rect, hasClip := activeClipZones[sur]
	if hasClip {
		if sur == grSavedZonesLayer {
			debugPrintf("*** WARNING: clip zone active on SAVED_ZONES_LAYER: rect=(%.0f,%.0f,%.0f,%.0f) ***\n", rect.X, rect.Y, rect.Width, rect.Height)
		}
		rl.BeginScissorMode(int32(rect.X), int32(rect.Y), int32(rect.Width), int32(rect.Height))
		defer rl.EndScissorMode()
	}

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

	if activeConfig.Widescreen && sur != ttmCloudsThread.ttmLayer {
		if isScreenSpanningDraw(sur, ttmSlot) && shouldScaleSprite(ttmSlot, imageNo) {
			// r.c. - see grDrawSprite() above for the per-frame anchor
			// rationale.
			thread := getThreadByLayer(sur)
			if thread != nil {
				if thread.hasScaleOffset {
					x += thread.scaleOffsetX
				} else {
					scaledX := int16(float32(x) * (float32(virtualWidth) / 640.0))
					thread.scaleOffsetX = scaledX - x
					thread.hasScaleOffset = true
					x = scaledX
				}
			} else {
				x = int16(float32(x) * (float32(virtualWidth) / 640.0))
			}
		} else {
			x += widescreenOffsetX
		}
	}

	x += int16(grDx)
	y += int16(grDy)

	srcSurface := ttmSlot.sprites[imageNo][spriteNo]
	//x += int16(srcSurface.Width) - 1 // In original C, but NOT NEEDED, in Raylib.

	rl.BeginTextureMode(*sur)
	defer rl.EndTextureMode()

	rect, hasClip := activeClipZones[sur]
	if hasClip {
		rl.BeginScissorMode(int32(rect.X), int32(rect.Y), int32(rect.Width), int32(rect.Height))
		defer rl.EndScissorMode()
	}

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
	// NOTE: The clip zone is intentionally NOT cleared here. The original game
	// sets a clip zone once with SET_CLIP_ZONE and expects it to persist across
	// many subsequent CLEAR_SCREEN/DRAW_SPRITE/UPDATE cycles. Only a new
	// SET_CLIP_ZONE call (with full-screen coords) resets the clip zone.

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
			// Overwrite the leftmost column (index 0) with column 2
			// and the rightmost column (index 639) with column 637
			// to remove edge flaws while preserving checkerboard dither parity.
			targetX := x
			if x == 0 {
				targetX = 2
			} else if x == width-1 {
				targetX = width - 3
			}

			byteIdx := y*bytesPerRow + (targetX / 2)

			// NOTE: This is a 4bit/per pixel color index
			var colorIdx int
			if targetX%2 == 0 {
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
	defer rl.UnloadTexture(spriteTexture)

	rt := rl.LoadRenderTexture(int32(virtualWidth), int32(virtualHeight))
	grBackgroundSur = &rt

	rl.BeginTextureMode(rt)
	defer rl.EndTextureMode()

	rl.ClearBackground(rl.Black)
	rl.DrawTexture(spriteTexture, int32(widescreenOffsetX), 0, rl.White)

	if activeConfig.Widescreen && widescreenOffsetX > 0 {
		for lx := int32(widescreenOffsetX) - 640; lx > -640; lx -= 640 {
			dist := (int32(widescreenOffsetX) - lx) / 640
			flipHorizontal := (dist % 2) != 0

			if flipHorizontal {
				// Skip column 0 to align checkerboard dithering
				src := rl.NewRectangle(1, 0, -float32(width-1), float32(height))
				dst := rl.NewRectangle(float32(lx+1), 0, float32(width-1), float32(height))
				rl.DrawTexturePro(spriteTexture, src, dst, rl.Vector2Zero(), 0, rl.White)
			} else {
				rl.DrawTexture(spriteTexture, lx, 0, rl.White)
			}
		}
		for rx := int32(widescreenOffsetX) + 640; rx < int32(virtualWidth); rx += 640 {
			dist := (rx - int32(widescreenOffsetX)) / 640
			flipHorizontal := (dist % 2) != 0

			if flipHorizontal {
				// Skip column 639 to align checkerboard dithering
				src := rl.NewRectangle(0, 0, -float32(width-1), float32(height))
				dst := rl.NewRectangle(float32(rx), 0, float32(width-1), float32(height))
				rl.DrawTexturePro(spriteTexture, src, dst, rl.Vector2Zero(), 0, rl.White)
			} else {
				rl.DrawTexture(spriteTexture, rx, 0, rl.White)
			}
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

	rt := rl.LoadRenderTexture(int32(virtualWidth), int32(virtualHeight))
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

	targetAspect := float32(virtualWidth) / float32(virtualHeight)
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
