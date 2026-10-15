package main

import (
	"bytes"
	"strings"
	"testing"
)

func mustPanicContains(t *testing.T, want string, f func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected panic containing %q", want)
		}
		if !strings.Contains(r.(string), want) {
			t.Fatalf("panic = %q, want substring %q", r, want)
		}
	}()
	f()
}

func TestUncompressRLELiteral(t *testing.T) {
	in := []byte{3, 'A', 'B', 'C'}
	got := uncompressRLE(bytes.NewReader(in), uint32(len(in)), 3)
	if string(got) != "ABC" {
		t.Fatalf("got %q, want %q", got, "ABC")
	}
}

func TestUncompressRLERepeated(t *testing.T) {
	in := []byte{0x84, 'X'}
	got := uncompressRLE(bytes.NewReader(in), uint32(len(in)), 4)
	if string(got) != "XXXX" {
		t.Fatalf("got %q, want %q", got, "XXXX")
	}
}

func TestUncompressRLEMixed(t *testing.T) {
	in := []byte{2, 'A', 'B', 0x83, 'C'}
	got := uncompressRLE(bytes.NewReader(in), uint32(len(in)), 5)
	if string(got) != "ABCCC" {
		t.Fatalf("got %q, want %q", got, "ABCCC")
	}
}

func TestUncompressRLERejectsTruncatedInput(t *testing.T) {
	mustPanicContains(t, "truncated input", func() {
		uncompressRLE(bytes.NewReader([]byte{0x82}), 2, 2)
	})
}

func TestUncompressRLERejectsTruncatedLiteral(t *testing.T) {
	in := []byte{3, 'A', 'B'}
	mustPanicContains(t, "truncated literal run", func() {
		uncompressRLE(bytes.NewReader(in), uint32(len(in)), 3)
	})
}

func TestUncompressRLERejectsRepeatedOutputOverflow(t *testing.T) {
	in := []byte{0x84, 'X'}
	mustPanicContains(t, "output overflow", func() {
		uncompressRLE(bytes.NewReader(in), uint32(len(in)), 3)
	})
}

func TestUncompressRLERejectsLiteralOutputOverflow(t *testing.T) {
	in := []byte{3, 'A', 'B', 'C'}
	mustPanicContains(t, "output overflow", func() {
		uncompressRLE(bytes.NewReader(in), uint32(len(in)), 2)
	})
}

func pack9BitCodes(codes ...uint16) []byte {
	var out []byte
	var acc uint32
	var bits uint
	for _, code := range codes {
		acc |= uint32(code&0x1FF) << bits
		bits += 9
		for bits >= 8 {
			out = append(out, byte(acc))
			acc >>= 8
			bits -= 8
		}
	}
	if bits > 0 {
		out = append(out, byte(acc))
	}
	return out
}

func TestUncompressLZWValidLiteralSequence(t *testing.T) {
	// Codes A, B, C. The decoder stops as soon as the requested output size
	// is filled, so this exercises a valid LZW stream without proprietary assets.
	in := pack9BitCodes('A', 'B', 'C')
	got := uncompressLZW(bytes.NewReader(in), uint32(len(in)), 3)
	if string(got) != "ABC" {
		t.Fatalf("got %q, want %q", got, "ABC")
	}
}

func TestUncompressLZWValidDictionaryReference(t *testing.T) {
	// A, B creates dictionary entry 257 = "AB"; referring to 257 must
	// therefore expand to AB, producing ABAB.
	in := pack9BitCodes('A', 'B', 257)
	got := uncompressLZW(bytes.NewReader(in), uint32(len(in)), 4)
	if string(got) != "ABAB" {
		t.Fatalf("got %q, want %q", got, "ABAB")
	}
}

func TestUncompressLZWRejectsEmptyInput(t *testing.T) {
	mustPanicContains(t, "truncated input", func() {
		uncompressLZW(bytes.NewReader(nil), 0, 1)
	})
}

func TestUncompressLZWRejectsDeclaredInputPastBuffer(t *testing.T) {
	mustPanicContains(t, "truncated input", func() {
		uncompressLZW(bytes.NewReader([]byte{0}), 2, 1)
	})
}

func TestUncompressLZWRejectsZeroOutput(t *testing.T) {
	mustPanicContains(t, "can't uncompress to 0 bytes", func() {
		uncompressLZW(bytes.NewReader([]byte{0}), 1, 0)
	})
}
