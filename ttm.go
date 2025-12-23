package main

import (
	"fmt"
)

var (
	ttmDx = 0
	ttmDy = 0
)

func ttmFindPreviousTag(ttmSlot *TTtmSlot, offset uint32) uint32 {
	var result uint32 = 0
	i := 0

	for ttmSlot.tags[i].offset < offset {
		result = ttmSlot.tags[i].offset
		i++
	}

	return result
}

func ttmFindTag(ttmSlot *TTtmSlot, reqdTag uint16) uint32 {
	var result uint32 = 0
	i := 0

	for result == 0 && i < ttmSlot.numTags {
		if ttmSlot.tags[i].id == reqdTag {
			result = ttmSlot.tags[i].offset
		} else {
			i++
		}
	}

	if result == 0 {
		fmt.Printf("WARN: TTM tag %d not found, returning offset 0000\n", reqdTag)
	}

	return result
}

func ttmLoadTTM(ttmSlot *TTtmSlot, name string) {
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

	// TODO : in SASKDATE.TTM, num SET_SCENE != ttmResource->numTags
	for tagNo < ttmSlot.numTags {
		ttmSlot.tags[tagNo].id = 0 // TODO is this useful ?
		tagNo++
	}
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
			fmt.Println("\tDRAW BACKGROUND")
		case 0x0110:
			fmt.Println("\tPURGE")
			if ttmThread.sceneTimer != 0 {
				ttmThread.nextGotoOffset = ttmFindPreviousTag(ttmSlot, offset)
			} else {
				ttmThread.isRunning = 2
			}
		case 0x0FF0:
			fmt.Println("\tUPDATE")
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

			fmt.Printf("\tSET DELAY => %d\n", result)
		case 0x1051:
			fmt.Printf("\tSET BMP SLOT: slot:%d\n", args[0])
			ttmThread.selectedBmpSlot = uint8(args[0])
		case 0x1061:
			fmt.Printf("\tSET_PALETTE_SLOT: slot:%d\n", args[0])
		case 0x1101:
			fmt.Printf("\t:LOCAL_TAG %d", args[0])
		case 0x1111:
			// r.c. seems like some script animation marker possibly, perhaps used for debugging.
			fmt.Printf("\t:TAG #%d ------------------------\n", args[0])
		case 0x1121:
			// is called before SAVE_IMAGE1, defines the id of the region
			// for further use by CLEAR_SCREEN
			// (see WOULDBE.TTM for a nice example)
			fmt.Printf("\tTTM_UNKNOWN_1 %d\n", args[0])
		case 0x1201:
			// ex TTM_UNKNOWN_2
			fmt.Printf("\tGOTO_TAG %d\n", args[0])
			ttmThread.nextGotoOffset = ttmFindTag(ttmSlot, args[0])
		case 0x2002:
			fmt.Printf("\tSET_COLORS %d %d\n", args[0], args[1])
			ttmThread.fgColor = uint8(args[0])
			ttmThread.bgColor = uint8(args[1])
		case 0x2012:
			// args always == (0,0)
			// at beginning of scenes, near LOAD_IMAGEs
			fmt.Printf("\tSET_FRAME1 %d %d\n", args[0], args[1])
		case 0x2022:
			fmt.Printf("\tTIMER %d %d\n", args[0], args[1])
			// Really, really not sure about this formula... but things
			// do work not so bad like that
			val := (args[0] + args[1]) / 2
			ttmThread.delay = val
			ttmThread.timer = val
		case 0x4004:
			fmt.Printf("\tSET_CLIP_ZONE: %d %d %d %d\n", args[0], args[1], args[2], args[3])
			grSetClipZone(ttmThread.ttmLayer, int16(args[0]), int16(args[1]), int16(args[2]), int16(args[3]))
		case 0x4204:
			fmt.Printf("\tCOPY_ZONE_TO_BG: x:%d, y:%d, w:%d, h:%d\n", args[0], args[1], args[2], args[3])
			grCopyZoneToBg(ttmThread.ttmLayer, args[0], args[1], args[2], args[3])
		case 0x4214:
			// defines the zone to be redrawn at each update ?
			// but seems not used in the original
			fmt.Printf("\tSAVE_IMAGE1 %d %d %d %d\n", args[0], args[1], args[2], args[3])
			//grSaveImage1(ttmThread->ttmLayer, args[0], args[1], args[2], args[3]);
		case 0xA002:
			fmt.Printf("\tDRAW_PIXEL %d %d\n", args[0], args[1])
			grDrawPixel(ttmThread.ttmLayer, int16(args[0]), int16(args[1]), ttmThread.fgColor)
		case 0xA054:
			// only once, in GJGULIVR.TTM.txt
			fmt.Printf("\tSAVE_ZONE %d %d %d %d\n", args[0], args[1], args[2], args[3])
			//grSaveZone(ttmThread->ttmLayer, args[0], args[1], args[2], args[3]);
		case 0xA064:
			// only once, in GJGULIVR.TTM.txt
			fmt.Printf("\tRESTORE_ZONE %d %d %d %d\n", args[0], args[1], args[2], args[3])
			//grRestoreZone(ttmThread->ttmLayer, args[0], args[1], args[2], args[3]);
		case 0xA0A4:
			fmt.Printf("\tDRAW_LINE %d %d %d %d\n", args[0], args[1], args[2], args[3])
			grDrawLine(ttmThread.ttmLayer, int16(args[0]), int16(args[1]), int16(args[2]), int16(args[3]), ttmThread.fgColor)
		case 0xA104:
			fmt.Printf("\tDRAW_RECT %d %d %d %d\n", args[0], args[1], args[2], args[3])
			grDrawRect(ttmThread.ttmLayer, int16(args[0]), int16(args[1]), args[2], args[3], ttmThread.fgColor)
		case 0xA404:
			fmt.Printf("\tDRAW_CIRCLE %d %d %d %d\n", args[0], args[1], args[2], args[3])
			grDrawCircle(ttmThread.ttmLayer, int16(args[0]), int16(args[1]), args[2], args[3], ttmThread.fgColor, ttmThread.bgColor)
		case 0xA504:
			fmt.Printf("\tDRAW_SPRITE x:%d y:%d sprtNo:%d imgNo:%d\n", args[0], args[1], args[2], args[3])
			grDrawSprite(ttmThread.ttmLayer, ttmThread.ttmSlot, int16(args[0]), int16(args[1]), args[2], args[3])
		case 0xA524:
			fmt.Printf("\tDRAW_SPRITE_FLIP x:%d y:%d sprtNo:%d imgNo:%d\n", args[0], args[1], args[2], args[3])
			grDrawSpriteFlip(ttmThread.ttmLayer, ttmThread.ttmSlot, int16(args[0]), int16(args[1]), args[2], args[3])
		case 0xA601:
			fmt.Printf("\tCLEAR SCREEN\n")
			grClearScreen(ttmThread.ttmLayer)
		case 0xB606:
			fmt.Printf("\tDRAW SCREEN: (NOT IMPLEMENTED)\n")
		case 0xC051:
			fmt.Printf("\tPLAY SAMPLE: sampleId:%d (NOT IMPLEMENTED)\n", args[0])
			//soundPlay(args[0]);
		case 0xF01F:
			fmt.Printf("\tLOAD_SCREEN: %q\n", finalStr)
			grLoadScreen(finalStr)
		case 0xF02F:
			fmt.Printf("\tLOAD_IMAGE: %q\n", finalStr)
			grLoadBmp(ttmSlot, uint16(ttmThread.selectedBmpSlot), finalStr)
		case 0xF05F:
			fmt.Printf("\tLOAD_PALETTE: %q\n", finalStr)
		}

		if offset >= ttmSlot.dataSize {
			ttmThread.isRunning = 2
			continueLoop = false
		}
	}

	ttmThread.ip = offset
}
