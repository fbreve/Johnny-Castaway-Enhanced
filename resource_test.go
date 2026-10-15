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
