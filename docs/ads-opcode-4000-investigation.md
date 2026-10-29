# BUILDING.ADS opcode 0x4000 investigation

Status: experiment design only; no production behavior changed.

## Verified interpreter behavior

- `decodeADSInstruction` consumes three words for `0x4000`.
- `0x1360` (`IF_NOT_RUNNING`) sets `inSkipBlock=1` if the referenced scene is running, but does not clear it otherwise.
- `0x2005` (`ADD_SCENE`) and `0x2010` (`STOP_SCENE`) do nothing when `inSkipBlock != 0`.
- `0x4000` currently logs `UNKNOWN_6` without applying `inSkipBlock` or changing scenes.
- `0x1510` (`PLAY_SCENE`) clears a pending skip or terminates the chunk otherwise.

## Interpreting trace records

- `phase=before` means the instruction was decoded and is about to enter its switch case; it does **not** mean the operation was applied.
- `phase=after` captures the state after the switch case, but it does not report the return value of scene operations. A changed `running43`/`running44` value is evidence of a state change, not proof of a visible animation difference.
- `0x1510` may clear `inSkipBlock` or terminate the current chunk. `0xfff0` is logged as a boundary, not as an effect on its own.
- `0x4000` is currently a no-op in the interpreter. Matching before/after states therefore describe the **current implementation**, not the original program’s intended semantics.
- The diagnostic is enabled only by targeted test mode for `BUILDING.ADS`; regular screensaver playback is not traced.

## Controlled comparison protocol

1. Capture the original screensaver and Enhanced while playing BUILDING.ADS tags 7/8, including transitions involving internal scenes 43 and 44.
2. Record for each executed opcode: ADS tag, byte offset, opcode and arguments, `inSkipBlock` before/after, `isSceneRunning(3,43)` and `isSceneRunning(3,44)` before/after, and scene-add/stop outcomes.
3. Compare the first and second `0x4000(3,44,1)` occurrences separately, for both BUILDING.ADS tags 7 and 8.
4. Repeat when scene 43 is running/not running and scene 44 is running/not running; note which states are actually reachable.
5. Only propose an implementation of `0x4000` if a reproducible behavioral difference establishes the missing effect.

## Open questions

- Does `0x4000` have an observable side effect or is it redundant for this sequence?
- Does skip state persist unexpectedly across adjacent `IF_NOT_RUNNING` checks?
- Which internal scene transitions can be reached deterministically in the original screensaver?

No assumptions about the unknown opcode's semantics are made here.

## Story selector mapping (verified in `story_data.go`)

- `BUILDING.ADS` tag 7 is a non-final island activity with `ISLAND | VARPOS_OK | LOWTIDE_OK`, and no fixed story day.
- Tag 8 is deliberately omitted from `storyScenes` because its content is identical to tag 7.
- Tags 5 and 9 are likewise marked identical in the source table.
- Internal TTM scenes `(3,43)` and `(3,44)` are **not** story selector entries. Their visual meaning has not been verified; do not call this a fire scene or request a fire reproduction based on these IDs.
- The experiment must first identify the actual visible animation for tags 7/8 and establish whether `0x4000` has a measurable effect.

## Capturing the targeted trace (experimental branch)

Build and launch the experimental branch with `/t BUILDING.ADS 7`. The diagnostic only emits `ADS_4000_TRACE` entries if matching instructions are actually executed; absence of the file or entries is not evidence that the opcode is absent. The diagnostic records `ADS_4000_TRACE` entries in `debug.log` in the process working directory (not necessarily beside the executable) without requiring `-k` or the Shift hotkey. No visual event identity is assumed. Preserve the full log and report what the animation actually depicts. The trace records execution state but does not implement opcode `0x4000`.

## Reproducible collection checklist

Run the experimental executable with the original proprietary assets available locally; do not commit or upload those assets to GitHub. Use a **separate, empty working directory** for each capture so that `debug.log` contains one session only.

1. Run `/t BUILDING.ADS 7`; record the visible animation, whether the process finished, and the full `debug.log`.
2. Run `/t BUILDING.ADS 8` in a different working directory; record the same information.
3. Run `python tools/analyze_ads_4000_trace.py <path-to-debug.log>` on each capture. Keep the raw log alongside the summary; summaries cannot reconstruct missing details.
4. Check for the `test-mode-start` marker, the number of `0x4000` before records, and any unpaired records. If the trace is missing or incomplete, **do not** interpret unchanged scene state as evidence of a no-op in the original.
5. Report the observed visual sequence separately from internal scene IDs. If comparing against the original program, capture equivalent visible conditions and describe differences without guessing their cause.

**Stop condition:** If tags 7/8 execute without a reproducible discrepancy against the original, leave `0x4000` unimplemented and keep this PR as an investigation. Any implementation proposal needs a concrete failing scenario and a regression test. This trace is an Enhanced-only instrument and cannot itself reveal the original program's internal opcode semantics.
