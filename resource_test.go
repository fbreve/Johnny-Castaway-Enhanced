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
