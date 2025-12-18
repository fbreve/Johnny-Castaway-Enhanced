package main

import "bytes"

var (
	nextBit     int
	current     uint8
	inOffset    uint32
	maxInOffset uint32
)

type TCodeTableEntry struct {
	prefix uint16
	append uint8
}

func getByte(buf *bytes.Reader) uint8 {
	if inOffset >= maxInOffset {
		return 0
	} else {
		inOffset++
		b, err := buf.ReadByte()
		if err != nil {
			// Mimic (uint8)fgetc(f) when fgetc returns EOF (-1)
			return 0xFF
		}
		return b
	}
}

func getBits(buf *bytes.Reader, n uint32) uint16 {
	if n == 0 {
		return 0
	}

	x := uint32(0)
	for i := uint32(0); i < n; i++ {
		if (current & (1 << nextBit)) != 0 {
			x |= uint32(1) << i
		}
		nextBit++

		if nextBit > 7 {
			current = getByte(buf)
			nextBit = 0
		}
	}

	return uint16(x)
}

func uncompressLZW(buf *bytes.Reader, inSize, outSize uint32) []byte {
	outData := make([]byte, outSize)

	stackPtr := uint32(0)
	nBits := uint8(9)
	freeEntry := uint32(257)

	var (
		decodeStack [4096]uint8
		codeTable   [4096]TCodeTableEntry
		oldCode     uint16
		lastByte    uint16
		bitPos      uint32
		outOffset   uint32
	)

	if outSize == 0 {
		panic("uncompressLZW() : can't uncompress to 0 bytes")
	}

	maxInOffset = inSize
	nextBit = 0
	inOffset = 0
	current = getByte(buf)
	tmpBits := getBits(buf, uint32(nBits))
	lastByte = tmpBits
	oldCode = tmpBits

	outData[outOffset] = uint8(oldCode)
	outOffset++

	for inOffset < inSize {
		newCode := getBits(buf, uint32(nBits))
		bitPos += uint32(nBits)

		if newCode == 256 {
			nBits3 := uint32(nBits << 3)
			nSkip := (nBits3 - ((bitPos - 1) % nBits3)) - 1
			getBits(buf, nSkip)
			nBits = 9
			freeEntry = 256
			bitPos = 0
		} else {
			code := newCode

			if uint32(code) >= freeEntry {
				if stackPtr > 4095 {
					break
				}

				decodeStack[stackPtr] = uint8(lastByte)
				stackPtr++
				code = oldCode
			}

			for code > 255 {
				if code > 4095 {
					break
				}
				decodeStack[stackPtr] = codeTable[code].append // TODO stackPtr >= 4096 ?
				stackPtr++
				code = codeTable[code].prefix
			}

			decodeStack[stackPtr] = uint8(code) // TODO stackPtr >= 4096 ?
			stackPtr++
			lastByte = code

			for stackPtr > 0 {

				stackPtr--

				if outOffset >= outSize {
					return outData
				}

				outData[outOffset] = decodeStack[stackPtr]
				outOffset++
			}

			if freeEntry < 4096 {
				codeTable[freeEntry].prefix = oldCode
				codeTable[freeEntry].append = uint8(lastByte)
				freeEntry++
				temp := uint32(1 << nBits)

				if freeEntry >= temp && nBits < 12 {
					nBits++
					bitPos = 0
				}
			}

			oldCode = newCode
		}
	}

	if inOffset != inSize {
		panic("error while uncompressing LZW")
	}
	return outData
}

func uncompressRLE(buf *bytes.Reader, inSize, outSize uint32) []byte {
	outData := make([]byte, outSize)
	var outOffset uint32

	inOffset = 0
	maxInOffset = inSize

	for outOffset < outSize {
		control := readUint8(buf)
		inOffset++

		if (control & 0x80) == 0x80 {
			length := control & 0x7F
			b := readUint8(buf)
			inOffset++

			for i := 0; i < int(length); i++ {
				outData[outOffset] = b // TODO outOffset > outSize ?
				outOffset++
			}
		} else {
			for i := 0; i < int(control); i++ {
				outData[outOffset] = readUint8(buf) // TODO idem
				outOffset++
				inOffset++
			}
		}
	}

	if inOffset != inSize {
		panic("error while uncompressing RLE")
	}

	return outData
}

func uncompress(buf *bytes.Reader, compressionMethod uint8, inSize, outSize uint32) []byte {
	switch compressionMethod {
	case 1:
		return uncompressRLE(buf, inSize, outSize)
	case 2:
		return uncompressLZW(buf, inSize, outSize)
	default:
		panic("unknown compression method")
	}
}
