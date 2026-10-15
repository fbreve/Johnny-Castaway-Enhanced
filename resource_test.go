package main

import (
	"bytes"
	"testing"
)

func expectPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	fn()
}

func TestReadCompressedSize(t *testing.T) {
	buf := bytes.NewReader([]byte{9, 0, 0, 0})
	if got := readCompressedSize(buf); got != 4 {
		t.Fatalf("readCompressedSize = %d, want 4", got)
	}

	expectPanic(t, func() {
		readCompressedSize(bytes.NewReader([]byte{4, 0, 0, 0}))
	})
}

func TestReadVersionString(t *testing.T) {
	buf := bytes.NewReader([]byte{'1', '.', '0', '0'})
	if got := readVersionString(buf, 4); got != "1.00" {
		t.Fatalf("readVersionString = %q", got)
	}

	expectPanic(t, func() {
		readVersionString(bytes.NewReader([]byte{'1', '.', '0'}), 3)
	})
	expectPanic(t, func() {
		readVersionString(bytes.NewReader([]byte{'1', '.', '0'}), 4)
	})
}


func TestResourceEntryDataBounds(t *testing.T) {
	data := make([]byte, 64)
	entries := []TMapFileEntry{
		{offset: 0},
		{offset: 32},
	}

	first, ok := resourceEntryData(data, entries, 0)
	if !ok || len(first) != 32 {
		t.Fatalf("first entry = len %d, ok %v; want len 32", len(first), ok)
	}
	second, ok := resourceEntryData(data, entries, 1)
	if !ok || len(second) != 32 {
		t.Fatalf("second entry = len %d, ok %v; want len 32", len(second), ok)
	}

	bad := [][]TMapFileEntry{
		{{offset: 65}},
		{{offset: 10}, {offset: 10}},
		{{offset: 20}, {offset: 10}},
		{{offset: 0}, {offset: 16}},
	}
	for i, entries := range bad {
		if _, ok := resourceEntryData(data, entries, 0); ok {
			t.Fatalf("bad case %d unexpectedly accepted", i)
		}
	}
}


func TestReadSizedBlockDoesNotCrossBoundary(t *testing.T) {
	buf := bytes.NewReader([]byte{1, 2, 3, 4, 5})
	block := readSizedBlock(buf, 3, "test")
	if block.Len() != 3 || buf.Len() != 2 {
		t.Fatalf("block len %d, parent len %d; want 3 and 2", block.Len(), buf.Len())
	}
}

func TestReadSizedBlockRejectsOversize(t *testing.T) {
	expectPanic(t, func() {
		readSizedBlock(bytes.NewReader([]byte{1, 2}), 3, "test")
	})
}

func TestADSResNameCannotConsumeNextBlock(t *testing.T) {
	// A declared RES payload containing an ID and two non-NUL name bytes.
	// The following SCR: bytes must remain outside the bounded sub-reader.
	parent := bytes.NewReader([]byte{1, 0, 'A', 'B', 'S', 'C', 'R', ':'})
	block := readSizedBlock(parent, 4, "ADS RES block")
	_ = readUint16(block)
	expectPanic(t, func() {
		getString(block, block.Len())
	})
	if got := string(readUint8Block(parent, 4)); got != "SCR:" {
		t.Fatalf("next block = %q, want SCR:", got)
	}
}



func TestValidateEntryCount(t *testing.T) {
	validateEntryCount(2, 6, 3, "test")
	expectPanic(t, func() {
		validateEntryCount(2, 5, 3, "test")
	})
	expectPanic(t, func() {
		validateEntryCount(1, 8, 0, "test")
	})
}


func TestBMPImageCountMustFitDimensionData(t *testing.T) {
	// parseBPMResource has already consumed BMP:/dimensions and INF:/DataSize
	// when this structural rule is applied: each image requires one uint16
	// width and one uint16 height.
	validateEntryCount(2, 8, 4, "BMP image dimensions")
	expectPanic(t, func() {
		validateEntryCount(3, 8, 4, "BMP image dimensions")
	})
}

func TestBMPImageCountRuntimeCapacity(t *testing.T) {
	if MaxSpritesPerBMP != len((TTtmSlot{}).sprites[0]) {
		t.Fatalf("MaxSpritesPerBMP = %d, sprite capacity = %d", MaxSpritesPerBMP, len((TTtmSlot{}).sprites[0]))
	}
}
