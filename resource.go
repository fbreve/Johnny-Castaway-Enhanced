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

func readCompressedSize(buf *bytes.Reader) uint32 {
	size := readUint32(buf)
	if size < 5 {
		panic(fmt.Sprintf("invalid compressed block size: %d", size))
	}
	return size - 5
}

func readVersionString(buf *bytes.Reader, size uint32) string {
	if size < 4 || uint64(size) > uint64(buf.Len()) {
		panic(fmt.Sprintf("invalid version string size: %d", size))
	}
	return string(readUint8Block(buf, int(size))[:4])
}

func readSizedBlock(buf *bytes.Reader, size uint32, name string) *bytes.Reader {
	if uint64(size) > uint64(buf.Len()) {
		panic(fmt.Sprintf("invalid %s size: %d exceeds %d remaining bytes", name, size, buf.Len()))
	}
	return bytes.NewReader(readUint8Block(buf, int(size)))
}

func parseTTMResource(buf *bytes.Reader) TTTMResource {
	ttmResource := TTTMResource{}

	readTag(buf, "VER:")
	ttmResource.VersionSize = readUint32(buf)
	ttmResource.VersionString = readVersionString(buf, ttmResource.VersionSize)

	readTag(buf, "PAG:")
	ttmResource.NumPages = readUint32(buf)
	ttmResource.PagUnknown1 = readUint8(buf)
	ttmResource.PagUnknown2 = readUint8(buf)

	readTag(buf, "TT3:")
	ttmResource.CompressedSize = readCompressedSize(buf)
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
	if ttmResource.TagSize < 2 {
		panic(fmt.Sprintf("invalid TAG block size: %d", ttmResource.TagSize))
	}
	tagBlock := readSizedBlock(buf, ttmResource.TagSize, "TAG block")
	ttmResource.NumTags = readUint16(tagBlock)
	if int(ttmResource.NumTags) > tagBlock.Len()/3 {
		panic(fmt.Sprintf("TAG count %d cannot fit in %d bytes", ttmResource.NumTags, tagBlock.Len()))
	}
	ttmResource.Tags = make([]TTags, ttmResource.NumTags)
	for i := range ttmResource.Tags {
		if tagBlock.Len() < 3 {
			panic(fmt.Sprintf("truncated TAG entry %d", i))
		}
		ttmResource.Tags[i].ID = readUint16(tagBlock)
		ttmResource.Tags[i].Description = getString(tagBlock, 40)
	}

	return ttmResource
}

func parseAdsResource(buf *bytes.Reader) TAdsResource {
	res := TAdsResource{}

	readTag(buf, "VER:")
	res.VersionSize = readUint32(buf)
	res.VersionString = readVersionString(buf, res.VersionSize)
	var err error

	readTag(buf, "ADS:")
	err = binary.Read(buf, binary.LittleEndian, res.AdsUnknown[:])
	if err != nil {
		panic(fmt.Errorf("binary.Read: %w", err))
	}

	readTag(buf, "RES:")
	res.ResSize = readUint32(buf)
	res.NumRes = readUint16(buf)

	if res.ResSize < 2 {
		panic(fmt.Sprintf("invalid ADS RES size: %d", res.ResSize))
	}
	resBlock := readSizedBlock(buf, res.ResSize-2, "ADS RES block")
	if int(res.NumRes) > resBlock.Len()/3 {
		panic(fmt.Sprintf("ADS RES count %d cannot fit in %d bytes", res.NumRes, resBlock.Len()))
	}
	res.Res = make([]TAdsRes, res.NumRes)
	// In original Screen Antics resources, ADS RES entries are stored as
	// (uint16 id + null-terminated name) with variable name length, not fixed
	// 40-byte records. Keep every entry confined to the declared RES block.
	for i := range res.Res {
		if resBlock.Len() < 3 {
			panic(fmt.Sprintf("truncated ADS RES entry %d", i))
		}
		res.Res[i].ID = readUint16(resBlock)
		res.Res[i].Name = getString(resBlock, resBlock.Len())
	}

	readTag(buf, "SCR:")
	res.CompressedSize = readCompressedSize(buf)
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
	if res.TagSize < 2 {
		panic(fmt.Sprintf("invalid TAG block size: %d", res.TagSize))
	}
	tagBlock := readSizedBlock(buf, res.TagSize, "TAG block")
	res.NumTags = readUint16(tagBlock)
	if int(res.NumTags) > tagBlock.Len()/3 {
		panic(fmt.Sprintf("TAG count %d cannot fit in %d bytes", res.NumTags, tagBlock.Len()))
	}
	res.Tags = make([]TTags, res.NumTags)
	for i := range res.Tags {
		if tagBlock.Len() < 3 {
			panic(fmt.Sprintf("truncated TAG entry %d", i))
		}
		res.Tags[i].ID = readUint16(tagBlock)
		res.Tags[i].Description = getString(tagBlock, 40)
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
	res.CompressedSize = readCompressedSize(buf)
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
	res.CompressedSize = readCompressedSize(buf)
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
	if int(mapFile.numEntries) > buf.Len()/8 {
		panic(fmt.Sprintf("invalid resource map entry count: %d exceeds %d available entries", mapFile.numEntries, buf.Len()/8))
	}
	mapFile.entries = make([]TMapFileEntry, int(mapFile.numEntries))
	for i := 0; i < len(mapFile.entries); i++ {
		mapFile.entries[i].length = readUint32(buf)
		mapFile.entries[i].offset = readUint32(buf)
	}

	return &mapFile
}

func resourceEntryData(b []byte, entries []TMapFileEntry, i int) ([]byte, bool) {
	if i < 0 || i >= len(entries) {
		return nil, false
	}
	start := uint64(entries[i].offset)
	end := uint64(len(b))
	if i+1 < len(entries) {
		end = uint64(entries[i+1].offset)
	}
	if start > uint64(len(b)) || end > uint64(len(b)) || end <= start || end-start < 17 {
		return nil, false
	}
	return b[int(start):int(end)], true
}

func parseResourceFile(filename string, mapFile *TMapFile) {
	b := embeddedRes

	for i := 0; i < len(mapFile.entries); i++ {
		seekedBuffer, ok := resourceEntryData(b, mapFile.entries, i)
		if !ok {
			fmt.Printf("Skipping entry %d: invalid resource bounds at offset %d\n", i, mapFile.entries[i].offset)
			continue
		}

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
			if numAdsResources >= len(adsResources) {
				fmt.Printf("Skipping %s: too many ADS resources\n", resName)
				continue
			}
			adsResources[numAdsResources] = parseAdsResource(buf)
			adsResources[numAdsResources].ResName = resName
			numAdsResources += 1
		case ".BMP":
			if numBmpResources >= len(bmpResources) {
				fmt.Printf("Skipping %s: too many BMP resources\n", resName)
				continue
			}
			bmpResources[numBmpResources] = parseBPMResource(buf)
			bmpResources[numBmpResources].ResName = resName
			numBmpResources += 1
		case ".PAL":
			if numPalResources >= len(palResources) {
				fmt.Printf("Skipping %s: too many PAL resources\n", resName)
				continue
			}
			palResources[numPalResources] = parsePalResource(buf)
			palResources[numPalResources].ResName = resName
			numPalResources += 1
		case ".SCR":
			if numScrResources >= len(scrResources) {
				fmt.Printf("Skipping %s: too many SCR resources\n", resName)
				continue
			}
			scrResources[numScrResources] = parseScrResource(buf)
			scrResources[numScrResources].ResName = resName
			numScrResources += 1
		case ".TTM":
			if numTtmResources >= len(ttmResources) {
				fmt.Printf("Skipping %s: too many TTM resources\n", resName)
				continue
			}
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
