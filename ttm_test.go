package main

import (
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func TestScanTTMTags(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		maxTags int
		wantID  uint16
		wantErr bool
	}{
		{name: "valid tag", data: []byte{0x11, 0x11, 0x34, 0x12}, maxTags: 1, wantID: 0x1234},
		{name: "truncated opcode", data: []byte{0x11}, maxTags: 1, wantErr: true},
		{name: "truncated tag", data: []byte{0x11, 0x11, 0x34}, maxTags: 1, wantErr: true},
		{name: "unterminated string", data: []byte{0x0f, 0x00, 'A', 'B'}, maxTags: 0, wantErr: true},
		{name: "truncated arguments", data: []byte{0x02, 0x00, 0x01, 0x00}, maxTags: 0, wantErr: true},
		{name: "too many tags", data: []byte{0x11, 0x11, 0x01, 0x00}, maxTags: 0, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tags, err := scanTTMTags(tt.data, uint32(len(tt.data)), tt.maxTags)
			if (err != nil) != tt.wantErr {
				t.Fatalf("scanTTMTags() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if len(tags) != 1 || tags[0].id != tt.wantID || tags[0].offset != 4 {
				t.Fatalf("scanTTMTags() tags = %#v", tags)
			}
		})
	}
}

func TestScanTTMTagsPreservesSingleZeroStringTermination(t *testing.T) {
	// The original scanner stops a string opcode when either byte of the
	// current 16-bit pair is zero. The following tag must therefore start
	// immediately after the pair containing 'A', 0.
	data := []byte{0x0f, 0xf0, 'A', 0, 0x11, 0x11, 0x34, 0x12}
	tags, err := scanTTMTags(data, uint32(len(data)), 1)
	if err != nil {
		t.Fatalf("scanTTMTags() error = %v", err)
	}
	if len(tags) != 1 || tags[0].id != 0x1234 || tags[0].offset != 8 {
		t.Fatalf("scanTTMTags() tags = %#v", tags)
	}
}

func TestScanTTMTagsRejectsOversizedDeclaration(t *testing.T) {
	if _, err := scanTTMTags([]byte{0, 0}, 3, 0); err == nil {
		t.Fatal("scanTTMTags() accepted dataSize larger than buffer")
	}
}


func TestDecodeTTMInstruction(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantOp  uint16
		wantStr string
		wantArg uint16
		wantErr bool
	}{
		{name: "word argument", data: []byte{0x01, 0x12, 0x34, 0x12}, wantOp: 0x1201, wantArg: 0x1234},
		{name: "string", data: []byte{0x0f, 0xf0, 'A', 0, 0, 0}, wantOp: 0xf00f, wantStr: "A"},
		{name: "truncated opcode", data: []byte{0x01}, wantErr: true},
		{name: "truncated arguments", data: []byte{0x02, 0x12, 0, 0}, wantErr: true},
		{name: "unterminated string", data: []byte{0x0f, 0xf0, 'A'}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var args [10]uint16
			var offset uint32
			op, str, err := decodeTTMInstruction(tt.data, uint32(len(tt.data)), &offset, &args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("decodeTTMInstruction() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if op != tt.wantOp || str != tt.wantStr || args[0] != tt.wantArg {
				t.Fatalf("decodeTTMInstruction() = op %#x str %q arg %#x", op, str, args[0])
			}
		})
	}
}


func TestDecodeTTMInstructionRejectsTooManyArgs(t *testing.T) {
	data := make([]byte, 2+11*2)
	data[0] = 0x0b
	var offset uint32
	var args [10]uint16
	if _, _, err := decodeTTMInstruction(data, uint32(len(data)), &offset, &args); err == nil {
		t.Fatal("expected oversized argument list to be rejected")
	}
}


func TestValidSpriteIndexBounds(t *testing.T) {
	var slot TTtmSlot
	var sprite0, sprite1 rl.Texture2D
	slot.numSprites[0] = 2
	slot.sprites[0][0] = &sprite0
	slot.sprites[0][1] = &sprite1

	if !validSpriteIndex(&slot, 1, 0) {
		t.Fatal("valid sprite index rejected")
	}
	slot.sprites[0][1] = nil
	if validSpriteIndex(&slot, 1, 0) {
		t.Fatal("nil sprite entry accepted")
	}
	if validSpriteIndex(&slot, 2, 0) {
		t.Fatal("sprite index at count boundary accepted")
	}
	if validSpriteIndex(&slot, 0, MaxBMPSlots) {
		t.Fatal("out-of-range image slot accepted")
	}
	if validSpriteIndex(nil, 0, 0) {
		t.Fatal("nil TTM slot accepted")
	}
}



func TestReleaseBmpToleratesNilSpriteEntries(t *testing.T) {
	var slot TTtmSlot
	slot.numSprites[0] = 1
	slot.slotBmpNames[0] = "BROKEN.BMP"

	// Cleanup must tolerate a partially populated slot without dereferencing nil.
	grReleaseBmp(&slot, 0)

	if slot.numSprites[0] != 0 {
		t.Fatalf("sprite count after release = %d, want 0", slot.numSprites[0])
	}
	if slot.slotBmpNames[0] != "" {
		t.Fatalf("slot name after release = %q, want empty", slot.slotBmpNames[0])
	}
}

func TestBmpSlotBounds(t *testing.T) {
	var slot TTtmSlot

	// Graphics helpers must reject script-controlled invalid slots without
	// touching fixed-size BMP arrays or requiring a graphics context.
	grLoadBmp(&slot, MaxBMPSlots, "INVALID.BMP")
	grReleaseBmp(&slot, MaxBMPSlots)
	grLoadBmp(nil, 0, "INVALID.BMP")
	grReleaseBmp(nil, 0)
}


func TestValidateBmpFrames(t *testing.T) {
	tests := []struct {
		name string
		res  TBMPResource
		wantErr bool
	}{
		{"valid frames", TBMPResource{NumImages: 2, Widths: []uint16{4, 2}, Heights: []uint16{1, 2}, UncompressedData: make([]byte, 4)}, false},
		{"odd width", TBMPResource{NumImages: 1, Widths: []uint16{3}, Heights: []uint16{1}, UncompressedData: make([]byte, 2)}, true},
		{"truncated later frame", TBMPResource{NumImages: 2, Widths: []uint16{4, 4}, Heights: []uint16{1, 1}, UncompressedData: make([]byte, 3)}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBmpFrames(&tt.res)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateBmpFrames() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestScreenPixelDataRequirements(t *testing.T) {
	tests := []struct {
		width, height int
		wantBytes     int
	}{
		{4, 1, 2},
		{640, 480, 153600},
	}
	for _, tt := range tests {
		got := (tt.width / 2) * tt.height
		if got != tt.wantBytes {
			t.Fatalf("%dx%d requires %d bytes, want %d", tt.width, tt.height, got, tt.wantBytes)
		}
	}
}

func TestTTMTimerDelayAlwaysMakesProgress(t *testing.T) {
	for _, tt := range []struct {
		lo, hi uint16
	}{
		{0, 0},
		{0, 3},
		{1, 1},
		{3, 3},
		{4, 4},
		{4, 8},
	} {
		for i := 0; i < 100; i++ {
			got := ttmTimerDelay(tt.lo, tt.hi)
			if got < 4 {
				t.Fatalf("ttmTimerDelay(%d, %d) = %d, want at least 4", tt.lo, tt.hi, got)
			}
			if tt.lo >= 4 && got < tt.lo {
				t.Fatalf("ttmTimerDelay(%d, %d) = %d, below lower bound", tt.lo, tt.hi, got)
			}
			if tt.hi >= tt.lo && tt.hi >= 4 && got > tt.hi {
				t.Fatalf("ttmTimerDelay(%d, %d) = %d, above upper bound", tt.lo, tt.hi, got)
			}
		}
	}
}
