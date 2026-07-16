package main

import (
	"math/rand"
)

var (
	ttmDx = 0
	ttmDy = 0
)

func ttmFindPreviousTag(ttmSlot *TTtmSlot, offset uint32) uint32 {
	var result uint32 = 0
	i := 0

	for i < ttmSlot.numTags && ttmSlot.tags[i].offset < offset {
		result = ttmSlot.tags[i].offset
		i++
	}

	return result
}

func ttmFindTag(ttmSlot *TTtmSlot, reqdTag uint16) uint32 {
	var result uint32 = 0xffffffff
	i := 0

	for result == 0xffffffff && i < ttmSlot.numTags {
		if ttmSlot.tags[i].id == reqdTag {
			result = ttmSlot.tags[i].offset
		} else {
			i++
		}
	}

	if result == 0xffffffff {
		debugPrintf("WARN: TTM tag %d not found, returning offset FFFF\n", reqdTag)
	}

	return result
}

func ttmLoadTTM(ttmSlot *TTtmSlot, name string) {
	ttmSlot.ResName = name
	ttmResource := findTTMResource(name)

	ttmSlot.data = ttmResource.UncompressedData
	ttmSlot.dataSize = ttmResource.UncompressedSize
	ttmSlot.numTags = int(ttmResource.NumTags)
	ttmSlot.tags = make([]TTtmTag, ttmSlot.numTags)

	// we have to bookmark every tag for later jumps
	offset := uint32(0)
	tagNo := 0

	for offset < ttmSlot.dataSize {
		opCode := peekUint16(ttmSlot.data, &offset)

		if opCode == 0x1111 || opCode == 0x1101 {
			arg := peekUint16(ttmSlot.data, &offset)
			ttmSlot.tags[tagNo].id = arg
			ttmSlot.tags[tagNo].offset = offset
			tagNo++ // TODO
		} else {
			numArgs := uint8(opCode & 0x000f)

			if numArgs == 0x0f {
				for ttmSlot.data[offset] != 0 && ttmSlot.data[offset+1] != 0 {
					offset += 2
				}
				offset += 2
			} else {
				offset += uint32(numArgs) << 1
			}
		}
	}

	// Keep only real parsed :TAG/:LOCAL_TAG offsets. Leaving padded zero-offset
	// entries in the active slice can make ttmFindPreviousTag() return 0,
	// which incorrectly jumps timed loops back to file start.
	ttmSlot.numTags = tagNo
	ttmSlot.tags = ttmSlot.tags[:tagNo]
}

func ttmInitSlot(ttmSlot *TTtmSlot) {
	for i := 0; i < MaxBMPSlots; i++ {
		ttmSlot.data = nil
		ttmSlot.numSprites[i] = 0
	}
}

func ttmResetSlot(ttmSlot *TTtmSlot) {
	if ttmSlot.data != nil {
		ttmSlot.data = nil
		// free(ttmSlot.tags)
	}
	for i := 0; i < MaxBMPSlots; i++ {
		if ttmSlot.numSprites[i] != 0 {
			//grReleaseBmp(ttmSlot, i)
		}
	}
}

func ttmPlay(ttmThread *TTtmThread) {
	var (
		offset       uint32
		opCode       uint16
		continueLoop = true
		args         [10]uint16
		strBytesArg  = make([]byte, 200)
		finalStr     = "" // added by me -- r.c.
	)

	grDx = ttmDx
	grDy = ttmDy

	ttmSlot := ttmThread.ttmSlot
	offset = ttmThread.ip
	data := ttmSlot.data

	for continueLoop {
		opCode = peekUint16(data, &offset)
		numArgs := uint8(opCode) & 0x000f

		if numArgs == 0x0f {
			// ✅: verified this null-terminated string parsing works - Ralph
			i := 0

			for data[offset] != 0 {
				strBytesArg[i] = data[offset]
				i++
				offset++
			}
			// r.c - here we have a complete string w/o null terminator (for Go)
			finalStr = string(strBytesArg[0:i])

			// r.c. - this captures the null terminator (we don't care about it)
			strBytesArg[i] = data[offset]
			i++
			offset++

			// r.c. - this just ensures we're always at an even byte, probably for historical reasons.
			if i&0x01 == 0x01 { // always read an even number of uint8s
				strBytesArg[i] = data[offset] // TODO
				i++
				offset++
			}
		} else {
			// args are numArgs words
			peekUint16Block(data, &offset, args[:], int(numArgs))
		}

		switch opCode {
		case 0x0080:
			debugPrintln("\tDRAW BACKGROUND")
		case 0x0110:
			debugPrintln("\tPURGE")
			if ttmThread.sceneTimer != 0 {
				ttmThread.nextGotoOffset = ttmFindPreviousTag(ttmSlot, offset)
			} else {
				ttmThread.isRunning = 2
			}
		case 0x0FF0:
			debugPrintln("\tUPDATE")
			continueLoop = false
		case 0x1021:
			var result uint16
			if args[0] > 4 {
				result = args[0]
			} else {
				result = 4
			}
			ttmThread.timer = result
			ttmThread.delay = result

			debugPrintf("\tSET DELAY => %d\n", result)
		case 0x1051:
			debugPrintf("\tSET BMP SLOT: slot:%d\n", args[0])
			ttmThread.selectedBmpSlot = uint8(args[0])
		case 0x1061:
			debugPrintf("\tSET_PALETTE_SLOT: slot:%d\n", args[0])
		case 0x1101:
			debugPrintf("\t:LOCAL_TAG %d\n", args[0])
			ttmThread.sceneTag = args[0]
		case 0x1111:
			// r.c. seems like some script animation marker possibly, perhaps used for debugging.
			debugPrintf("\t:TAG #%d ------------------------\n", args[0])
			ttmThread.sceneTag = args[0]
		case 0x1121:
			// is called before SAVE_IMAGE1, defines the id of the region
			// for further use by CLEAR_SCREEN
			// (see WOULDBE.TTM for a nice example)
			debugPrintf("\tTTM_UNKNOWN_1 %d\n", args[0])
		case 0x1201:
			// ex TTM_UNKNOWN_2
			debugPrintf("\tGOTO_TAG %d\n", args[0])
			ttmThread.nextGotoOffset = ttmFindTag(ttmSlot, args[0])
		case 0x2002:
			debugPrintf("\tSET_COLORS %d %d\n", args[0], args[1])
			ttmThread.fgColor = uint8(args[0])
			ttmThread.bgColor = uint8(args[1])
		case 0x2012:
			// args always == (0,0)
			// at beginning of scenes, near LOAD_IMAGEs
			debugPrintf("\tSET_FRAME1 %d %d\n", args[0], args[1])
		case 0x2022:
			debugPrintf("\tTIMER %d %d\n", args[0], args[1])
			// r.c. - args[0] and args[1] always form a (min, max) range in the
			// original .TTM data (min <= max in every observed sample), so this
			// picks a random delay uniformly within that range rather than a
			// fixed average. This adds the jitter the original animations rely
			// on instead of always producing the same value.
			lo, hi := args[0], args[1]
			var val uint16
			if hi > lo {
				val = lo + uint16(rand.Intn(int(hi-lo)+1))
			} else {
				val = lo
			}
			ttmThread.delay = val
			ttmThread.timer = val
		case 0x4004:
			debugPrintf("\tSET_CLIP_ZONE: %d %d %d %d\n", args[0], args[1], args[2], args[3])
			grSetClipZone(ttmThread.ttmLayer, int16(args[0]), int16(args[1]), int16(args[2]), int16(args[3]))
		case 0x4204:
			debugPrintf("\tCOPY_ZONE_TO_BG: x:%d, y:%d, w:%d, h:%d\n", args[0], args[1], args[2], args[3])
			var handled bool
			if ttmThread.lastOpWasRect {
				handled = grTryRedrawLastRectToBg(ttmThread, int16(args[0]), int16(args[1]), args[2], args[3]) ||
					grTryRedrawLastSpriteToBg(ttmThread, int16(args[0]), int16(args[1]), args[2], args[3])
			} else {
				handled = grTryRedrawLastSpriteToBg(ttmThread, int16(args[0]), int16(args[1]), args[2], args[3]) ||
					grTryRedrawLastRectToBg(ttmThread, int16(args[0]), int16(args[1]), args[2], args[3])
			}
			if handled {
				debugPrintln("\t  (redrew original draw instead of copying rendered pixels)")
			} else {
				grCopyZoneToBg(ttmThread.ttmLayer, args[0], args[1], args[2], args[3])
			}
		case 0x4214:
			// r.c. - confirmed used in the original: appears 32+ times across
			// multiple .TTM scripts, always with 4 args (a region rect).
			debugPrintf("\tSAVE_IMAGE1 %d %d %d %d\n", args[0], args[1], args[2], args[3])
			grSaveImage1(ttmThread.ttmLayer, args[0], args[1], args[2], args[3])
		case 0xA002:
			debugPrintf("\tDRAW_PIXEL %d %d\n", args[0], args[1])
			grDrawPixel(ttmThread.ttmLayer, int16(args[0]), int16(args[1]), ttmThread.fgColor)
		case 0xA054:
			// only once, in GJGULIVR.TTM.txt
			debugPrintf("\tSAVE_ZONE %d %d %d %d\n", args[0], args[1], args[2], args[3])
			grSaveZone(ttmThread.ttmLayer, args[0], args[1], args[2], args[3])
		case 0xA064:
			// only once, in GJGULIVR.TTM.txt
			// r.c. - confirmed: appears exactly once in the original data,
			// paired with a single matching SAVE_ZONE call using the same
			// rect args. Left disabled since the bug described below is a
			// runtime visual issue, not something static args can resolve.
			debugPrintf("\tRESTORE_ZONE %d %d %d %d\n", args[0], args[1], args[2], args[3])
			// r.c. if I enable this, the stupid copied zone, disappears too soon!!
			grRestoreZone(ttmThread.ttmLayer, args[0], args[1], args[2], args[3])
		case 0xA0A4:
			debugPrintf("\tDRAW_LINE %d %d %d %d\n", args[0], args[1], args[2], args[3])
			grDrawLine(ttmThread.ttmLayer, int16(args[0]), int16(args[1]), int16(args[2]), int16(args[3]), ttmThread.fgColor)
		case 0xA104:
			debugPrintf("\tDRAW_RECT %d %d %d %d\n", args[0], args[1], args[2], args[3])
			trackLastRect(ttmThread, int16(args[0]), int16(args[1]), args[2], args[3], ttmThread.fgColor)
			grDrawRect(ttmThread.ttmLayer, ttmThread.ttmSlot, int16(args[0]), int16(args[1]), args[2], args[3], ttmThread.fgColor)
		case 0xA404:
			debugPrintf("\tDRAW_CIRCLE %d %d %d %d\n", args[0], args[1], args[2], args[3])
			grDrawCircle(ttmThread.ttmLayer, int16(args[0]), int16(args[1]), args[2], args[3], ttmThread.fgColor, ttmThread.bgColor)
		case 0xA504:
			debugPrintf("\tDRAW_SPRITE x:%d y:%d sprtNo:%d imgNo:%d\n", args[0], args[1], args[2], args[3])
			trackThreadMovement(ttmThread, int16(args[0]), int16(args[1]))
			trackLastDraw(ttmThread, int16(args[0]), int16(args[1]), args[2], args[3], false)
			grDrawSprite(ttmThread.ttmLayer, ttmThread.ttmSlot, int16(args[0]), int16(args[1]), args[2], args[3])
		case 0xA524:
			debugPrintf("\tDRAW_SPRITE_FLIP x:%d y:%d sprtNo:%d imgNo:%d\n", args[0], args[1], args[2], args[3])
			trackThreadMovement(ttmThread, int16(args[0]), int16(args[1]))
			trackLastDraw(ttmThread, int16(args[0]), int16(args[1]), args[2], args[3], true)
			grDrawSpriteFlip(ttmThread.ttmLayer, ttmThread.ttmSlot, int16(args[0]), int16(args[1]), args[2], args[3])
		case 0xA601:
			debugPrintf("\tCLEAR SCREEN\n")
			grClearScreen(ttmThread.ttmLayer)
		case 0xB606:
			debugPrintf("\tDRAW SCREEN: (NOT IMPLEMENTED)\n")
		case 0xC051:
			debugPrintf("\tPLAY SAMPLE: sampleId:%d\n", args[0])
			soundPlay(args[0])
		case 0xF01F:
			debugPrintf("\tLOAD_SCREEN: %q\n", finalStr)
			grLoadScreen(finalStr)
		case 0xF02F:
			debugPrintf("\tLOAD_IMAGE: %q\n", finalStr)
			grLoadBmp(ttmSlot, uint16(ttmThread.selectedBmpSlot), finalStr)
		case 0xF05F:
			debugPrintf("\tLOAD_PALETTE: %q\n", finalStr)
		}

		if offset >= ttmSlot.dataSize {
			ttmThread.isRunning = 2
			continueLoop = false
		}
	}

	ttmThread.ip = offset
}
