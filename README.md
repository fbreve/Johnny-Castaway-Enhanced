# Johnny Castaway - 2026 Edition

How it's built
* Written in 100% Go - easy to cross-compile to different platforms
* Uses Raylib game framework - can run on consoles even
* Goals:
  * Desktop (MacOS, Linux, Windows)
  * WASM

Tested Files
* `RESOURCE.001` - `md5: 8bb6c99e9129806b5089a39d24228a36`
* `RESOURCE.MAP` - `md5: 374e6d05c5e0acd88fb5af748948c899`

Resource types:
* `.BMP` = used for sprites (4bits per pixel, color indexed (16 color max))
* `.SCR` = used for backgrounds (4bits per pixel, color indexed (16 color max))
* `.ADS` = scene level orchestration (higher level)
* `.TTM` = animation sequencing scripts (lower level)
* `.PAL` = color palette - this game only used up to 16 colors
* `.WAV` = audio - but this engine just references extracted .wav files and plays them

Other implementations:
* https://github.com/jno6809/jc_reborn C (this code is based on this one)
* https://github.com/bailli/Johnny - C++
* [ScummVM DGDS engine - related but not Johnny Castaway](https://github.com/scummvm/scummvm/tree/master/engines/dgds)

Other references:

ScummVM has some more comprehensive implementation of ADS and TTM instruction set, but it's
likely not compatible with Castaway's simple codebase because it looks to be a super-set of this architecture.
