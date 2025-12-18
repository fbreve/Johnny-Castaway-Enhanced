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
	ttmSlots          [MaxTTMSlots]TTtmSlot

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
		grUpdateDisplay(nil, ttmThreads[:], nil)
		grUpdateDelay = int(ttmThreads[0].delay)
	}

	adsStopScene(0)
	ttmResetSlot(&ttmSlots[0])
}

func adsPlayChunk() {

}

func adsPlayTriggeredChunks() {

}

func adsPlay() {

}

func adsPlayBench() {

}

func adsPlayIntro() {
	grLoadScreen("INTRO.SCR")
	grUpdateDelay = 100
	grUpdateDisplay(nil, ttmThreads[:], nil)
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
	panic("adsPlayWalk (NOT IMPLEMENTED FOO!!!!)")
}
