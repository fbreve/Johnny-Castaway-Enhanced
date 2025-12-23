package main

import (
	"fmt"
	"math/rand"
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

		if ttmThread.isRunning == 1 {
			if ttmThread.sceneSlot == ttmSlotNo && ttmThread.sceneTag == ttmTag {
				fmt.Printf("(%d,%d) thread is already running - didn't add extra one\n", ttmSlotNo, ttmTag)
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
		ttmThread.ip = ttmFindTag(&ttmSlots[ttmSlotNo], ttmTag)
	} else {
		ttmThread.ip = 0
	}

	if int16(arg3) < 0 {
		ttmThread.sceneTimer = -int16(arg3)
	} else if int16(arg3) > 0 {
		ttmThread.sceneIterations = arg3 - 1
	}

	ttmThread.ttmLayer = grNewLayer()
	numThreads++
}

func adsStopScene(sceneNo int) {
	grFreeLayer(ttmThreads[sceneNo].ttmLayer)
	ttmThreads[sceneNo].isRunning = 0
	numThreads--
}

func adsStopSceneByTtmTag(ttmSlotNo, ttmTag uint16) {
	for i := 0; i < MaxTTMThreads; i++ {
		ttmThread := &ttmThreads[i]
		if ttmThread.isRunning != 0 {
			if ttmThread.sceneSlot == ttmSlotNo && ttmThread.sceneTag == ttmTag {
				adsStopScene(i)
			}
		}
	}
}

func isSceneRunning(ttmSlotNo, ttmTag uint16) bool {
	for i := 0; i < MaxTTMThreads; i++ {
		ttmThread := &ttmThreads[i]
		if ttmThread.isRunning == 1 &&
			ttmThread.sceneSlot == ttmSlotNo &&
			ttmThread.sceneTag == ttmTag {
			return true
		}
	}
	return false
}

func adsRandomPickOp() *TAdsRandOp {
	totalWeight := 0
	partialWeight := 0
	res := 0

	for i := 0; i < adsNumRandOps; i++ {
		totalWeight += int(adsRandOps[i].weight)
	}

	a := rand.Int() % totalWeight

	for res := 0; res < adsNumRandOps; res++ {
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
			fmt.Printf("RANDOM: chose ADD_SCENE %d %d...\n", op.slot, op.tag)
		case OP_STOP_SCENE:
			fmt.Printf("RANDOM: chose STOP_SCENE %d %d...\n", op.slot, op.tag)
		default:
			fmt.Println("RANDOM: chose NOP")
		}
	} else {
		fmt.Println("RANDOM: no operation to choose from")
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
		grUpdateDelay = int(ttmThreads[0].delay)
	}

	adsStopScene(0)
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

	for continueLoop > 0 && offset < dataSize {
		opcode = peekUint16(data, &offset)

		switch opcode {

		case 0x1070:
			// Inside an IF_LASTPLAYED chunk, local IF_LASTPLAYED
			// which overrides the global IF_LASTPLAYEDs.
			peekUint16Block(data, &offset, args[:], 2)
			fmt.Println("IF_LASTPLAYED_LOCAL")
			inIfLastplayedLocal = 1
			adsChunksLocal[numAdsChunksLocal].scene.slot = args[0]
			adsChunksLocal[numAdsChunksLocal].scene.tag = args[1]
			adsChunksLocal[numAdsChunksLocal].offset = offset
			numAdsChunksLocal++
		case 0x1330:
			// Always just before a call to ADD_SCENE with same (ttm,tag)
			// references tags which init commands : LOAD_IMAGE LOAD_SCREEN etc.
			//   - one exception: FISHING.ADS tag 3
			//   - seems to be a synonym of "IF_NOT_RUNNING"
			//   - if so, our implementation works fine anyway by ignoring this one...
			peekUint16Block(data, &offset, args[:], 2)
			fmt.Println("IF_UNKNOWN_1 %d %d", args[0], args[1])
		case 0x1350:
			peekUint16Block(data, &offset, args[:], 2)
			fmt.Println("IF_LASTPLAYED %d %d", args[0], args[1])
			if inOrBlock == 0 {
				continueLoop = 0
			}

			inOrBlock = 0
		case 0x1360:
			peekUint16Block(data, &offset, args[:], 2)
			fmt.Println("IF_NOT_RUNNING %d %d", args[0], args[1])
			if isSceneRunning(args[0], args[1]) {
				inSkipBlock = 1
			}
		case 0x1370:
			peekUint16Block(data, &offset, args[:], 2)
			fmt.Println("IF_IS_RUNNING %d %d", args[0], args[1])
			if isSceneRunning(args[0], args[1]) {
				inSkipBlock = 1
			} else {
				inSkipBlock = 0
			}
		case 0x1420:
			fmt.Println("AND")
		case 0x1430:
			fmt.Println("OR")
			inOrBlock = 1
		case 0x1510:
			// PLAY_SCENE : in fact, sort of a 'closing brace' for a
			// statement block (several types possible).
			// TODO : implement that in a cleaner way.
			// For now, works quite well like that though...
			fmt.Println("PLAY_SCENE")
			if inSkipBlock == 1 {
				inSkipBlock = 0
			} else {
				continueLoop = 0
			}
		case 0x1520:
			// Only in ACTIVITY.ADS tag 7, after IF_LASTPLAYED_LOCAL
			peekUint16Block(data, &offset, args[:], 5)
			fmt.Println("ADD_SCENE_LOCAL")
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
			fmt.Printf("ADD_SCENE %d %d %d %d\n", args[0], args[1], args[2], args[3])
			if !(inSkipBlock > 0) { // TODO - TEMPO
				if inRandBlock > 0 {
					adsRandomAddScene(args[0], args[1], args[2], args[3])
				} else {
					adsAddScene(args[0], args[1], args[2])
				}
			}
		case 0x2010:
			peekUint16Block(data, &offset, args[:], 3)
			fmt.Printf("STOP_SCENE %d %d %d", args[0], args[1], args[2])
			if !(inSkipBlock > 0) { // TODO - TEMPO
				if inRandBlock > 0 {
					adsRandomStopSceneByTtmTag(args[0], args[1], args[2])
				} else {
					adsStopSceneByTtmTag(args[0], args[1])
				}
			}
		case 0x3010:
			fmt.Println("RANDOM_START")
			adsRandomStart()
			inRandBlock = 1
		case 0x3020:
			peekUint16Block(data, &offset, args[:], 1)
			fmt.Println("NOP")
			if inRandBlock > 0 {
				adsRandomNop(args[0])
			}
		case 0x30ff:
			fmt.Println("RANDOM_END")
			adsRandomEnd()
			inRandBlock = 0
		case 0x4000:
			peekUint16Block(data, &offset, args[:], 3)
			fmt.Println("UNKNOWN_6") // only in BUILDING.ADS tag 7
		case 0xf010:
			fmt.Println("FADE_OUT")
		case 0xf200:
			peekUint16Block(data, &offset, args[:], 1)
			fmt.Printf("GOSUB_TAG %d\n", args[0]) // ex UNKNOWN_8
			// "quick and dirty" implementation, sufficient for
			// JCastaway : only encountered in STAND.ADS to tag 14
			// which only contains 1 scene
			adsPlayChunk(data, dataSize, adsFindTag(args[0]))
		case 0xffff:
			fmt.Println("END")
			if inSkipBlock > 0 {
				// TODO - no doubt this is q&d
				inSkipBlock = 0
			} else {
				adsStopRequested = 1
			}
		case 0xfff0:
			fmt.Println("END_IF")
		default:
			fmt.Printf(":TAG %d\n", opcode)
		}
	}
}

func adsPlayTriggeredChunks(data []byte, dataSize uint32, ttmSlotNo, ttmTag uint16) {
	// First we deal with the case where a local trigger was declared
	// (only one occurrence of this, in ACTIVITY.ADS tag #7)

	if numAdsChunksLocal != 0 {
		for i := 0; i < numAdsChunksLocal; i++ {
			if adsChunksLocal[i].scene.slot == ttmSlotNo && adsChunksLocal[i].scene.tag == ttmTag {
				adsPlayChunk(data, dataSize, adsChunksLocal[i].offset)
				numAdsChunksLocal--
			}
		}
	} else { // Then, the general case
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
	fmt.Printf("\n\n========== Playing ADS: %s:%d ==========\n", adsResource.ResName, adsTag)

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
	for numThreads > 0 {
		if ttmBackgroundThread.isRunning > 0 && ttmBackgroundThread.timer == 0 {
			fmt.Println("    ------> Animate bg")
			ttmBackgroundThread.timer = ttmBackgroundThread.delay
			islandAnimate(&ttmBackgroundThread)
		}

		if ttmCloudsThread.isRunning > 0 && ttmCloudsThread.timer == 0 {
			fmt.Println("    ------> Animate clouds")
			ttmCloudsThread.timer = ttmCloudsThread.delay
			islandAnimateClouds(&ttmCloudsThread)
		}

		for i := 0; i < MaxTTMThreads; i++ {
			// Call ttmPlay() for each thread which timer reaches 0
			if ttmThreads[i].isRunning > 0 && ttmThreads[i].timer == 0 {
				fmt.Printf("    ------> Thread #%d\n", i)
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

		// Determine min timer through all threads
		mini := uint16(300)

		if ttmBackgroundThread.isRunning > 0 {
			mini = ttmBackgroundThread.timer
		}

		if ttmCloudsThread.isRunning > 0 {
			if ttmCloudsThread.timer < mini {
				mini = ttmCloudsThread.timer
			}
		}

		for i := 0; i < MaxTTMThreads; i++ {
			if ttmThreads[i].isRunning > 0 {
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
			if ttmThreads[i].isRunning > 0 {
				ttmThreads[i].timer -= mini
			}
		}

		fmt.Printf(" ******* WAIT: %d ticks *******\n", mini)
		grUpdateDelay = int(mini)

		// Various threads processes
		for i := 0; i < MaxTTMThreads; i++ {
			if ttmThreads[i].isRunning > 0 && ttmThreads[i].timer == 0 {

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
					if ttmThreads[i].sceneIterations > 0 {
						ttmThreads[i].sceneIterations--
						ttmThreads[i].isRunning = 1
						ttmThreads[i].ip = ttmFindTag(&ttmSlots[ttmThreads[i].sceneSlot], ttmThreads[i].sceneTag)
					} else { // Is there one (or more) IF_LASTPLAYED matching the terminated thread ?
						adsStopScene(i)
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

func adsPlayBench() {

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
	ttmBackgroundThread.isRunning = 3
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
	ttmInitSlot(&ttmCloudsSlot)
	ttmCloudsThread.ttmSlot = &ttmCloudsSlot
	ttmCloudsThread.isRunning = 3
	ttmCloudsThread.delay = 8
	ttmCloudsThread.timer = 0
	if ttmCloudsThread.ttmLayer != nil {
		grFreeLayer(ttmCloudsThread.ttmLayer)
		ttmCloudsThread.ttmLayer = nil
	}
	ttmCloudsThread.ttmLayer = grNewLayer()

	islandAnimateClouds(&ttmCloudsThread)
}

func adsReleaseIsland() {
	ttmBackgroundThread.isRunning = 0
	ttmResetSlot(&ttmBackgroundSlot)

	if ttmHolidayThread.isRunning != 0 {
		ttmHolidayThread.isRunning = 0
		grFreeLayer(ttmHolidayThread.ttmLayer)
	}
}

func adsNoIsland() {
	grDx = 0
	grDy = 0
	grInitEmptyBackground()
}

func adsPlayWalk(fromSpot, fromHdg, toSpot, toHdg int) {
	adsAddScene(0, 0, 0)
	grLoadBmp(&ttmSlots[0], 0, "JOHNWALK.BMP")

	grDx = islandState.xPos
	grDy = islandState.yPos

	ttmThreads[0].timer = 6
	ttmThreads[0].delay = 6 // 12 ?

	walkInit(fromSpot, fromHdg, toSpot, toHdg)
	ttmThreads[0].delay = uint16(walkAnimate(&ttmThreads[0], ttmBackgroundThread.ttmSlot))

	for ttmThreads[0].delay > 0 {
		// Call each thread which timer reaches 0
		if ttmBackgroundThread.timer == 0 {
			fmt.Println("    ------> Animate bg")
			ttmBackgroundThread.timer = ttmBackgroundThread.delay
			islandAnimate(&ttmBackgroundThread)
		}

		if ttmThreads[0].timer == 0 {
			fmt.Println("    ------> Animate walking")
			walkResult := uint16(walkAnimate(&ttmThreads[0], ttmBackgroundThread.ttmSlot))
			ttmThreads[0].timer = walkResult
			ttmThreads[0].delay = walkResult
		}

		// Refresh display
		grUpdateDisplay(&ttmBackgroundThread, ttmThreads[:], &ttmHolidayThread, &ttmCloudsThread)

		// Determine min timer from the two threads
		mini := uint16(300)
		if ttmBackgroundThread.timer < ttmThreads[0].timer {
			mini = ttmBackgroundThread.timer
		} else {
			mini = ttmThreads[0].timer
		}

		// Decrease all timers by the shortest one, and wait that amount of time
		ttmBackgroundThread.timer -= mini
		ttmThreads[0].timer -= mini

		fmt.Printf(" ******* WAIT: %d ticks *******\n", mini)
		grUpdateDelay = int(mini)
	}

	adsStopScene(0)
}
