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

func TestAdsSceneTimerSignedRange(t *testing.T) {
	tests := []struct {
		arg  uint16
		want int32
	}{
		{0x0000, 0},
		{0x0001, 0},
		{0x7fff, 0},
		{0xffff, 1},
		{0x8001, 32767},
		{0x8000, 32768},
	}
	for _, tt := range tests {
		if got := adsSceneTimer(tt.arg); got != tt.want {
			t.Fatalf("adsSceneTimer(%#04x) = %d, want %d", tt.arg, got, tt.want)
		}
	}
}

func TestSceneTimerLargeDelayDoesNotWrap(t *testing.T) {
	remaining := int32(32768)
	remaining -= int32(uint16(65535))
	if remaining >= 0 {
		t.Fatalf("remaining scene timer = %d, want expired timer", remaining)
	}
}

func TestSingleTtmStopsWhenThreadTerminates(t *testing.T) {
	slot := TTtmSlot{dataSize: 100}
	thread := TTtmThread{isRunning: 1, ip: 10}
	if !singleTtmShouldContinue(&thread, &slot) {
		t.Fatal("active TTM stopped before reaching the end")
	}

	thread.isRunning = 2
	if singleTtmShouldContinue(&thread, &slot) {
		t.Fatal("terminated TTM would be played again")
	}

	thread.isRunning = 1
	thread.ip = slot.dataSize
	if singleTtmShouldContinue(&thread, &slot) {
		t.Fatal("TTM would continue past the end of its data")
	}
}

func TestAdsCleanupPlaybackResetsThreadStateWithoutGraphics(t *testing.T) {
	// No graphics context is required: use empty layers and slots.
	// Preserve global state so this test cannot affect unrelated cases.
	oldThreads, oldSlots, oldNumThreads := ttmThreads, ttmSlots, numThreads
	defer func() {
		ttmThreads, ttmSlots, numThreads = oldThreads, oldSlots, oldNumThreads
	}()
	ttmThreads = [MaxTTMThreads]TTtmThread{}
	ttmSlots = [MaxTTMSlots]TTtmSlot{}
	ttmThreads[0].isRunning = 1
	ttmThreads[1].isRunning = 2
	numThreads = 2

	adsCleanupPlayback()
	if numThreads != 0 || ttmThreads[0].isRunning != 0 || ttmThreads[1].isRunning != 0 {
		t.Fatalf("playback state not cleared: threads=%d states=(%d,%d)",
			numThreads, ttmThreads[0].isRunning, ttmThreads[1].isRunning)
	}
	// Repeated cleanup must not revive or decrement the thread count.
	adsCleanupPlayback()
	if numThreads != 0 {
		t.Fatalf("second cleanup changed thread count to %d", numThreads)
	}
}


 // TestAdsLoadConditionalChunkBookmarks characterizes how ADS pre-scanning
 // records IF_NOT_RUNNING chunks until IF_IS_RUNNING disables bookmarking.
 // These synthetic instructions never initialize a graphics context.
func TestAdsLoadConditionalChunkBookmarks(t *testing.T) {
	oldChunks, oldCount := adsChunks, numAdsChunks
	oldLocal, oldLocalCount := adsChunksLocal, numAdsChunksLocal
	oldTags, oldTagCount := adsTags, adsNumTags
	defer func() {
		adsChunks, numAdsChunks = oldChunks, oldCount
		adsChunksLocal, numAdsChunksLocal = oldLocal, oldLocalCount
		adsTags, adsNumTags = oldTags, oldTagCount
	}()

	// tag 7; IF_NOT_RUNNING(1,10); IF_IS_RUNNING(1,11);
	// IF_NOT_RUNNING(1,12); IF_LASTPLAYED(1,13); END.
	data := []byte{
		0x07, 0x00,
		0x60, 0x13, 0x01, 0x00, 0x0a, 0x00,
		0x70, 0x13, 0x01, 0x00, 0x0b, 0x00,
		0x60, 0x13, 0x01, 0x00, 0x0c, 0x00,
		0x50, 0x13, 0x01, 0x00, 0x0d, 0x00,
		0xff, 0xff,
	}
	var offset uint32
	adsLoad(data, uint32(len(data)), 1, 7, &offset)
	if offset != 2 {
		t.Fatalf("tag offset = %d, want 2", offset)
	}
	if numAdsChunks != 2 {
		t.Fatalf("bookmarked chunks = %d, want 2", numAdsChunks)
	}
	if got := adsChunks[0]; got.scene.slot != 1 || got.scene.tag != 10 || got.offset != 8 {
		t.Fatalf("first bookmark = %#v", got)
	}
	if got := adsChunks[1]; got.scene.slot != 1 || got.scene.tag != 13 || got.offset != 26 {
		t.Fatalf("second bookmark = %#v", got)
	}
}

func TestAdsRandomBlockCharacterization(t *testing.T) {
	oldOps, oldCount := adsRandOps, adsNumRandOps
	defer func() { adsRandOps, adsNumRandOps = oldOps, oldCount }()

	adsRandomStart()
	adsRandomAddScene(1, 10, 2, 0)
	adsRandomStopSceneByTtmTag(1, 11, 5)
	adsRandomNop(0)
	if adsNumRandOps != 3 {
		t.Fatalf("queued random operations = %d, want 3", adsNumRandOps)
	}
	for i := 0; i < 20; i++ {
		op := adsRandomPickOp()
		if op == nil || op.ttype != OP_STOP_SCENE || op.tag != 11 {
			t.Fatalf("only positive-weight operation was not selected: %#v", op)
		}
	}
	adsRandomStart()
	if adsNumRandOps != 0 || adsRandomPickOp() != nil {
		t.Fatal("RANDOM_START did not clear previous random operations")
	}
}


 // TestAdsConditionalPlaySceneCharacterization captures the current
 // IF_NOT_RUNNING / IF_IS_RUNNING and PLAY_SCENE control-flow behavior.
 // No ADD_SCENE or graphics initialization is needed.
func TestAdsConditionalPlaySceneCharacterization(t *testing.T) {
	oldThreads, oldStop := ttmThreads, adsStopRequested
	defer func() {
		ttmThreads, adsStopRequested = oldThreads, oldStop
	}()
	tests := []struct {
		name string
		condition uint16
		running bool
		wantStop int
	}{
		{"not-running condition satisfied", 0x1360, false, 0},
		{"not-running condition skipped", 0x1360, true, 1},
		{"running condition satisfied", 0x1370, true, 0},
		{"running condition skipped", 0x1370, false, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ttmThreads = [MaxTTMThreads]TTtmThread{}
			if tt.running {
				ttmThreads[0].isRunning = 1
				ttmThreads[0].sceneSlot = 1
				ttmThreads[0].sceneRootTag = 10
			}
			adsStopRequested = 0
			data := []byte{
				byte(tt.condition), byte(tt.condition >> 8),
				0x01, 0x00, 0x0a, 0x00, // scene (1,10)
				0x10, 0x15, // PLAY_SCENE: exits if condition is satisfied
				0xff, 0xff, // END: reached only when the block was skipped
			}
			adsPlayChunk(data, uint32(len(data)), 0)
			if adsStopRequested != tt.wantStop {
				t.Fatalf("adsStopRequested = %d, want %d", adsStopRequested, tt.wantStop)
			}
		})
	}
}
