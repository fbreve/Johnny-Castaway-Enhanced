# Johnny Castaway Enhanced

**Johnny Castaway Enhanced** is a modified fork of [Johnny Castaway 2026 by igtoth](https://github.com/igtoth/Johnny-Castaway-2026-Public), which in turn is a fork of [Johnny Castaway 2026 by deckarep](https://github.com/deckarep/Johnny-Castaway-2026).

The ultimate goal of this fork is to add key enhancements, resolve bugs, and make this classic screensaver run as faithfully to the original as possible, fully optimized and smooth on modern **Windows 10 & 11**.

---

## 🚀 Improvements & Fixes in this Fork

Here are the enhancements and fixes implemented in this version:

### 🖥️ Windows Screensaver & Lifecycle
* **Options Dialog (`/c`)**: Implemented a Raylib-based screensaver configuration window (400x350) for setting options (Sound, Load Background, Password Protection, and "Start of Day" time setting). Preferences are saved persistently to `%USERPROFILE%\.johnny_castaway_2026`.
* **Quiet Preview Mode (`/p`)**: Handled screensaver preview calls by exiting cleanly and quietly.
* **Graceful Exit & Display/HDR Cleanup**: Replaced direct hard-kills (`os.Exit`) with state-controlled cleanup flags. This ensures proper teardown, cleanly restoring desktop resolution and HDR display states on exit.
* **Console Window Management**: Hides the background console window when launching the application as a screensaver.
* **Window Flags Cleanup**: Removed unnecessary topmost window flag (`FlagWindowTopmost`) and added resizable flag (`FlagWindowResizable`) to allow proper Windows focus, screensaver lifecycle handling, and window minimization.
* **Display Power Watcher / HDR Mitigation (Experimental)**: Added registration for display power state notifications (`GUID_CONSOLE_DISPLAY_STATE`). Minimizes the window and pauses OpenGL render loops when the display turns off, restoring them when it turns back on, to mitigate HDR lock-screen issues (currently unresolved, kept for visibility).

### 🎨 Rendering & Scaling
* **Widescreen Canvas Scaling**: Implemented letterbox and pillarbox centering of the virtual 4:3 viewport on widescreen resolutions. Clears unused margins to solid black, keeping the scene layout correct.
* **Composed Circular Iris Transitions**: Uses a dedicated global RenderTexture `grFinalRenderSur` to freeze the final frame buffer during `grFadeOut`. This prevents Johnny and other sprite layers from abruptly vanishing during transitions, and increases fade transition speeds to a smooth 30 FPS.
* **Coordinate & Scale Fixes**:
  * Corrected background zone copying coordinates and orientation inside `grCopyZoneToBg`.
  * Corrected coconut scale inside bounding boxes in `grDrawCircle`.
  * Resolved sub-scene mismatches on the central island setup.

### ⚙️ Engine, Scripts, and RNG
* **TIMER Opcode RNG**: Fixed the `TIMER` opcode (`0x2022`) to sample uniformly from the target range `[args[0], args[1]]` (instead of taking the static average), restoring the natural jitter/timing variation of the original screensaver.
* **Script Comment & Opcode Corrections**: Verified and fixed several script interpreter assumptions against real `.TTM`/`.ADS` streams (e.g., `SAVE_IMAGE1`, `:TAG` labels for jump routing, mismatch of tag counts, and redundant duplicate scene additions).

### 💾 Memory & Resource Optimization
* **VRAM Texture Leaks**: Fixed a memory leak in sprite loading (`graphics.go` / `grLoadBmp`) where CPU-side Image data was never unloaded after uploading to GPU VRAM.

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

