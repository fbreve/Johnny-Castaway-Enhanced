package main

import "testing"

func TestReadADSWordsBounds(t *testing.T) {
	var dest [2]uint16
	var offset uint32
	data := []byte{0x34, 0x12, 0x78, 0x56}
	if !readADSWords(data, uint32(len(data)), &offset, dest[:], 2) {
		t.Fatal("readADSWords rejected valid data")
	}
	if dest != [2]uint16{0x1234, 0x5678} || offset != 4 {
		t.Fatalf("readADSWords = %#v offset %d", dest, offset)
	}

	offset = 0
	if readADSWords(data[:3], 3, &offset, dest[:], 2) {
		t.Fatal("readADSWords accepted truncated words")
	}
	offset = 0
	if readADSWords(data, 5, &offset, dest[:], 1) {
		t.Fatal("readADSWords accepted dataSize larger than buffer")
	}
}

func TestAdsLoadHandlesTruncatedData(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		tags uint16
	}{
		{name: "truncated opcode", data: []byte{0x01}, tags: 0},
		{name: "truncated arguments", data: []byte{0x50, 0x13, 0x01, 0x00}, tags: 0},
		{name: "too many tags", data: []byte{0x01, 0x00}, tags: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var offset uint32
			adsLoad(tt.data, uint32(len(tt.data)), tt.tags, 99, &offset)
		})
	}
}

func TestAdsLoadBookmarksTag(t *testing.T) {
	data := []byte{
		0x07, 0x00,       // tag 7
		0x20, 0x14,       // AND, no arguments
		0xff, 0xff,       // END
	}
	var offset uint32
	adsLoad(data, uint32(len(data)), 1, 7, &offset)
	if offset != 2 {
		t.Fatalf("tag offset = %d, want 2", offset)
	}
	if adsNumTags != 1 || len(adsTags) != 1 || adsTags[0].id != 7 {
		t.Fatalf("tags = %#v, adsNumTags = %d", adsTags, adsNumTags)
	}
}


func TestDecodeADSInstructionBounds(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantOp  uint16
		wantArg uint16
		wantErr bool
	}{
		{name: "valid", data: []byte{0x20, 0x30, 0x34, 0x12}, wantOp: 0x3020, wantArg: 0x1234},
		{name: "truncated opcode", data: []byte{0x20}, wantErr: true},
		{name: "truncated arguments", data: []byte{0x50, 0x13, 0x01, 0x00}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var offset uint32
			var args [10]uint16
			op, err := decodeADSInstruction(tt.data, uint32(len(tt.data)), &offset, &args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && (op != tt.wantOp || args[0] != tt.wantArg) {
				t.Fatalf("op = %04x arg = %04x", op, args[0])
			}
		})
	}
}

func TestAdsLookupTagMissing(t *testing.T) {
	adsTags = []TTtmTag{{id: 7, offset: 2}}
	adsNumTags = 1
	if offset, ok := adsLookupTag(99); ok || offset != 0 {
		t.Fatalf("missing tag lookup = (%d, %v), want (0, false)", offset, ok)
	}
	if offset, ok := adsLookupTag(7); !ok || offset != 2 {
		t.Fatalf("existing tag lookup = (%d, %v), want (2, true)", offset, ok)
	}
}


func TestAdsRandomOpsCapacity(t *testing.T) {
	adsRandomStart()
	for i := 0; i < len(adsRandOps)+2; i++ {
		adsRandomNop(1)
	}
	if adsNumRandOps != len(adsRandOps) {
		t.Fatalf("adsNumRandOps = %d, want %d", adsNumRandOps, len(adsRandOps))
	}
}

func TestAdsRandomPickOpZeroWeight(t *testing.T) {
	adsRandomStart()
	adsRandomNop(0)
	if op := adsRandomPickOp(); op != nil {
		t.Fatal("zero-weight random block unexpectedly returned an operation")
	}
}
