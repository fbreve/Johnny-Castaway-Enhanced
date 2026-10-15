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
	}
	inOffset++
	b, err := buf.ReadByte()
	if err != nil {
		// Mimic (uint8)fgetc(f) when fgetc returns EOF (-1)
		return 0xFF
	}
	return b
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
	if outSize == 0 {
		panic("uncompressLZW(): can't uncompress to 0 bytes")
	}
	if inSize == 0 || uint64(inSize) > uint64(buf.Len()) {
		panic("uncompressLZW(): truncated input")
	}

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

	push := func(b uint8) {
		if stackPtr >= uint32(len(decodeStack)) {
			panic("uncompressLZW(): decode stack overflow")
		}
		decodeStack[stackPtr] = b
		stackPtr++
	}

	maxInOffset = inSize
	nextBit = 0
	inOffset = 0
	current = getByte(buf)
	tmpBits := getBits(buf, uint32(nBits))
	if tmpBits > 255 {
		panic("uncompressLZW(): invalid initial code")
	}
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

			if uint32(code) > freeEntry {
				panic("uncompressLZW(): invalid code")
			}
			if uint32(code) == freeEntry {
				push(uint8(lastByte))
				code = oldCode
			}

			for code > 255 {
				if code >= uint16(len(codeTable)) || uint32(code) >= freeEntry {
					panic("uncompressLZW(): invalid dictionary reference")
				}
				push(codeTable[code].append)
				code = codeTable[code].prefix
			}

			push(uint8(code))
			lastByte = code

			for stackPtr > 0 {
				stackPtr--
				if outOffset >= outSize {
					return outData
				}
				outData[outOffset] = decodeStack[stackPtr]
				outOffset++
			}

			if freeEntry < uint32(len(codeTable)) {
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

	if uint64(inSize) > uint64(buf.Len()) {
		panic("uncompressRLE(): truncated input")
	}

	inOffset = 0
	maxInOffset = inSize

	readInputByte := func() uint8 {
		if inOffset >= inSize {
			panic("uncompressRLE(): truncated input")
		}
		b, err := buf.ReadByte()
		if err != nil {
			panic("uncompressRLE(): truncated input")
		}
		inOffset++
		return b
	}

	for outOffset < outSize {
		control := readInputByte()

		if (control & 0x80) == 0x80 {
			length := uint32(control & 0x7F)
			b := readInputByte()
			if length > outSize-outOffset {
				panic("uncompressRLE(): output overflow")
			}

			for i := uint32(0); i < length; i++ {
				outData[outOffset] = b
				outOffset++
			}
		} else {
			length := uint32(control)
			if length > inSize-inOffset {
				panic("uncompressRLE(): truncated literal run")
			}
			if length > outSize-outOffset {
				panic("uncompressRLE(): output overflow")
			}

			for i := uint32(0); i < length; i++ {
				outData[outOffset] = readInputByte()
				outOffset++
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
