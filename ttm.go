package main

import (
	"fmt"
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

	tags, err := scanTTMTags(ttmSlot.data, ttmSlot.dataSize, ttmSlot.numTags)
	if err != nil {
		panic(fmt.Errorf("TTM resource %s: %w", name, err))
	}
	ttmSlot.tags = tags
	ttmSlot.numTags = len(tags)
}

func scanTTMTags(data []byte, dataSize uint32, maxTags int) ([]TTtmTag, error) {
	if uint64(dataSize) > uint64(len(data)) {
		return nil, fmt.Errorf("declared data size %d exceeds buffer size %d", dataSize, len(data))
	}

	tags := make([]TTtmTag, 0, maxTags)
	offset := uint32(0)
	for offset < dataSize {
		if dataSize-offset < 2 {
			return nil, fmt.Errorf("truncated opcode at offset %d", offset)
		}
		opCode := peekUint16(data, &offset)
		if opCode == 0x1111 || opCode == 0x1101 {
			if dataSize-offset < 2 {
				return nil, fmt.Errorf("truncated tag at offset %d", offset)
			}
			if len(tags) >= maxTags {
				return nil, fmt.Errorf("more tags than declared (%d)", maxTags)
			}
			arg := peekUint16(data, &offset)
			tags = append(tags, TTtmTag{id: arg, offset: offset})
			continue
		}

		numArgs := uint8(opCode & 0x000f)
		if numArgs == 0x0f {
			for {
				if dataSize-offset < 2 {
					return nil, fmt.Errorf("unterminated string at offset %d", offset)
				}
				if data[offset] == 0 && data[offset+1] == 0 {
					offset += 2
					break
				}
				offset += 2
			}
		} else {
			if int(numArgs) > len(args) {
		return 0, "", fmt.Errorf("too many arguments (%d) at offset %d", numArgs, *offset-2)
	}
	argBytes := uint32(numArgs) * 2
			if argBytes > dataSize-offset {
				return nil, fmt.Errorf("truncated arguments at offset %d", offset)
			}
			offset += argBytes
		}
	}
	return tags, nil
}

func ttmInitSlot(ttmSlot *TTtmSlot) {
	for i := 0; i < MaxBMPSlots; i++ {
		ttmSlot.data = nil
		ttmSlot.numSprites[i] = 0
		ttmSlot.baseBmpNames[i] = ""
		ttmSlot.slotBmpNames[i] = ""
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
		ttmSlot.baseBmpNames[i] = ""
		ttmSlot.slotBmpNames[i] = ""
	}
}

// primaryAnchorImageNo returns which imageNo slot should be used as the
// stable widescreen-scale anchor for a given TTM, if any. For WOULDBE.TTM
// that's the boat hull (imageNo 4, BOAT.BMP) - it's present and drawn every
// frame throughout the boat-encounter tags, and unlike the passengers/girl
// (imageNo 2, WOULDBE.BMP, which include swim-splash and idle-wave frames
// that aren't always at a "representative" x), its x is exactly the boat's
// true position. Using it as a fixed reference avoids the anchor silently
// changing to a different sprite frame-to-frame (which happened when the
// anchor was just "whichever screen-spanning sprite is drawn first" -
// confirmed via trace: the boat's OWN raw script x is constant at 134
// throughout WOULDBE.TTM tag 8 and at 165 throughout tag 11, yet with a
// first-sprite-wins anchor its resolved on-screen x wobbled by up to 40px,
// because the first sprite drawn some frames was a passenger/splash sprite
// instead of the boat).
func primaryAnchorImageNo(ttmSlot *TTtmSlot) (uint16, bool) {
	if ttmSlot.ResName == "WOULDBE.TTM" {
		return 4, true
	}
	return 0, false
}

func decodeTTMInstruction(data []byte, dataSize uint32, offset *uint32, args *[10]uint16) (uint16, string, error) {
	if uint64(dataSize) > uint64(len(data)) || *offset > dataSize || dataSize-*offset < 2 {
		return 0, "", fmt.Errorf("truncated opcode at offset %d", *offset)
	}
	opCode := peekUint16(data, offset)
	numArgs := uint8(opCode) & 0x0f
	if numArgs == 0x0f {
		start := *offset
		for *offset < dataSize && data[*offset] != 0 {
			*offset = *offset + 1
		}
		if *offset >= dataSize {
			return 0, "", fmt.Errorf("unterminated string at offset %d", start)
		}
		str := string(data[start:*offset])
		*offset = *offset + 1
		if (*offset-start)&1 == 1 {
			if *offset >= dataSize {
				return 0, "", fmt.Errorf("truncated string padding at offset %d", *offset)
			}
			*offset = *offset + 1
		}
		return opCode, str, nil
	}
	argBytes := uint32(numArgs) * 2
	if argBytes > dataSize-*offset {
		return 0, "", fmt.Errorf("truncated arguments at offset %d", *offset)
	}
	peekUint16Block(data, offset, args[:], int(numArgs))
	return opCode, "", nil
}

// ttmScanForAnchor looks ahead from offset (without executing anything)
// through the upcoming opcodes of the CURRENT frame only, searching for a
// DRAW_SPRITE/DRAW_SPRITE_FLIP using wantImageNo. Stops cleanly at the next
// UPDATE (end of this frame) or at GOTO_TAG (script control flow changes,
// so a linear scan can no longer safely predict what executes next -
// TTM scripts have no other branching, only ADS scripts do). Returns the
// sprite's raw (unscaled) x if found.
func ttmScanForAnchor(ttmSlot *TTtmSlot, startOffset uint32, wantImageNo uint16) (int16, bool) {
	data := ttmSlot.data
	offset := startOffset
	var args [10]uint16

	for offset < ttmSlot.dataSize {
		opCode, _, err := decodeTTMInstruction(data, ttmSlot.dataSize, &offset, &args)
		if err != nil {
			return 0, false
		}
		switch opCode {
		case 0x0FF0: // UPDATE - end of this frame, stop
			return 0, false
		case 0x1201: // GOTO_TAG - control flow changes, can't safely scan further
			return 0, false
		case 0xA504, 0xA524: // DRAW_SPRITE / DRAW_SPRITE_FLIP
			if args[3] == wantImageNo {
				return int16(args[0]), true
			}
		}
	}
	return 0, false
}

func ttmPlay(ttmThread *TTtmThread) {
	var (
		offset       uint32
		opCode       uint16
		continueLoop = true
		args         [10]uint16
		finalStr     = "" // added by me -- r.c.
	)

	grDx = ttmDx
	grDy = ttmDy

	// r.c. - fresh anchor every frame; see the hasScaleOffset field comment
	// on TTtmThread for why. Prefer a stable, known anchor sprite (found via
	// lookahead) over whichever spanning sprite happens to be drawn first.
	ttmThread.hasScaleOffset = false
	if activeConfig.Widescreen {
		if wantImageNo, ok := primaryAnchorImageNo(ttmThread.ttmSlot); ok {
			if anchorX, found := ttmScanForAnchor(ttmThread.ttmSlot, ttmThread.ip, wantImageNo); found {
				scaledX := int16(float32(anchorX) * (float32(virtualWidth) / 640.0))
				ttmThread.scaleOffsetX = scaledX - anchorX
				ttmThread.hasScaleOffset = true
			}
		}
	}

	ttmSlot := ttmThread.ttmSlot
	offset = ttmThread.ip
	data := ttmSlot.data

	for continueLoop {
		if offset >= ttmSlot.dataSize {
			ttmThread.isRunning = 2
			break
		}
		var err error
		opCode, finalStr, err = decodeTTMInstruction(data, ttmSlot.dataSize, &offset, &args)
		if err != nil {
			debugPrintf("WARN: malformed TTM instruction: %v\n", err)
			ttmThread.isRunning = 2
			break
		}
		switch opCode {
		case 0x0080:
			debugPrintln("\tCLEAR_IMGSLOT")
			grRestoreBmpSlot(ttmSlot, uint16(ttmThread.selectedBmpSlot))
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
			resName := "?"
			if ttmThread.ttmSlot != nil {
				resName = ttmThread.ttmSlot.ResName
			}
			debugPrintf("\tSET_CLIP_ZONE [%s tag=%d]: raw=(%d,%d,%d,%d)\n", resName, ttmThread.sceneTag, args[0], args[1], args[2], args[3])
			grSetClipZone(ttmThread.ttmLayer, int16(args[0]), int16(args[1]), int16(args[2]), int16(args[3]))
			if rect, ok := activeClipZones[ttmThread.ttmLayer]; ok {
				debugPrintf("\t  -> final scissor rect: (%.0f,%.0f,%.0f,%.0f) virtualWidth=%d widescreenOffsetX=%d grDx=%d\n", rect.X, rect.Y, rect.Width, rect.Height, virtualWidth, widescreenOffsetX, grDx)
			} else {
				debugPrintf("\t  -> clip zone cleared (full-screen reset)\n")
			}
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

