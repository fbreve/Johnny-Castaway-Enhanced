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
