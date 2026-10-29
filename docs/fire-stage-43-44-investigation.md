# BUILDING.ADS fire-stage 43/44 investigation

Status: experiment design only; no production behavior changed.

## Verified interpreter behavior

- `decodeADSInstruction` consumes three words for `0x4000`.
- `0x1360` (`IF_NOT_RUNNING`) sets `inSkipBlock=1` if the referenced scene is running, but does not clear it otherwise.
- `0x2005` (`ADD_SCENE`) and `0x2010` (`STOP_SCENE`) do nothing when `inSkipBlock != 0`.
- `0x4000` currently logs `UNKNOWN_6` without applying `inSkipBlock` or changing scenes.
- `0x1510` (`PLAY_SCENE`) clears a pending skip or terminates the chunk otherwise.

## Controlled comparison protocol

1. Capture the original screensaver and Enhanced at the same fire-stage transitions, including the first appearance of scenes 43 and 44.
2. Record for each executed opcode: ADS tag, byte offset, opcode and arguments, `inSkipBlock` before/after, `isSceneRunning(3,43)` and `isSceneRunning(3,44)` before/after, and scene-add/stop outcomes.
3. Compare the first and second `0x4000(3,44,1)` occurrences separately, for both BUILDING.ADS tags 7 and 8.
4. Repeat when scene 43 is running/not running and scene 44 is running/not running; note which states are actually reachable.
5. Only propose an implementation of `0x4000` if a reproducible behavioral difference establishes the missing effect.

## Open questions

- Does `0x4000` have an observable side effect or is it redundant for this sequence?
- Does skip state persist unexpectedly across adjacent `IF_NOT_RUNNING` checks?
- Which fire-stage transitions can be reached deterministically in the original screensaver?

No assumptions about the unknown opcode's semantics are made here.

## Story selector mapping (verified in `story_data.go`)

- `BUILDING.ADS` tag 7 is a non-final island activity with `ISLAND | VARPOS_OK | LOWTIDE_OK`, and no fixed story day.
- Tag 8 is deliberately omitted from `storyScenes` because its content is identical to tag 7.
- Tags 5 and 9 are likewise marked identical in the source table.
- Internal TTM scenes `(3,43)` and `(3,44)` are **not** story selector entries. Their visual meaning has not been verified; do not call this a fire scene or request a fire reproduction based on these IDs.
- The experiment must first identify the actual visible animation for tags 7/8 and establish whether `0x4000` has a measurable effect. A missing visible fire is not itself a regression.

## Capturing the targeted trace (experimental branch)

Build and launch the experimental branch with `/t BUILDING.ADS 7`. The diagnostic records `ADS_4000_TRACE` entries in `debug.log` in the process working directory without requiring `-k` or the Shift hotkey. It does not prove that the visual sequence is a fire. Preserve the full log and report what the animation actually depicts. The trace records execution state but does not implement opcode `0x4000`.
