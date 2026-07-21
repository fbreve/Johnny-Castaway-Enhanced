# Johnny Castaway Enhanced

**Johnny Castaway Enhanced** is a preservation-focused fork of [Johnny Castaway 2026 by igtoth](https://github.com/igtoth/Johnny-Castaway-2026-Public), which in turn is a fork of [Johnny Castaway 2026 by deckarep](https://github.com/deckarep/Johnny-Castaway-2026).

The ultimate goal of this fork is to add key enhancements, resolve bugs, and make this classic screensaver run as faithfully to the original as possible, fully optimized and smooth on modern **Windows 10 & 11**.

---

## 🚀 Improvements & Fixes in this Fork

Here are the enhancements and fixes implemented in this version:

### 🖥️ Windows Screensaver & Lifecycle
* **Options Dialog (`/c`)**: Implemented a Raylib-based screensaver configuration window (600x500) for setting options (Sound, Load Background, Widescreen, Scaling Filter, Password Protection, Software OpenGL, "Start of Day" time setting, and Independent instances). Preferences are saved persistently to `%USERPROFILE%\.johnny_castaway_2026`.
* **Quiet Preview Mode (`/p`)**: Handled screensaver preview calls by exiting cleanly and quietly.
* **Test Mode (`/t` or `-t` [<ADS_FILE>] [<TAG_NO>])**: Added a developer test mode that runs scene sequences sequentially on loop, allowing debugging of animations and behaviors in isolation. By default, it loops the palm tree climb/dive (`ACTIVITY.ADS` tag 4) and the water return (`JOHNNY.ADS` tag 3). You can optionally append a custom scene filename and tag number (e.g. `/t BUILDING.ADS 2` or `/t activity 4`) to run and inspect any specific sequence on loop.
* **Independent Instances per Monitor**: Added an option (disabled by default) to run independent instances of Johnny Castaway on each connected monitor, enabling users to watch different scenes simultaneously. Spawns separate borderless window child processes positioned on each monitor. Moving the mouse or pressing any key on any monitor signals all instances to cleanly close immediately. Sound is automatically disabled on non-primary displays to prevent overlapping audio.
* **Interactive Debug Hotkeys (`/k` or `-k`)**: Added runtime hotkeys to pause/unpause, speed up, or single-step through animations. The controls utilize Windows global `GetAsyncKeyState` key polling, allowing them to work focus-free regardless of which monitor's window is active or if focus was temporarily lost.
  * **Space**: Toggle pause / resume (displays a yellow `[PAUSED]` overlay).
  * **Enter**: Advance exactly one frame when paused.
  * **M**: Toggle maximum speed mode (displays a yellow `[MAX SPEED]` overlay and disables logical delay).
  * **Left Shift**: Toggle the debug text overlay (and `debug.log` writing; now correctly requires `-k`, matching the rest of this hotkey group).
  * **Escape**: Exit the screensaver/application cleanly.
* **Render Benchmark Mode (`/b` or `-b`)**: Ported the performance test logic from the original `bench.c` in `jc_reborn`. It executes three timed passes of 3 seconds each to benchmark rendering 1, 4, and 8 concurrent compositing sprite layers on screen. Benchmark results are printed to the console, saved to `bench.log`, and rendered directly on screen across all active display viewports.
* **Default Launch Mode (No flags)**: When double-clicked or run without command-line flags, the screensaver now defaults to screensaver mode (spanning all monitors), but ignores general key presses to prevent accidental exits. In this default watch mode, only the **Escape** key (and mouse movement/clicks) will close the screensaver. Standard Windows screensaver launches using **`/s`** retain the normal "any key exits" screensaver behavior.
* **Graceful Exit & Display/HDR Cleanup**: Replaced direct hard-kills (`os.Exit`) with state-controlled cleanup flags. This ensures proper teardown, cleanly restoring desktop resolution and HDR display states on exit.
* **Console Window Management**: Hides the background console window when launching the application as a screensaver.
* **Window Flags Cleanup**: Removed unnecessary topmost window flag (`FlagWindowTopmost`) and added resizable flag (`FlagWindowResizable`) to allow proper Windows focus, screensaver lifecycle handling, and window minimization.


### 🎨 Rendering & Scaling
* **Multi-Monitor Spanning**: Automatically detects and spans the screensaver window across all connected displays. Renders a separate, correctly letterboxed (4:3) copy of the scene centered on each monitor individually, with matching monitor-bound circular iris transitions.
* **True Widescreen Support (Optional)**: Added a widescreen rendering mode that dynamically extends the virtual canvas width based on the active monitor's aspect ratio.
  * Extends and tiles/repeats the classic ocean background texture to fill the left and right margins, horizontally mirroring alternate copies to seamlessly align dither/checkerboard patterns.
  * Dynamically expands cloud animation boundaries so clouds float across the entire widescreen width.
  * Offsets all sprite coordinates and graphics drawings by `widescreenOffsetX` to keep Johnny's central island centered.
  * When disabled, the screensaver falls back to the standard pillarbox/letterbox centering of the virtual 4:3 viewport.
* **Scaling & Resampling Filters**: Added a dropdown selection menu in the Options window to select how the virtual viewport is upscaled to the display resolution:
  * **Nearest**: Classic pixel-art nearest neighbor scaling.
  * **Bilinear**: Standard bilinear texture filtering for a soft look.
  * **Sharp Bilinear**: Custom shader that upscales to the nearest integer multiple before applying bilinear filtering to the screen size. This keeps pixel edges crisp while ensuring all virtual pixels are exactly the same size, eliminating shimmering/aliasing.
  * **CRT Dither**: Custom shader that combines sharp bilinear coordinate mapping with a horizontal low-pass `[1, 2, 1]` filter. This blends adjacent horizontal pixels to merge dither/checkerboard patterns into solid colors, mimicking composite CRT monitors.
  * **Smart Dither**: Custom shader that analyzes color differences in a local pixel neighborhood to selectively blend only dithered checkerboard areas, keeping outlines, text, and flat colors perfectly sharp.
  * **Aperture Grille**: Custom shader simulating a flat CRT screen, combining sharp bilinear coordinate mapping, subtle aperture grille RGB subpixel stripes, and virtual scanlines (without tube curvature or vignetting).
  * **CRT Simulator**: Custom shader simulating retro tube monitors, combining barrel distortion curvature, vignette lighting falloff, subtle aperture grille RGB subpixels, and virtual scanlines.
  * **Scanlines Overlay (Optional)**: Added a checkbox option to toggle subtle retro scanlines (15% opacity). The scanline grid dynamically centers on the boundaries of the virtual pixels, giving an authentic CRT TV look regardless of physical monitor resolution (bypassed in pure Nearest/Bilinear modes when disabled).
* **Composed Circular Iris Transitions**: Uses a dedicated global RenderTexture `grFinalRenderSur` to freeze the final frame buffer during `grFadeOut`. This prevents Johnny and other sprite layers from abruptly vanishing during transitions, and increases fade transition speeds to a smooth 30 FPS.
* **Saved Zones Layer (`grSavedZonesLayer`) & Wave Animation Safety**: Redirected background zone copying (`grCopyZoneToBg`) to write to a dedicated persistent layer (`grSavedZonesLayer`) composited after the background. This prevents the background wave/tide animations (which run on separate clocks directly on the background buffer) from permanently corrupting and overwriting saved items like Johnny's sandcastles.
* **Redraw Last Sprite Fallback (`grTryRedrawLastSpriteToBg`)**: Implemented a fallback mechanism during background zone copies to redraw the original sprite at its recorded state instead of doing raw pixel copies from the frame buffer, avoiding rendering artifacts. Updated the matching logic to use a bounding-box containment check (allowing a small tolerance) to correctly match and copy sprites even when the script's target copy rectangle is slightly larger and offset from the sprite's draw boundaries (fixing the disappearing yacht/ship in Gulliver's Travels `BUILDING.ADS` Tag 4).
* **RESTORE_ZONE Support**: Enabled `RESTORE_ZONE` in the script interpreter (`ttm.go`) so that persistent background zones (like the anchored ship) are correctly cleared from the background layer when requested by the animation scripts (e.g. when the ship sails away in Gulliver's Travels Tag 64). Reimplemented `grSaveZone`/`grRestoreZone` as a scoped snapshot/restore of just the requested rect (instead of releasing the entire `grSavedZonesLayer`), so erasing one frozen decoration no longer wipes out every other one sharing the same persistent layer.
* **Redraw Last Rect Fallback (`grTryRedrawLastRectToBg`)**: Extended the same "redraw instead of raw pixel copy" fallback used for sprites to also cover `DRAW_RECT`-based freezes. The tanker passing in front of the island (`VISITOR.ADS` Tag 3, `GJVIS6.TTM`) builds its hull not from a sprite but from ~28 solid-color `DRAW_RECT` strips, each frozen with its own `COPY_ZONE_TO_BG` in the same tick - previously only sprite-based freezes had a safe fallback, so these strips regularly failed to freeze correctly (rendering the hull's side as transparent/see-through). Added recency tracking (`lastOpWasRect`) so whichever of sprite or rect was drawn most recently is checked first, avoiding the small bow-cap sprite occasionally being matched instead of the much wider hull rect. Also bridges a small (~2px), genuine authoring gap present in the original `GJVIS6.TTM` data between two of its hull strips, which only became visible once the strips themselves started freezing correctly.
* **Direction-Aware Plane Orbit Depth (BUILDING.ADS Tag 2)**: Solved plane-to-tree layering issues during the palm tree plane-chase scene. Flipped (right-to-left) planes are correctly drawn behind Johnny, while non-flipped (left-to-right) planes are drawn in front of him. Also handles compositing multiple simultaneous Johnny-related threads (like the main body and accessories like the binoculars) cleanly in the middle pass.
* **Plane Orbit State Bypass**: Implemented a bypass in `trackLastDraw` to ignore background sandcastle restoration draws, preventing them from clobbering the plane thread's flight direction state (`lastDrawFlipped`).
* **Scissor-Clipping & Coordinate System**: Fixed coordinate evaluation in `grSetClipZone` to treat arguments as absolute bounding box coordinates `(x1, y1) -> (x2, y2)`. Resolved coordinate conversion discrepancy by feeding raw coordinates directly to Raylib's `BeginScissorMode`, which internally handles OpenGL's vertical y-axis flipping. Ensured clip zones persist across `grClearScreen` invocations until explicitly overwritten or reset.
* **Clip Reset Detection Fix (MJDIVE Climb Sequence)**: Corrected clip reset logic so only true full-screen clip zones clear scissoring. This preserves intentional partial clip zones such as `SET_CLIP_ZONE 0,0,639,279` used during `MJDIVE.TTM` tag 2, restoring the original climb-back visual behavior.
* **Coordinate & Scale Fixes**:
  * Corrected background zone copying coordinates and orientation inside `grCopyZoneToBg`.
  * Corrected coconut scale inside bounding boxes in `grDrawCircle`.
  * Resolved sub-scene mismatches on the central island setup.
  * Corrected walk transition coordinate offsets (`ttmDx`/`ttmDy`) in `storyPlay` so that transitions between island halves (e.g. returning from water to the left side of the island) render Johnny at the correct offset immediately.
  * Corrected custom test mode coordinate positioning so that scenes with `LEFT_ISLAND` flags (like the plane visitor) load the background screen and center offsets shifted left correctly, preventing characters from walking on water.
  * Scaled horizontal coordinates of screen-spanning elements (like the biplane flyby in `GJVIS3.TTM` and `GJVIS5.TTM`, the tanker in `GJVIS6.TTM`, and rescue boats in `WOULDBE.TTM`/`THEEND.TTM`) across the full widescreen viewport width when widescreen mode is enabled. Restricted scaling only to true screen-spanning sprite image slots (e.g. the speedboat or tanker itself) while applying the standard island shift offset (`widescreenOffsetX`) to static island-based sprites in the same TTMs (such as the palm tree trunk overlay or Johnny standing/walking on the island), preventing graphical misalignment.
  * **Stable widescreen scale anchor**: Composite multi-part sprites (like the biplane body and its spinning propeller nose), line-drawn elements (like the waterskiing rope), and grouped sprites that need to stay a fixed distance apart (like a boat's passengers) all need to scale together as one rigid group, not independently - scaling each sprite's `x` independently stretches the gaps between them in proportion to their distance from 0, visibly pulling a towed water-skier's rope loose or floating a passenger off the deck. Fixed by computing the scale delta from a single, known anchor sprite (e.g. the boat hull) found via a lookahead scan at the start of each frame - before any drawing happens that tick - and applying that same delta to every other screen-spanning sprite, line, and rect drawn that frame. This replaced an earlier approach that derived the anchor from whichever spanning sprite happened to be widest on the *previous* tick: besides the one-tick lag causing a stationary boat to visibly step backward the instant it resumed moving after stopping, "whichever sprite is widest" isn't a stable reference from tick to tick (a splash/wave decoration can outrank the boat hull on some frames), which showed up as the boat's own on-screen position wobbling by dozens of pixels between otherwise-identical ticks even though its raw script coordinate never changed. `DRAW_RECT` fills (used for the tanker's hull) and `DRAW_LINE` calls get the same anchor-based scaling, computed from each strip's left/right edges rather than scaling `x` and `width` independently - the latter rounds differently strip to strip and was opening a ~1px seam at nearly every hull strip boundary in widescreen mode.
  * **WALKSTUF.ADS depth sorting**: The boat encounter (`WOULDBE.TTM`) runs Johnny's own reaction (sitting under the tree, startled, watching) and the boat/girl as two independent concurrent TTM threads for a good part of the sequence. Since thread slot assignment just grabs whichever slot is free rather than tracking any deliberate z-order, and Johnny's thread happened to consistently land at a lower slot index than the boat's, plain index-order compositing painted the boat/girl on top of Johnny every time - regardless of the correct sprite order already present inside `WOULDBE.TTM`'s own script (which does draw Johnny after/on top of the girl; that's simply irrelevant once they're on two separate layers). Added a second, simpler always-on-top compositing pass alongside the existing flip-direction-aware plane logic (kept separate since this case has no direction-dependent front/behind - Johnny should just always render on top), keyed by ADS name + TTM tag rather than hardcoded thread indices.
  * **WOULDBE.TTM widescreen & ladder alignment**: `WOULDBE.TTM` is marked screen-spanning so the boat hull (`BOAT.BMP`, slot 4) and passengers (`WOULDBE.BMP`, slot 2) scale continuously with a single rigid anchor across all arrival, parked, and drive-off tags. To keep Johnny aligned with the boat ladder during interactions without shifting static island poses, slot 3 poses are classified dynamically: Johnny swimming to/reaching the boat (`JOHNWOUL.BMP` sprites 6..16) and climbing the boat ladder (`DRUNKJON.BMP` sprites 16..22) scale with the boat anchor, while static island poses (sitting under the tree, staggering on the island) remain shifted by `widescreenOffsetX`.
  * **Widescreen Clip Zone & Line Edge Pinning**: Many TTM scripts (e.g. `MJFISHC.TTM` shark-drag sequence, `FISHWALK`, `GJDIVE`, `MJDIVE`, `MJSAND`) specify `SET_CLIP_ZONE` bounds or `DRAW_LINE` endpoints with `x1 <= 0` or `x2 >= 639` to indicate unconstrained drawing up to the screen boundary. In widescreen mode, applying a standard `widescreenOffsetX` shift centered those 640px bounds within the wider viewport, inadvertently clipping lines and off-screen sprite animations (such as the shark dragging Johnny) at the original 4:3 boundary instead of allowing them to extend to the actual edge of the screen. Updated `grSetClipZone` and `grDrawLine` to pin endpoints touching `0` or `639` to the true canvas edges (`0` and `virtualWidth - 1`) when widescreen mode is enabled.

### ⚙️ Engine, Scripts, and RNG
* **Plane Visitor Duplicate Rendering**: Resolved a race condition where Johnny would duplicate/ghost in the plane visitor scene (`VISITOR.ADS` tag 5) by updating the interpreter's active thread tag (`ttmThread.sceneTag`) dynamically upon hitting tag markers (`0x1111` or `0x1101`), and stopping active walk/stand threads (`tag 7`, `8`, or `10`) concurrently.
* **TIMER Opcode RNG**: Fixed the `TIMER` opcode (`0x2022`) to sample uniformly from the target range `[args[0], args[1]]` (instead of taking the static average), restoring the natural jitter/timing variation of the original screensaver.
* **Script Comment & Opcode Corrections**: Verified and fixed several script interpreter assumptions against real `.TTM`/`.ADS` streams (e.g., `SAVE_IMAGE1`, `:TAG` labels for jump routing, mismatch of tag counts, and redundant duplicate scene additions).
* **Local Trigger Chaining (`IF_LASTPLAYED_LOCAL` gating)**: Fixed a bug in `adsPlayTriggeredChunks` where having any local chunk pending globally blocked general/global chunk dispatches. The fallback general dispatch now properly fires for all unrelated scenes and tags while local triggers remain active.
* **Anti-aliased Config Screen Typography**: Upgraded Setup UI font rendering in `runOptionsWindow()` to load Windows system fonts (`Segoe UI` / `Arial`) as a high-resolution 64pt TTF atlas with bilinear texture filtering (`rl.FilterBilinear`) and zero character spacing, producing crisp, smooth, anti-aliased text across all setup options and dropdown menus.
* **Clean Scene State for Non-Island Endings**: Fixed a bug where leftover cloud and shoreline wave threads from preceding island scenes continued running during non-island scenes (such as `JOHNNY.ADS` tag 1 "The End" or ending sequence tag 6). `adsReleaseIsland` and `adsNoIsland` now explicitly stop and free `ttmCloudsThread` and `ttmBackgroundThread`, ensuring non-island final scenes start with a clean slate. Updated test mode (`-t`) in `main.go` to respect scene `ISLAND` flags so standalone test playback accurately mirrors runtime behavior.

### 💾 Memory & Resource Optimization
* **VRAM Texture Leaks**: Fixed a memory leak in sprite loading (`graphics.go` / `grLoadBmp`) where CPU-side Image data was never unloaded after uploading to GPU VRAM.
* **Optimized Cloud Animations**: Re-enabled the original animated clouds by loading the `BACKGRND.BMP` sprite sheet once at island initialization instead of reloading and re-uploading the entire texture to the GPU every few ticks. This restores the classic clouds with negligible performance overhead.
* **Legacy Hardware & Software OpenGL Compatibility**: Supports preloading the system `opengl32.dll` to keep GPU hardware-acceleration default on modern systems, while allowing legacy/unsupported hardware (like Intel HD Graphics 3000 or virtual machines) to run via CPU-based Mesa Software Rendering (LLVMpipe) by ticking "Use Software OpenGL (Mesa)" in the Setup UI.

---

## 📜 Original Project Details & Credits

Below is the original documentation and history from the parent repositories:

Johnny Castaway 2026 Edition is a Go/Raylib port of [jc_reborn](https://github.com/jno6809/jc_reborn). Without this version, this ported version would not exist, so I want to give credit where credit is due.

### How it's built
* Written in 100% Go - easy to cross-compile to different platforms
* Uses Raylib game framework - can run on consoles even
* Original Goals:
  * Desktop (MacOS, Linux, Windows)
  * WASM

### Tested Files
* `RESOURCE.001` - `md5: 8bb6c99e9129806b5089a39d24228a36`
* `RESOURCE.MAP` - `md5: 374e6d05c5e0acd88fb5af748948c899`

### Resource types
* `.BMP` = used for sprites (4bits per pixel, color indexed (16 color max))
* `.SCR` = used for backgrounds (4bits per pixel, color indexed (16 color max))
* `.ADS` = scene level orchestration (higher level)
* `.TTM` = animation sequencing scripts (lower level)
* `.PAL` = color palette - this game only used up to 16 colors
* `.WAV` = audio - but this engine just references extracted .wav files and plays them

### Other implementations
* https://github.com/jno6809/jc_reborn C (this code is based on this one)
* https://github.com/bailli/Johnny - C++
* [ScummVM DGDS engine - related but not Johnny Castaway](https://github.com/scummvm/scummvm/tree/master/engines/dgds)

### Other references
ScummVM has some more comprehensive implementation of ADS and TTM instruction set, but it's
likely not compatible with Castaway's simple codebase because it looks to be a super-set of this architecture.

### Original Plan of action (deckarep)
* I've tried to make ScreenSaverView work in the past, i'm not going down that rabbit hole
* Instead, I will create a menu bar application, always running, native to MacOS, and offers controls to customize
  functionality as needed such as IDLE_TIMEOUT
* Since that will always be running, after determining idle timeout, it will pop open an app either fullscreen or not
  Additionally we can detect the mouse motion and kill the app (just like a screensaver)
* Only downside is, a user's own screensaver could interfere, so they need to turn off that and other power management
  crap that they might have enabled.
* See DarwinKit examples on menu bar app

