package main

import (
	"fmt"
	"math/rand"

	rl "github.com/gen2brain/raylib-go/raylib"
)


const (
	MaxRandomOps      = 10
	MaxAdsChunks      = 100
	MaxAdsChunksLocal = 1
)

const (
	OP_ADD_SCENE  = 0
	OP_STOP_SCENE = 1
	OP_NOP        = 2
)

var (
	currentAdsName string

	// r.c. - explicit, confirmed exception list for threads that have no
	// COPY_ZONE_TO_BG (or equivalent) of their own in the script, but are
	// known (verified against the real disassembly, and against the
	// original game) to need their final frame kept as a lasting decoration
	// when stopped. This is deliberately NOT a heuristic: every heuristic
	// tried (position bounding box, draw-count duration) failed because a
	// character can legitimately hold still for a long dramatic beat
	// (Johnny watching the invasion from the palm tree) in a way that's
	// indistinguishable from a genuine decoration by behavior alone. Add
	// entries here only after confirming via the disassembly + a real test
	// that the given (slot,tag) has no explicit freeze of its own AND is
	// meant to persist.
	stopFreezeExceptions = map[string]bool{
		"BUILDING.ADS:1:35": true, // the anchored ship in tag 2 (confirmed: draws sprtNo:8/9 imgNo:4 at fixed 196,124, rect 168x136 matching the visually-confirmed ship size) - ends via natural completion, no COPY_ZONE_TO_BG of its own
	}

	// r.c. - explicit identity for Johnny's own thread during the
	// palm-tree/plane-chase scene. The active tags for Johnny on the tree
	// are 48-52, 55, and 56 (drawing from imgNo:3, MJSANDC.BMP).
	// Tags 46 and 84 are deliberately omitted as they belong to the
	// sandcastle building thread, which would otherwise falsely match and
	// override the primary Johnny thread due to thread index priority.
	// Used to select Johnny's active threads for direction-aware plane
	// compositing (see grUpdateDisplay).
	johnnyThreadTags = map[string]bool{
		"BUILDING.ADS:1:48": true,
		"BUILDING.ADS:1:49": true,
		"BUILDING.ADS:1:50": true,
		"BUILDING.ADS:1:51": true,
		"BUILDING.ADS:1:52": true,
		"BUILDING.ADS:1:55": true,
		"BUILDING.ADS:1:56": true,
	}

	// r.c. - WALKSTUF.ADS (WOULDBE.TTM) runs Johnny's own reaction
	// (sitting under the tree / startled / watching) as one thread
	// CONCURRENTLY with a second thread for the boat+girl, for tags
	// 1, 2, 6 and 14 (confirmed via a live trace: "ADD_SCENE 1 <tag>"
	// followed by "*** compositing: ... active=[#N tag=<johnny tag>]
	// [#M tag=<boat tag>] ***"). Because adsAddScene always picks the
	// lowest free thread slot, and Johnny's thread happens to be
	// allocated first in this scene, Johnny's thread consistently ends
	// up at a LOWER array index than the boat/girl thread - and plain
	// array-index compositing in grUpdateDisplay draws higher indices
	// on top, so the girl/boat rendered in front of Johnny every time,
	// regardless of how the sprites are ordered inside WOULDBE.TTM's own
	// script (which does correctly draw Johnny after/on top of the girl
	// - that's just irrelevant once they're on separate layers).
	// Unlike the BUILDING.ADS plane-chase case, there's no direction-
	// dependent front/behind here - Johnny should simply always be on
	// top - so this uses its own simple always-on-top pass rather than
	// the flip-aware 3-pass logic below.
	alwaysOnTopThreadTags = map[string]bool{
		"WALKSTUF.ADS:1:1":  true,
		"WALKSTUF.ADS:1:2":  true,
		"WALKSTUF.ADS:1:6":  true,
		"WALKSTUF.ADS:1:14": true,
	}
)

func isAlwaysOnTopThread(ttmSlotNo, ttmTag uint16) bool {
	key := fmt.Sprintf("%s:%d:%d", currentAdsName, ttmSlotNo, ttmTag)
	return alwaysOnTopThreadTags[key]
}

func isJohnnyThread(ttmSlotNo, ttmTag uint16) bool {
	key := fmt.Sprintf("%s:%d:%d", currentAdsName, ttmSlotNo, ttmTag)
	return johnnyThreadTags[key]
}

func shouldFreezeOnStop(ttmSlotNo, ttmTag uint16) bool {
	key := fmt.Sprintf("%s:%d:%d", currentAdsName, ttmSlotNo, ttmTag)
	return stopFreezeExceptions[key]
}

var (
	adsChunks    [MaxAdsChunks]TAdsChunk
	numAdsChunks = 0

	adsChunksLocal    [MaxAdsChunksLocal]TAdsChunk
	numAdsChunksLocal = 0

	ttmBackgroundSlot TTtmSlot
	ttmHolidaySlot    TTtmSlot
	ttmCloudsSlot     TTtmSlot
	ttmSlots          [MaxTTMSlots]TTtmSlot

	ttmCloudsThread     TTtmThread
	ttmBackgroundThread TTtmThread
	ttmHolidayThread    TTtmThread
	ttmThreads          [MaxTTMThreads]TTtmThread

	adsTags    []TTtmTag
	adsNumTags = 0

	adsRandOps    [MaxRandomOps]TAdsRandOp
	adsNumRandOps = 0

	numThreads       = 0
	adsStopRequested = 0
)

type TAdsChunk struct {
	scene  TAdsScene
	offset uint32
}

type TAdsRandOp struct {
	ttype    int
	slot     uint16
	tag      uint16
	numPlays uint16
	weight   uint16
}

func adsLoad(data []byte, dataSize uint32, numTags uint16, tag uint16, tagOffset *uint32) {
	var offset uint32 = 0
	var args [10]uint16
	var bookmarkingChunks = 0
	var bookmarkingIfNotRunnings = 0

	numAdsChunks = 0
	numAdsChunksLocal = 0
	*tagOffset = 0
	adsNumTags = 0
	adsTags = make([]TTtmTag, numTags)

	for offset < dataSize {
		opCode := peekUint16(data, &offset)

		switch opCode {
		case 0x1350: // IF_LASTPLAYED
			if bookmarkingChunks != 0 {
				bookmarkingIfNotRunnings = 0
				peekUint16Block(data, &offset, args[:], 2)
				adsChunks[numAdsChunks].scene.slot = args[0]
				adsChunks[numAdsChunks].scene.tag = args[1]
				adsChunks[numAdsChunks].offset = offset
				numAdsChunks++
			} else {
				offset += 2 << 1
			}
		case 0x1360: // IF_NOT_RUNNING
			// We only bookmark the IF_NOT_RUNNINGs
			// preceding the first IF_LAST_PLAYED or IF_IS_RUNNING

			if bookmarkingChunks != 0 && bookmarkingIfNotRunnings != 0 {
				peekUint16Block(data, &offset, args[:], 2)
				adsChunks[numAdsChunks].scene.slot = args[0]
				adsChunks[numAdsChunks].scene.tag = args[1]
				adsChunks[numAdsChunks].offset = offset
				numAdsChunks++
			} else {
				offset += 2 << 1
			}
		case 0x1370: // IF_IS_RUNNING
			bookmarkingIfNotRunnings = 0
			offset += 2 << 1
		case 0x1070:
			offset += 2 << 1
		case 0x1330:
			offset += 2 << 1
		case 0x1420:
			offset += 0 << 1
		case 0x1430:
			offset += 0 << 1
			// OR   // TODO : manage here if_lastplayed OK tags ?
		case 0x1510:
			offset += 0 << 1
		case 0x1520:
			offset += 5 << 1
		case 0x2005:
			offset += 4 << 1
		case 0x2010:
			offset += 3 << 1
		case 0x2014:
			offset += 0 << 1
		case 0x3010:
			offset += 0 << 1
		case 0x3020:
			offset += 1 << 1
		case 0x30ff:
			offset += 0 << 1
		case 0x4000:
			offset += 3 << 1
		case 0xf010:
			offset += 0 << 1
		case 0xf200:
			offset += 1 << 1
		case 0xffff:
			offset += 0 << 1
		case 0xfff0:
			offset += 0 << 1
		default:
			adsTags[adsNumTags].id = opCode
			adsTags[adsNumTags].offset = offset
			adsNumTags++
			if opCode == tag {
				*tagOffset = offset
				bookmarkingChunks = 1
				bookmarkingIfNotRunnings = 1
			} else {
				bookmarkingChunks = 0
				bookmarkingIfNotRunnings = 0
			}
		}
	}

	if adsNumTags != int(numTags) {
		fmt.Println("WARN: didn't find every tag in ADS data")
	}

	if *tagOffset == 0 {
		fmt.Printf("WARN: ADS tag %d not found, starting from offset 0\n", tag)
	}

}

func adsReleaseAds() {
	// free(adsTags)
}

func adsFindTag(reqdTag uint16) uint32 {
	var result uint32 = 0
	i := 0

	for result == 0 && i < adsNumTags {
		if adsTags[i].id == reqdTag {
			result = adsTags[i].offset
		} else {
			i++
		}
	}

	if result == 0 {
		fmt.Printf("WARN: ADS tag %d not found, returning offset 0000\n", reqdTag)
	}
	return result
}

func adsAddScene(ttmSlotNo, ttmTag, arg3 uint16) {
	for i := 0; i < MaxTTMThreads; i++ {
		ttmThread := &ttmThreads[i]

		if ttmThread.isRunning != 0 {
			if ttmThread.sceneSlot == ttmSlotNo && ttmThread.sceneTag == ttmTag {
				fmt.Printf("WARN: (%d,%d) thread is already running - didn't add extra one\n", ttmSlotNo, ttmTag)
				return
			}
		}
	}

	i := 0
	for ttmThreads[i].isRunning != 0 {
		i++
	}

	ttmThread := &ttmThreads[i]

	ttmThread.ttmSlot = &ttmSlots[ttmSlotNo]
	ttmThread.isRunning = 1
	ttmThread.sceneSlot = ttmSlotNo
	ttmThread.sceneTag = ttmTag
	ttmThread.sceneTimer = 0
	ttmThread.sceneIterations = 0
	ttmThread.delay = 4
	ttmThread.timer = 0
	ttmThread.nextGotoOffset = 0
	ttmThread.selectedBmpSlot = 0
	ttmThread.fgColor = 0x0f
	ttmThread.bgColor = 0x0f

	if ttmSlotNo != 0 {
		offset := ttmFindTag(&ttmSlots[ttmSlotNo], ttmTag)
		if offset == 0xffffffff {
			ttmThread.isRunning = 2 // terminate immediately
			ttmThread.ip = 0
		} else {
			ttmThread.ip = offset
		}
	} else {
		ttmThread.ip = 0
	}

	if int16(arg3) < 0 {
		ttmThread.sceneTimer = -(int16(arg3))
	} else if int16(arg3) > 0 {
		ttmThread.sceneIterations = arg3 - 1
	}

	ttmThread.ttmLayer = grNewLayer()
	ttmThread.moveTracked = false
	ttmThread.moveMinX, ttmThread.moveMaxX = 0, 0
	ttmThread.moveMinY, ttmThread.moveMaxY = 0, 0
	ttmThread.drawCount = 0
	ttmThread.hasLastDraw = false
	ttmThread.settledEntryCount = 0
	ttmThread.settledX = 0
	ttmThread.settledY = 0
	numThreads++
}

func adsStopScene(sceneNo int, keepAsDecoration bool) {
	_ = keepAsDecoration
	if shouldFreezeOnStop(ttmThreads[sceneNo].sceneSlot, ttmThreads[sceneNo].sceneTag) {
		// r.c. - TEMP: -t test mode keeps landing on a run where the ship's
		// thread only ever draws sprtNo:8 (confirmed wrong via screenshot)
		// before completing, so the settled-position histogram never gets a
		// chance to also see sprtNo:9 (confirmed present in an earlier,
		// longer story-mode run - looped ~50x). Bypassing the histogram to
		// test sprtNo:9 directly instead of picking whatever this run
		// happened to observe.
		if ttmThreads[sceneNo].sceneSlot == 1 && ttmThreads[sceneNo].sceneTag == 35 {
			if grSavedZonesLayer == nil {
				grSavedZonesLayer = grNewLayer()
			}
			debugPrintln("*** STOP_SCENE exception: TEMP forcing sprtNo=9 imgNo=4 for the ship ***")
			grDrawSprite(grSavedZonesLayer, ttmThreads[sceneNo].ttmSlot, 196, 124, 9, 4)
		} else {
			grRedrawMostCommonSettledSpriteToBg(&ttmThreads[sceneNo])
		}
	}
	grFreeLayer(ttmThreads[sceneNo].ttmLayer)
	ttmThreads[sceneNo].isRunning = 0
	numThreads--
}

func adsStopSceneByTtmTag(ttmSlotNo, ttmTag uint16, keepAsDecoration bool) {
	for i := 0; i < MaxTTMThreads; i++ {
		ttmThread := &ttmThreads[i]
		if ttmThread.isRunning != 0 {
			if ttmThread.sceneSlot == ttmSlotNo {
				match := ttmThread.sceneTag == ttmTag
				if ttmSlotNo == 5 && ttmTag == 10 {
					if ttmThread.sceneTag == 7 || ttmThread.sceneTag == 8 || ttmThread.sceneTag == 10 {
						match = true
					}
				}
				if match {
					adsStopScene(i, keepAsDecoration)
				}
			}
		}
	}
}

func isSceneRunning(ttmSlotNo, ttmTag uint16) int {
	for i := 0; i < MaxTTMThreads; i++ {
		ttmThread := &ttmThreads[i]
		if ttmThread.isRunning != 0 &&
			ttmThread.sceneSlot == ttmSlotNo &&
			ttmThread.sceneTag == ttmTag {
			return 1
		}
	}
	return 0
}

func adsRandomPickOp() *TAdsRandOp {
	totalWeight := 0
	partialWeight := 0
	res := 0

	for i := 0; i < adsNumRandOps; i++ {
		totalWeight += int(adsRandOps[i].weight)
	}

	a := rand.Intn(totalWeight)

	for res = 0; res < adsNumRandOps; res++ {
		partialWeight += int(adsRandOps[res].weight)
		if a < partialWeight {
			break
		}
	}
	return &adsRandOps[res]
}

func adsRandomStart() {
	adsNumRandOps = 0
}

func adsRandomAddScene(ttmSlotNo, ttmTag, numPlays, weight uint16) {
	adsRandOps[adsNumRandOps].ttype = OP_ADD_SCENE
	adsRandOps[adsNumRandOps].slot = ttmSlotNo
	adsRandOps[adsNumRandOps].tag = ttmTag
	adsRandOps[adsNumRandOps].numPlays = numPlays
	adsRandOps[adsNumRandOps].weight = weight
	adsNumRandOps++
}

func adsRandomStopSceneByTtmTag(ttmSlotNo, ttmTag, weight uint16) {
	adsRandOps[adsNumRandOps].ttype = OP_STOP_SCENE
	adsRandOps[adsNumRandOps].slot = ttmSlotNo
	adsRandOps[adsNumRandOps].tag = ttmTag
	adsRandOps[adsNumRandOps].numPlays = 0
	adsRandOps[adsNumRandOps].weight = weight
	adsNumRandOps++
}

func adsRandomNop(weight uint16) {
	adsRandOps[adsNumRandOps].ttype = OP_NOP
	adsRandOps[adsNumRandOps].slot = 0
	adsRandOps[adsNumRandOps].tag = 0
	adsRandOps[adsNumRandOps].numPlays = 0
	adsRandOps[adsNumRandOps].weight = weight
	adsNumRandOps++
}

func adsRandomEnd() {
	if adsNumRandOps != 0 {
		op := adsRandomPickOp()
		switch op.ttype {
		case OP_ADD_SCENE:
			debugPrintf("RANDOM: chose ADD_SCENE %d %d...\n", op.slot, op.tag)
			adsAddScene(op.slot, op.tag, op.numPlays)
		case OP_STOP_SCENE:
			debugPrintf("RANDOM: chose STOP_SCENE %d %d...\n", op.slot, op.tag)
			adsStopSceneByTtmTag(op.slot, op.tag, true)
		default:
			debugPrintln("RANDOM: chose NOP")
		}
	} else {
		debugPrintln("RANDOM: no operation to choose from")
	}
}

func adsInit() {
	for i := 0; i < MaxTTMSlots; i++ {
		ttmInitSlot(&ttmSlots[i])
	}

	for i := 0; i < MaxTTMThreads; i++ {
		ttmThreads[i].isRunning = 0
		ttmThreads[i].timer = 0
	}

	grUpdateDelay = 0
	ttmBackgroundThread.isRunning = 0
	ttmHolidayThread.isRunning = 0
	ttmCloudsThread.isRunning = 0
	numThreads = 0
	adsStopRequested = 0
}

func adsPlaySingleTtm(ttmName string) {
	adsInit()
	ttmLoadTTM(&ttmSlots[0], ttmName)
	adsAddScene(0, 0, 0)
	ttmThreads[0].ip = 0

	for ttmThreads[0].ip < ttmSlots[0].dataSize {
		ttmPlay(&ttmThreads[0])
		ttmThreads[0].isRunning = 1
		grUpdateDisplay(nil, ttmThreads[:], nil, nil)
		if shouldExitApp {
			return
		}
		grUpdateDelay = int(ttmThreads[0].delay)
	}

	adsStopScene(0, false)
	ttmResetSlot(&ttmSlots[0])
}

func adsPlayChunk(data []byte, dataSize, offset uint32) {
	var (
		opcode              uint16 = 0
		args                [10]uint16
		inRandBlock         = 0
		inOrBlock           = 0
		inSkipBlock         = 0
		inIfLastplayedLocal = 0
		continueLoop        = 1
	)

	for continueLoop != 0 && offset < dataSize {
		opcode = peekUint16(data, &offset)

		switch opcode {

		case 0x1070:
			// Inside an IF_LASTPLAYED chunk, local IF_LASTPLAYED
			// which overrides the global IF_LASTPLAYEDs.
			peekUint16Block(data, &offset, args[:], 2)
			debugPrintln("IF_LASTPLAYED_LOCAL")
			inIfLastplayedLocal = 1
			adsChunksLocal[numAdsChunksLocal].scene.slot = args[0]
			adsChunksLocal[numAdsChunksLocal].scene.tag = args[1]
			adsChunksLocal[numAdsChunksLocal].offset = offset
			numAdsChunksLocal++
		case 0x1330:
			// r.c. - confirmed against the original .ADS data: in 54 of 55
			// occurrences across all of the game's .ADS files, this opcode's
			// (args[0], args[1]) exactly matches the (ttm, tag) pair of the
			// ADD_SCENE call that immediately follows it -- consistent with
			// it being IF_NOT_RUNNING, or some closely related check, just
			// as suspected. The one exception found (ACTIVITY.ADS, checking
			// tag 20 but adding tag 42 on the same ttm slot) doesn't change
			// anything in practice: adsAddScene() already refuses to add a
			// scene whose exact (ttm, tag) thread is already running, so
			// whether or not this opcode is treated as a real conditional,
			// the resulting behavior is identical. Safe to keep ignoring it.
			peekUint16Block(data, &offset, args[:], 2)
			debugPrintf("IF_UNKNOWN_1 %d %d\n", args[0], args[1])
		case 0x1350:
			peekUint16Block(data, &offset, args[:], 2)
			debugPrintf("IF_LASTPLAYED %d %d\n", args[0], args[1])
			if inOrBlock == 0 {
				continueLoop = 0
			}

			inOrBlock = 0
		case 0x1360:
			peekUint16Block(data, &offset, args[:], 2)
			debugPrintf("IF_NOT_RUNNING %d %d\n", args[0], args[1])
			if isSceneRunning(args[0], args[1]) != 0 {
				inSkipBlock = 1
			}
		case 0x1370:
			peekUint16Block(data, &offset, args[:], 2)
			debugPrintf("IF_IS_RUNNING %d %d\n", args[0], args[1])
			// r.c - possible bug fixed, the inSkipBlock = 0|1 were swapped originally
			if isSceneRunning(args[0], args[1]) == 0 {
				inSkipBlock = 1
			} else {
				inSkipBlock = 0
			}
		case 0x1420:
			debugPrintln("AND")
		case 0x1430:
			debugPrintln("OR")
			inOrBlock = 1
		case 0x1510:
			// PLAY_SCENE : in fact, sort of a 'closing brace' for a
			// statement block (several types possible).
			// TODO : implement that in a cleaner way.
			// For now, works quite well like that though...
			debugPrintln("PLAY_SCENE")
			if inSkipBlock != 0 {
				inSkipBlock = 0
			} else {
				continueLoop = 0
			}
		case 0x1520:
			// Only in ACTIVITY.ADS tag 7, after IF_LASTPLAYED_LOCAL
			peekUint16Block(data, &offset, args[:], 5)
			debugPrintln("ADD_SCENE_LOCAL")
			if inIfLastplayedLocal != 0 {
				// First pass : the scene was queued by IF_LASTPLAYED_LOCAL,
				// nothing more to do for now
				inIfLastplayedLocal = 0
			} else {
				// Second pass (we were called directly from the scheduler)
				// --> we launch the execution of the scene
				adsAddScene(args[1], args[2], args[3])
			}
		case 0x2005:
			peekUint16Block(data, &offset, args[:], 4)
			debugPrintf("ADD_SCENE %d %d %d %d\n", args[0], args[1], args[2], args[3])
			if inSkipBlock == 0 { // TODO - TEMPO
				if inRandBlock != 0 {
					adsRandomAddScene(args[0], args[1], args[2], args[3])
				} else {
					adsAddScene(args[0], args[1], args[2])
				}
			}
		case 0x2010:
			peekUint16Block(data, &offset, args[:], 3)
			debugPrintf("STOP_SCENE %d %d %d\n", args[0], args[1], args[2])
			if inSkipBlock == 0 { // TODO - TEMPO
				if inRandBlock != 0 {
					adsRandomStopSceneByTtmTag(args[0], args[1], args[2])
				} else {
					adsStopSceneByTtmTag(args[0], args[1], true)
				}
			}
		case 0x3010:
			debugPrintln("RANDOM_START")
			adsRandomStart()
			inRandBlock = 1
		case 0x3020:
			peekUint16Block(data, &offset, args[:], 1)
			debugPrintln("NOP")
			if inRandBlock != 0 {
				adsRandomNop(args[0])
			}
		case 0x30ff:
			debugPrintln("RANDOM_END")
			adsRandomEnd()
			inRandBlock = 0
		case 0x4000:
			peekUint16Block(data, &offset, args[:], 3)
			debugPrintln("UNKNOWN_6") // only in BUILDING.ADS tag 7
		case 0xf010:
			debugPrintln("FADE_OUT")
		case 0xf200:
			peekUint16Block(data, &offset, args[:], 1)
			debugPrintf("GOSUB_TAG %d\n", args[0]) // ex UNKNOWN_8
			// "quick and dirty" implementation, sufficient for
			// JCastaway : only encountered in STAND.ADS to tag 14
			// which only contains 1 scene
			adsPlayChunk(data, dataSize, adsFindTag(args[0]))
		case 0xffff:
			debugPrintln("END")
			if inSkipBlock != 0 {
				// TODO - no doubt this is q&d
				inSkipBlock = 0
			} else {
				adsStopRequested = 1
			}
		case 0xfff0:
			debugPrintln("END_IF")
		default:
			debugPrintf(":TAG %d\n", opcode)
		}
	}
}

func adsPlayTriggeredChunks(data []byte, dataSize uint32, ttmSlotNo, ttmTag uint16) {
	// First we deal with the case where a local trigger was declared
	// (only one occurrence of this, in ACTIVITY.ADS tag #7).
	//
	// r.c. - fixed: this used to gate on `numAdsChunksLocal != 0` globally,
	// which meant that as long as ANY local chunk was pending anywhere, the
	// general dispatch below was skipped entirely for every other tag
	// completion too -- not just the one tag the local chunk was actually
	// registered for. Now we only skip the general dispatch for the
	// specific (slot, tag) pair a local chunk actually matched and fired
	// for; every other tag completion still falls through to the general
	// case as intended.
	localMatched := false
	for i := 0; i < numAdsChunksLocal; i++ {
		if adsChunksLocal[i].scene.slot == ttmSlotNo && adsChunksLocal[i].scene.tag == ttmTag {
			adsPlayChunk(data, dataSize, adsChunksLocal[i].offset)
			numAdsChunksLocal--
			localMatched = true
		}
	}

	if !localMatched {
		// Note : in a few rare cases (eg BUILDING.ADS tag #2), the ADS script
		// contains several 'IF_LASTPLAYED' commands for one given scene.
		for i := 0; i < numAdsChunks; i++ {
			if adsChunks[i].scene.slot == ttmSlotNo && adsChunks[i].scene.tag == ttmTag {
				adsPlayChunk(data, dataSize, adsChunks[i].offset)
			}
		}
	}
}

func adsPlay(adsName string, adsTag uint16) {
	var (
		offset   uint32 = 0
		data     []byte
		dataSize uint32
	)

	adsResource := findAdsResource(adsName)
	debugPrintf("\n\n========== Playing ADS: %s:%d ==========\n", adsResource.ResName, adsTag)
	currentAdsName = adsResource.ResName

	data = adsResource.UncompressedData
	dataSize = adsResource.UncompressedSize

	for i := 0; i < int(adsResource.NumRes); i++ {
		ttmLoadTTM(&ttmSlots[adsResource.Res[i].ID], adsResource.Res[i].Name)
	}

	adsLoad(data, dataSize, adsResource.NumTags, adsTag, &offset)

	adsStopRequested = 0
	grUpdateDelay = 0

	// Play the first ADS chunk of the sequence
	adsPlayChunk(data, dataSize, offset)

	// Main ADS loop
	for numThreads != 0 {
		if ttmBackgroundThread.isRunning != 0 && ttmBackgroundThread.timer == 0 {
			debugPrintln("    ------> Animate bg")
			ttmBackgroundThread.timer = ttmBackgroundThread.delay
			islandAnimate(&ttmBackgroundThread)
		}

		if ttmCloudsThread.isRunning != 0 && ttmCloudsThread.timer == 0 {
			debugPrintln("    ------> Animate clouds")
			ttmCloudsThread.timer = ttmCloudsThread.delay
			islandAnimateClouds(&ttmCloudsThread)
		}

		for i := 0; i < MaxTTMThreads; i++ {
			// Call ttmPlay() for each thread which timer reaches 0
			if ttmThreads[i].isRunning != 0 && ttmThreads[i].timer == 0 {
				debugPrintf("    ------> Thread #%d\n", i)
				ttmThreads[i].timer = ttmThreads[i].delay
				ttmPlay(&ttmThreads[i])
			}
		}

		// r.c. todo
		//if debugMode {
		//
		//}

		// Refresh display
		grUpdateDisplay(&ttmBackgroundThread, ttmThreads[:], &ttmHolidayThread, &ttmCloudsThread)
		if shouldExitApp {
			return
		}

		// Determine min timer through all threads
		mini := uint16(300)

		if ttmBackgroundThread.isRunning != 0 {
			mini = ttmBackgroundThread.timer
		}

		if ttmCloudsThread.isRunning != 0 {
			if ttmCloudsThread.timer < mini {
				mini = ttmCloudsThread.timer
			}
		}

		for i := 0; i < MaxTTMThreads; i++ {
			if ttmThreads[i].isRunning != 0 {
				if ttmThreads[i].delay < mini {
					mini = ttmThreads[i].delay
				}

				if ttmThreads[i].timer < mini {
					mini = ttmThreads[i].timer
				}
			}
		}

		// Decrease all timers by the shortest one, and wait that amount of time
		ttmBackgroundThread.timer -= mini
		ttmCloudsThread.timer -= mini

		for i := 0; i < MaxTTMThreads; i++ {
			if ttmThreads[i].isRunning != 0 {
				ttmThreads[i].timer -= mini
			}
		}

		debugPrintf(" ******* WAIT: %d ticks *******\n", mini)
		grUpdateDelay = int(mini)

		// Various threads processes
		for i := 0; i < MaxTTMThreads; i++ {
			if ttmThreads[i].isRunning != 0 && ttmThreads[i].timer == 0 {
				// Process jumps
				if ttmThreads[i].nextGotoOffset != 0 {
					ttmThreads[i].ip = ttmThreads[i].nextGotoOffset
					ttmThreads[i].nextGotoOffset = 0
				}

				// Managing the timer which was indicated in ADD_SCENE arg3 (neg. value)
				if ttmThreads[i].sceneTimer > 0 {
					ttmThreads[i].sceneTimer -= int16(ttmThreads[i].delay)
					if ttmThreads[i].sceneTimer <= 0 {
						ttmThreads[i].isRunning = 2
					}
				}

				// Free terminated threads
				if ttmThreads[i].isRunning == 2 {
					// Managing the numPlays which was indicated in ADD_SCENE arg3 (postive value)
					if ttmThreads[i].sceneIterations != 0 {
						ttmThreads[i].sceneIterations--
						offset := ttmFindTag(&ttmSlots[ttmThreads[i].sceneSlot], ttmThreads[i].sceneTag)
						if offset == 0xffffffff {
							ttmThreads[i].isRunning = 2
							ttmThreads[i].ip = 0
						} else {
							ttmThreads[i].isRunning = 1
							ttmThreads[i].ip = offset
						}
					} else { // Is there one (or more) IF_LASTPLAYED matching the terminated thread ?
						adsStopScene(i, true)
						if adsStopRequested == 0 {
							adsPlayTriggeredChunks(data, dataSize, ttmThreads[i].sceneSlot, ttmThreads[i].sceneTag)
						}
					}
				}
			}
		}
	}

	for i := 0; i < MaxTTMSlots; i++ {
		ttmResetSlot(&ttmSlots[i])
	}

	grRestoreZone(nil, 0, 0, 0, 0)

	adsReleaseAds()
}

func adsPlayBench() []string {
	// Mirrors jc_reborn's adsPlayBench() / benchInit() / benchPlay().
	// Runs three timed passes (1, 4, 8 sprite layers) for 3 seconds each.
	// Returns result strings (windowsgui binary has no stdout).

	var results []string
	numsLayers := []int{1, 4, 8}

	adsInit()

	// Allocate layers for all 8 threads and point them at slot 0
	for i := 0; i < 8; i++ {
		ttmThreads[i].ttmSlot = &ttmSlots[0]
		ttmThreads[i].isRunning = 1
		ttmThreads[i].selectedBmpSlot = 0
		ttmThreads[i].ttmLayer = grNewLayer()
	}

	// benchInit: load the ocean background + boat sprite sheet
	grLoadScreen("OCEAN00.SCR")
	grLoadBmp(&ttmSlots[0], 0, "BOAT.BMP")

	grUpdateDelay = 0

	boatX := [8]int{}

	for _, numLayers := range numsLayers {
		for i := 0; i < MaxTTMThreads; i++ {
			if i < numLayers {
				ttmThreads[i].isRunning = 1
			} else {
				ttmThreads[i].isRunning = 0
			}
		}

		startTime := rl.GetTime()
		counter := 0

		for rl.GetTime()-startTime <= 3.0 {
			// benchPlay: clear each active layer and draw the bouncing boat
			for i := 0; i < numLayers; i++ {
				if ttmThreads[i].ttmLayer != nil {
					rl.BeginTextureMode(*ttmThreads[i].ttmLayer)
					rl.ClearBackground(rl.Blank)
					rl.EndTextureMode()
				}
				grDrawSprite(ttmThreads[i].ttmLayer, &ttmSlots[0], int16(boatX[i]), int16(180+25*i), 0, 0)
				boatX[i] = (boatX[i] + 5) % 640
			}

			grUpdateDisplay(nil, ttmThreads[:], nil, nil)
			if shouldExitApp {
				goto benchDone
			}
			counter++
		}

		results = append(results, fmt.Sprintf("%d-layer bench --> %d fps", numLayers, counter/3))
	}

benchDone:
	for i := 0; i < 8; i++ {
		adsStopScene(i, false)
	}
	ttmResetSlot(&ttmSlots[0])
	return results
}


func adsPlayIntro() {
	grLoadScreen("INTRO.SCR")
	grUpdateDelay = 100
	grUpdateDisplay(nil, ttmThreads[:], nil, nil)
	grFadeOut()
	ttmResetSlot(&ttmSlots[0])
}

func adsInitIsland() {
	// Init the background thread (animated waves)
	// and call islandInit() to draw the background

	ttmInitSlot(&ttmBackgroundSlot)

	ttmBackgroundThread.ttmSlot = &ttmBackgroundSlot
	if activeConfig.Background {
		ttmBackgroundThread.isRunning = 3
	} else {
		ttmBackgroundThread.isRunning = 0
	}
	ttmBackgroundThread.delay = 40 // TODO
	ttmBackgroundThread.timer = 0

	islandInit(&ttmBackgroundThread)

	// Init the "holiday" layer and thread

	ttmInitSlot(&ttmHolidaySlot)

	ttmHolidayThread.ttmSlot = &ttmHolidaySlot
	ttmHolidayThread.isRunning = 0
	ttmHolidayThread.delay = 0
	ttmHolidayThread.timer = 0

	islandInitHoliday(&ttmHolidayThread)

	// Clouds
	if activeConfig.Background {
		ttmInitSlot(&ttmCloudsSlot)
		ttmCloudsThread.ttmSlot = &ttmCloudsSlot
		ttmCloudsThread.isRunning = 3
		ttmCloudsThread.delay = 8
		ttmCloudsThread.timer = 0
		if ttmCloudsThread.ttmLayer != nil {
			// r.c. - original C has these lines swapped which is incorrect logic
			grFreeLayer(ttmCloudsThread.ttmLayer)
			ttmCloudsThread.ttmLayer = nil
		}
		ttmCloudsThread.ttmLayer = grNewLayer()

		// r.c. - load the cloud sprite sheet once here, at init, instead of
		// every animation tick. islandAnimateClouds previously reloaded
		// this same BMP (a full resource decompress + GPU texture upload)
		// on every tick just to redraw clouds a few pixels over, which is
		// the actual cost behind the "super taxing" complaint that led to
		// clouds being disabled entirely. Only positions change per tick;
		// the sprite sheet itself doesn't need to be reloaded each time.
		grLoadBmp(&ttmCloudsSlot, 0, "BACKGRND.BMP")

		islandAnimateClouds(&ttmCloudsThread)
	} else {
		ttmCloudsThread.isRunning = 0
	}
}

func adsReleaseIsland() {
	ttmBackgroundThread.isRunning = 0
	ttmResetSlot(&ttmBackgroundSlot)

	// r.c. - this used to leave ttmCloudsThread running: adsInitIsland()
	// turns clouds on (isRunning=3) whenever an ISLAND scene starts, but
	// nothing ever turned them back off again when leaving one. Since the
	// next scene picked by storyPlay() can easily be a non-ISLAND FINAL
	// scene (e.g. JOHNNY.ADS tag 6/1, the day-10/day-11 endings), those
	// scenes would inherit clouds still drifting from whatever island
	// scene came before, even though their own TTM script never draws or
	// wants clouds. Stop them symmetrically with the background thread.
	if ttmCloudsThread.isRunning != 0 {
		ttmCloudsThread.isRunning = 0
		if ttmCloudsThread.ttmLayer != nil {
			grFreeLayer(ttmCloudsThread.ttmLayer)
			ttmCloudsThread.ttmLayer = nil
		}
	}

	if ttmHolidayThread.isRunning != 0 {
		ttmHolidayThread.isRunning = 0
		grFreeLayer(ttmHolidayThread.ttmLayer)
	}
}

func adsNoIsland() {
	grDx = 0
	grDy = 0

	// r.c. - this used to only clear the background, leaving the
	// background(wave)/clouds/holiday threads exactly as whatever state
	// they were left in by the last ISLAND scene. Confirmed via a
	// disassembly of THEEND.TTM (JOHNNY.ADS tag 1, "The End"): its own
	// script never draws clouds or shoreline waves - so any seen during
	// that scene are these leftover global threads, not part of the
	// scene itself. Explicitly stop all three here so a non-ISLAND FINAL
	// scene always starts from a clean slate, regardless of how it was
	// reached (adsReleaseIsland() above now handles the normal transition,
	// but this is the actual state adsNoIsland() puts us into, so it
	// shouldn't rely on the previous scene having released cleanly).
	ttmBackgroundThread.isRunning = 0
	if ttmCloudsThread.isRunning != 0 {
		ttmCloudsThread.isRunning = 0
		if ttmCloudsThread.ttmLayer != nil {
			grFreeLayer(ttmCloudsThread.ttmLayer)
			ttmCloudsThread.ttmLayer = nil
		}
	}
	if ttmHolidayThread.isRunning != 0 {
		ttmHolidayThread.isRunning = 0
		grFreeLayer(ttmHolidayThread.ttmLayer)
	}

	grInitEmptyBackground()
}

func adsPlayWalk(fromSpot, fromHdg, toSpot, toHdg int) {
	adsAddScene(0, 0, 0)
	grLoadBmp(&ttmSlots[0], 0, "JOHNWALK.BMP")

	// r.c. - was: grDx = islandState.xPos; grDy = islandState.yPos (no
	// LEFT_ISLAND offset). story.go now sets ttmDx/ttmDy to the destination
	// scene's correctly-offset position before calling this function, so we
	// use that instead of recomputing an un-offset value here. Walking
	// toward a LEFT_ISLAND scene (e.g. returning from water on that side of
	// the island) was rendering at the wrong half's X position otherwise.
	grDx = ttmDx
	grDy = ttmDy

	ttmThreads[0].timer = 6
	ttmThreads[0].delay = 6 // 12 ?

	walkInit(fromSpot, fromHdg, toSpot, toHdg)
	ttmThreads[0].delay = uint16(walkAnimate(&ttmThreads[0], ttmBackgroundThread.ttmSlot))

	for ttmThreads[0].delay != 0 {
		// Call each thread which timer reaches 0
		if ttmBackgroundThread.timer == 0 {
			debugPrintln("    ------> Animate bg")
			ttmBackgroundThread.timer = ttmBackgroundThread.delay
			islandAnimate(&ttmBackgroundThread)
		}

		if ttmCloudsThread.timer == 0 {
			debugPrintln("    ------> Animate clouds")
			ttmCloudsThread.timer = ttmCloudsThread.delay
			islandAnimateClouds(&ttmCloudsThread)
		}

		if ttmThreads[0].timer == 0 {
			debugPrintln("    ------> Animate walking")
			walkResult := uint16(walkAnimate(&ttmThreads[0], ttmBackgroundThread.ttmSlot))
			ttmThreads[0].timer = walkResult
			ttmThreads[0].delay = walkResult
		}

		// Refresh display
		grUpdateDisplay(&ttmBackgroundThread, ttmThreads[:], &ttmHolidayThread, &ttmCloudsThread)
		if shouldExitApp {
			return
		}

		// Determine min timer from the two threads
		mini := uint16(300)
		if ttmBackgroundThread.timer < ttmThreads[0].timer {
			mini = ttmBackgroundThread.timer
		} else if ttmCloudsThread.timer < ttmThreads[0].timer {
			mini = ttmCloudsThread.timer
		} else {
			mini = ttmThreads[0].timer
		}

		// Decrease all timers by the shortest one, and wait that amount of time
		ttmBackgroundThread.timer -= mini
		ttmCloudsThread.timer -= mini
		ttmThreads[0].timer -= mini

		debugPrintf(" ******* WAIT: %d ticks *******\n", mini)
		grUpdateDelay = int(mini)
	}

	adsStopScene(0, false)
}
