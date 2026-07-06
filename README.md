# Johnny Castaway Enhanced

**Johnny Castaway Enhanced** is a preservation-focused fork of [Johnny Castaway 2026 by igtoth](https://github.com/igtoth/Johnny-Castaway-2026-Public), which in turn is a fork of [Johnny Castaway 2026 by deckarep](https://github.com/deckarep/Johnny-Castaway-2026).

The ultimate goal of this fork is to add key enhancements, resolve bugs, and make this classic screensaver run as faithfully to the original as possible, fully optimized and smooth on modern **Windows 10 & 11**.

---

## 🚀 Improvements & Fixes in this Fork

Here are the enhancements and fixes implemented in this version:

### 🖥️ Windows Screensaver & Lifecycle
* **Options Dialog (`/c`)**: Implemented a Raylib-based screensaver configuration window (400x430) for setting options (Sound, Load Background, Password Protection, Software OpenGL, "Start of Day" time setting, and Independent instances per monitor). Preferences are saved persistently to `%USERPROFILE%\.johnny_castaway_2026`.
* **Quiet Preview Mode (`/p`)**: Handled screensaver preview calls by exiting cleanly and quietly.
* **Test Mode (`/t` or `-t` [<ADS_FILE>] [<TAG_NO>])**: Added a developer test mode that runs scene sequences sequentially on loop, allowing debugging of animations and behaviors in isolation. By default, it loops the palm tree climb/dive (`ACTIVITY.ADS` tag 4) and the water return (`JOHNNY.ADS` tag 3). You can optionally append a custom scene filename and tag number (e.g. `/t BUILDING.ADS 2` or `/t activity 4`) to run and inspect any specific sequence on loop.
* **Independent Instances per Monitor**: Added an option (disabled by default) to run independent instances of Johnny Castaway on each connected monitor, enabling users to watch different scenes simultaneously. Spawns separate borderless window child processes positioned on each monitor. Moving the mouse or pressing any key on any monitor signals all instances to cleanly close immediately. Sound is automatically disabled on non-primary displays to prevent overlapping audio.
* **Graceful Exit & Display/HDR Cleanup**: Replaced direct hard-kills (`os.Exit`) with state-controlled cleanup flags. This ensures proper teardown, cleanly restoring desktop resolution and HDR display states on exit.
* **Console Window Management**: Hides the background console window when launching the application as a screensaver.
* **Window Flags Cleanup**: Removed unnecessary topmost window flag (`FlagWindowTopmost`) and added resizable flag (`FlagWindowResizable`) to allow proper Windows focus, screensaver lifecycle handling, and window minimization.

### 🎨 Rendering & Scaling
* **Multi-Monitor Spanning**: Automatically detects and spans the screensaver window across all connected displays. Renders a separate, correctly letterboxed (4:3) copy of the scene centered on each monitor individually, with matching monitor-bound circular iris transitions.
* **Widescreen Canvas Scaling**: Implemented letterbox and pillarbox centering of the virtual 4:3 viewport on widescreen resolutions. Clears unused margins to solid black, keeping the scene layout correct.
* **Composed Circular Iris Transitions**: Uses a dedicated global RenderTexture `grFinalRenderSur` to freeze the final frame buffer during `grFadeOut`. This prevents Johnny and other sprite layers from abruptly vanishing during transitions, and increases fade transition speeds to a smooth 30 FPS.
* **Saved Zones Layer (`grSavedZonesLayer`) & Wave Animation Safety**: Redirected background zone copying (`grCopyZoneToBg`) to write to a dedicated persistent layer (`grSavedZonesLayer`) composited after the background. This prevents the background wave/tide animations (which run on separate clocks directly on the background buffer) from permanently corrupting and overwriting saved items like Johnny's sandcastles.
* **Redraw Last Sprite Fallback (`grTryRedrawLastSpriteToBg`)**: Implemented a fallback mechanism during background zone copies to redraw the original sprite at its recorded state instead of doing raw pixel copies from the frame buffer, avoiding rendering artifacts.
* **Direction-Aware Plane Orbit Depth (BUILDING.ADS Tag 2)**: Solved plane-to-tree layering issues during the palm tree plane-chase scene. Flipped (right-to-left) planes are correctly drawn behind Johnny, while non-flipped (left-to-right) planes are drawn in front of him. Also handles compositing multiple simultaneous Johnny-related threads (like the main body and accessories like the binoculars) cleanly in the middle pass.
* **Plane Orbit State Bypass**: Implemented a bypass in `trackLastDraw` to ignore background sandcastle restoration draws, preventing them from clobbering the plane thread's flight direction state (`lastDrawFlipped`).
* **Scissor-Clipping & Coordinate System**: Fixed coordinate evaluation in `grSetClipZone` to treat arguments as absolute bounding box coordinates `(x1, y1) -> (x2, y2)`. Resolved coordinate conversion discrepancy by feeding raw coordinates directly to Raylib's `BeginScissorMode`, which internally handles OpenGL's vertical y-axis flipping. Ensured clip zones persist across `grClearScreen` invocations until explicitly overwritten or reset.
* **Clip Reset Detection Fix (MJDIVE Climb Sequence)**: Corrected clip reset logic so only true full-screen clip zones clear scissoring. This preserves intentional partial clip zones such as `SET_CLIP_ZONE 0,0,639,279` used during `MJDIVE.TTM` tag 2, restoring the original climb-back visual behavior.
* **Coordinate & Scale Fixes**:
  * Corrected background zone copying coordinates and orientation inside `grCopyZoneToBg`.
  * Corrected coconut scale inside bounding boxes in `grDrawCircle`.
  * Resolved sub-scene mismatches on the central island setup.
  * Corrected walk transition coordinate offsets (`ttmDx`/`ttmDy`) in `storyPlay` so that transitions between island halves (e.g. returning from water to the left side of the island) render Johnny at the correct offset immediately.

### ⚙️ Engine, Scripts, and RNG
* **TIMER Opcode RNG**: Fixed the `TIMER` opcode (`0x2022`) to sample uniformly from the target range `[args[0], args[1]]` (instead of taking the static average), restoring the natural jitter/timing variation of the original screensaver.
* **Script Comment & Opcode Corrections**: Verified and fixed several script interpreter assumptions against real `.TTM`/`.ADS` streams (e.g., `SAVE_IMAGE1`, `:TAG` labels for jump routing, mismatch of tag counts, and redundant duplicate scene additions).
* **Local Trigger Chaining (`IF_LASTPLAYED_LOCAL` gating)**: Fixed a bug in `adsPlayTriggeredChunks` where having any local chunk pending globally blocked general/global chunk dispatches. The fallback general dispatch now properly fires for all unrelated scenes and tags while local triggers remain active.

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

