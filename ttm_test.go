package main

import "testing"

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
		{name: "unterminated string", data: []byte{0x0f, 0x00, 'A', 0x00}, maxTags: 0, wantErr: true},
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
	slot.numSprites[0] = 2

	if !validSpriteIndex(&slot, 1, 0) {
		t.Fatal("valid sprite index rejected")
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


func TestBmpSlotBounds(t *testing.T) {
	var slot TTtmSlot

	// Graphics helpers must reject script-controlled invalid slots without
	// touching fixed-size BMP arrays or requiring a graphics context.
	grLoadBmp(&slot, MaxBMPSlots, "INVALID.BMP")
	grReleaseBmp(&slot, MaxBMPSlots)
	grLoadBmp(nil, 0, "INVALID.BMP")
	grReleaseBmp(nil, 0)
}
