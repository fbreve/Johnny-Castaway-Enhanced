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
