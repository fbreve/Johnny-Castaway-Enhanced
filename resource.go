package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"
)

const (
	MaxADSResources = 100
	MaxBMPResources = 200
	MaxPALResources = 1
	MaxSCRResources = 20
	MaxTTMResources = 100
)

var (
	numAdsResources int
	numBmpResources int
	numPalResources int
	numScrResources int
	numTtmResources int

	adsResources = make([]TAdsResource, MaxADSResources)
	bmpResources = make([]TBMPResource, MaxBMPResources)
	palResources = make([]TPALResource, MaxPALResources)
	scrResources = make([]TSCRResource, MaxSCRResources)
	ttmResources = make([]TTTMResource, MaxTTMResources)
)

type TMapFile struct {
	unknown [6]uint8 //ignored

	resFilename string

	numEntries uint16
	entries    []TMapFileEntry
}

type TMapFileEntry struct {
	length  uint32
	offset  uint32
	resName string
	resSize uint32
}

type TAdsResource struct {
	ResName           string
	VersionSize       uint32
	VersionString     string
	AdsUnknown        [4]uint8
	ResSize           uint32
	NumRes            uint16
	Res               []TAdsRes
	CompressedSize    uint32
	CompressionMethod uint8
	UncompressedSize  uint32
	UncompressedData  []uint8
	TagSize           uint32
	NumTags           uint16
	Tags              []TTags
}

type TAdsRes struct {
	ID   uint16
	Name string
}

type TBMPResource struct {
	ResName           string
	Width             uint16
	Height            uint16
	DataSize          uint32
	NumImages         uint16
	Widths            []uint16
	Heights           []uint16
	CompressedSize    uint32
	CompressionMethod uint8
	UncompressedSize  uint32
	UncompressedData  []uint8
}

type TColor struct {
	R uint8
	G uint8
	B uint8
}

type TPALResource struct {
	ResName  string
	Size     uint16
	Unknown1 uint8
	Unknown2 uint8
	Colors   [256]TColor
}

type TSCRResource struct {
	ResName           string
	TotalSize         uint16
	Flags             uint16
	DimSize           uint32
	Width             uint16
	Height            uint16
	CompressedSize    uint32
	CompressionMethod uint8
	UncompressedSize  uint32
	UncompressedData  []uint8
}

type TTTMResource struct {
	ResName           string
	VersionSize       uint32
	VersionString     string
	NumPages          uint32
	PagUnknown1       uint8
	PagUnknown2       uint8
	CompressedSize    uint32
	CompressionMethod uint8
	UncompressedSize  uint32
	UncompressedData  []uint8
	TtiUnknown1       uint8
	TtiUnknown2       uint8
	TtiUnknown3       uint8
	TtiUnknown4       uint8
	TagSize           uint32
	NumTags           uint16
	Tags              []TTags
}

type TTags struct {
	ID          uint16
	Description string
}

func parseTTMResource(buf *bytes.Reader) TTTMResource {
	ttmResource := TTTMResource{}

	readTag(buf, "VER:")
	ttmResource.VersionSize = readUint32(buf)
	ttmResource.VersionString = string(readUint8Block(buf, int(ttmResource.VersionSize))[0:4])

	readTag(buf, "PAG:")
	ttmResource.NumPages = readUint32(buf)
	ttmResource.PagUnknown1 = readUint8(buf)
	ttmResource.PagUnknown2 = readUint8(buf)

	readTag(buf, "TT3:")
	ttmResource.CompressedSize = readUint32(buf) - 5
	ttmResource.CompressionMethod = readUint8(buf)
	ttmResource.UncompressedSize = readUint32(buf)
	ttmResource.UncompressedData = uncompress(
		buf,
		ttmResource.CompressionMethod,
		ttmResource.CompressedSize,
		ttmResource.UncompressedSize,
	)

	readTag(buf, "TTI:")
	ttmResource.TtiUnknown1 = readUint8(buf)
	ttmResource.TtiUnknown2 = readUint8(buf)
	ttmResource.TtiUnknown3 = readUint8(buf)
	ttmResource.TtiUnknown4 = readUint8(buf)

	readTag(buf, "TAG:")
	ttmResource.TagSize = readUint32(buf)
	ttmResource.NumTags = readUint16(buf)

	ttmResource.Tags = make([]TTags, ttmResource.NumTags)
	for i := 0; i < len(ttmResource.Tags); i++ {
		ttmResource.Tags[i].ID = readUint16(buf)
		ttmResource.Tags[i].Description = getString(buf, 40)
	}

	return ttmResource
}

func parseAdsResource(buf *bytes.Reader) TAdsResource {
	res := TAdsResource{}

	readTag(buf, "VER:")
	res.VersionSize = readUint32(buf)
	tempVersionString := make([]uint8, res.VersionSize)
	err := binary.Read(buf, binary.LittleEndian, tempVersionString)
	if err != nil {
		panic(fmt.Errorf("binary.Read: %w", err))
	}
	res.VersionString = string(tempVersionString[0:4])

	readTag(buf, "ADS:")
	err = binary.Read(buf, binary.LittleEndian, res.AdsUnknown[:])
	if err != nil {
		panic(fmt.Errorf("binary.Read: %w", err))
	}

	readTag(buf, "RES:")
	res.ResSize = readUint32(buf)
	res.NumRes = readUint16(buf)

	res.Res = make([]TAdsRes, res.NumRes)
	// In original Screen Antics resources, ADS RES entries are stored as
	// (uint16 id + null-terminated name) with variable name length, not fixed
	// 40-byte records. Reading fixed-width names misaligns the stream and causes
	// wrong slot->TTM mappings for some ADS scripts (notably JOHNNY.ADS).
	resBytesRead := uint32(0)
	for i := 0; i < int(res.NumRes) && resBytesRead < res.ResSize-2; i++ {
		res.Res[i].ID = readUint16(buf)
		resBytesRead += 2

		nameBytes := make([]byte, 0, 32)
		for {
			b, err := buf.ReadByte()
			if err != nil {
				break
			}
			resBytesRead++
			if b == 0 {
				break
			}
			nameBytes = append(nameBytes, b)
		}
		res.Res[i].Name = string(nameBytes)
	}

	readTag(buf, "SCR:")
	res.CompressedSize = readUint32(buf) - 5
	res.CompressionMethod = readUint8(buf)
	res.UncompressedSize = readUint32(buf)
	res.UncompressedData = uncompress(
		buf,
		res.CompressionMethod,
		res.CompressedSize,
		res.UncompressedSize,
	)

	readTag(buf, "TAG:")
	res.TagSize = readUint32(buf)
	res.NumTags = readUint16(buf)

	res.Tags = make([]TTags, res.NumTags)
	for i := 0; i < int(res.NumTags); i++ {
		res.Tags[i].ID = readUint16(buf)
		res.Tags[i].Description = getString(buf, 40)
	}

	return res
}

func parseBPMResource(buf *bytes.Reader) TBMPResource {
	res := TBMPResource{}

	readTag(buf, "BMP:")
	res.Width = readUint16(buf)
	res.Height = readUint16(buf)

	readTag(buf, "INF:")
	res.DataSize = readUint32(buf)
	res.NumImages = readUint16(buf)

	res.Widths = readUint16Block(buf, int(res.NumImages))
	res.Heights = readUint16Block(buf, int(res.NumImages))

	readTag(buf, "BIN:")
	res.CompressedSize = readUint32(buf) - 5
	res.CompressionMethod = readUint8(buf)
	res.UncompressedSize = readUint32(buf)

	res.UncompressedData = uncompress(
		buf,
		res.CompressionMethod,
		res.CompressedSize,
		res.UncompressedSize,
	)

	return res
}

func parsePalResource(buf *bytes.Reader) TPALResource {
	res := TPALResource{}

	readTag(buf, "PAL:")
	res.Size = readUint16(buf)
	res.Unknown1 = readUint8(buf)
	res.Unknown2 = readUint8(buf)

	readTag(buf, "VGA:")

	// skipped i guess
	_ = readUint8(buf)
	_ = readUint8(buf)
	_ = readUint8(buf)
	_ = readUint8(buf)

	for i := 0; i < 256; i++ {
		res.Colors[i].R = readUint8(buf)
		res.Colors[i].G = readUint8(buf)
		res.Colors[i].B = readUint8(buf)
	}

	return res
}

func parseScrResource(buf *bytes.Reader) TSCRResource {
	res := TSCRResource{}

	readTag(buf, "SCR:")
	res.TotalSize = readUint16(buf)
	res.Flags = readUint16(buf)

	readTag(buf, "DIM:")
	res.DimSize = readUint32(buf)
	res.Width = readUint16(buf)
	res.Height = readUint16(buf)

	readTag(buf, "BIN:")
	res.CompressedSize = readUint32(buf) - 5
	res.CompressionMethod = readUint8(buf)
	res.UncompressedSize = readUint32(buf)

	res.UncompressedData = uncompress(
		buf,
		res.CompressionMethod,
		res.CompressedSize,
		res.UncompressedSize,
	)

	return res
}

func parseMapFile(filename string) *TMapFile {
	b := embeddedMap

	mapFile := TMapFile{}

	buf := bytes.NewReader(b)
	err := binary.Read(buf, binary.LittleEndian, mapFile.unknown[:])
	if err != nil {
		panic(fmt.Errorf("binary.Read: %w", err))
	}

	tempFilename := make([]uint8, 13)
	err = binary.Read(buf, binary.LittleEndian, tempFilename[:])
	if err != nil {
		panic(fmt.Errorf("binary.Read: %w", err))
	}
	mapFile.resFilename = strings.TrimRight(string(tempFilename[0:12]), "\x00 ")

	mapFile.numEntries = readUint16(buf)
	mapFile.entries = make([]TMapFileEntry, int(mapFile.numEntries))
	for i := 0; i < len(mapFile.entries); i++ {
		mapFile.entries[i].length = readUint32(buf)
		mapFile.entries[i].offset = readUint32(buf)
	}

	return &mapFile
}

func parseResourceFile(filename string, mapFile *TMapFile) {
	b := embeddedRes

	for i := 0; i < len(mapFile.entries); i++ {
		if int(mapFile.entries[i].offset)+17 > len(b) {
			fmt.Printf("Skipping entry %d: offset %d out of range\n", i, mapFile.entries[i].offset)
			continue
		}
		seekedBuffer := b[mapFile.entries[i].offset:]

		tempFilename := make([]uint8, 13)
		buf := bytes.NewReader(seekedBuffer)
		err := binary.Read(buf, binary.LittleEndian, &tempFilename)
		if err != nil {
			panic(fmt.Errorf("binary.Read: %w", err))
		}

		idx := bytes.Index(tempFilename, []byte("."))
		if idx == -1 {
			fmt.Printf("Skipping entry %d: no ext in name\n", i)
			continue
		}
		endIdx := idx + 4
		if endIdx > len(tempFilename) {
			endIdx = len(tempFilename)
		}
		mapFile.entries[i].resName = strings.TrimRight(string(tempFilename[0:endIdx]), "\x00")
		err = binary.Read(buf, binary.LittleEndian, &mapFile.entries[i].resSize)
		if err != nil {
			panic(fmt.Errorf("binary.Read: %w", err))
		}

		resName := mapFile.entries[i].resName
		resType := resName[idx:]

		switch resType {
		case ".ADS":
			adsResources[numAdsResources] = parseAdsResource(buf)
			adsResources[numAdsResources].ResName = resName
			numAdsResources += 1
		case ".BMP":
			bmpResources[numBmpResources] = parseBPMResource(buf)
			bmpResources[numBmpResources].ResName = resName
			numBmpResources += 1
		case ".PAL":
			palResources[numPalResources] = parsePalResource(buf)
			palResources[numPalResources].ResName = resName
			numPalResources += 1
		case ".SCR":
			scrResources[numScrResources] = parseScrResource(buf)
			scrResources[numScrResources].ResName = resName
			numScrResources += 1
		case ".TTM":
			ttmResources[numTtmResources] = parseTTMResource(buf)
			ttmResources[numTtmResources].ResName = resName
			numTtmResources += 1
		default:
			fmt.Println("File ignored: ", resName)
		}
	}
}

func parseResourceFiles(filename string) {
	mapFile := parseMapFile(filename)
	resFile := strings.TrimRight(mapFile.resFilename, "\x00 ")
	parseResourceFile(resFile, mapFile)
}

func findAdsResource(name string) *TAdsResource {
	for i := 0; i < numAdsResources; i++ {
		if adsResources[i].ResName == name {
			return &adsResources[i]
		}
	}
	panic("ads resource: " + name + " not found")
}

func findBMPResource(name string) *TBMPResource {
	for i := 0; i < numBmpResources; i++ {
		if bmpResources[i].ResName == name {
			return &bmpResources[i]
		}
	}
	panic("bmp resource: " + name + " not found")
}

func findSCRResource(name string) *TSCRResource {
	for i := 0; i < numScrResources; i++ {
		if scrResources[i].ResName == name {
			return &scrResources[i]
		}
	}
	panic("scr resource: " + name + " not found")
}

func findTTMResource(name string) *TTTMResource {
	for i := 0; i < numTtmResources; i++ {
		if ttmResources[i].ResName == name {
			return &ttmResources[i]
		}
	}
	panic("ttm resource: " + name + " not found")
}
