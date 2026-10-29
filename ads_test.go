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


 // TestAdsConsecutiveConditionsCharacterization records the current
 // handling of sequential IF_NOT_RUNNING and IF_IS_RUNNING instructions.
 // The interpreter uses one skip flag rather than nested condition frames.
func TestAdsConsecutiveConditionsCharacterization(t *testing.T) {
	oldThreads, oldStop := ttmThreads, adsStopRequested
	defer func() { ttmThreads, adsStopRequested = oldThreads, oldStop }()
	tests := []struct {
		name string
		firstRunning bool
		secondRunning bool
		wantStop int
	}{
		{"both conditions satisfied", false, true, 0},
		{"first skipped second satisfied", true, true, 0},
		{"first satisfied second skipped", false, false, 1},
		{"both skipped", true, false, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ttmThreads = [MaxTTMThreads]TTtmThread{}
			if tt.firstRunning {
				ttmThreads[0] = TTtmThread{isRunning: 1, sceneSlot: 1, sceneRootTag: 10}
			}
			if tt.secondRunning {
				ttmThreads[1] = TTtmThread{isRunning: 1, sceneSlot: 1, sceneRootTag: 11}
			}
			adsStopRequested = 0
			data := []byte{
				0x60, 0x13, 0x01, 0x00, 0x0a, 0x00, // IF_NOT_RUNNING(1,10)
				0x70, 0x13, 0x01, 0x00, 0x0b, 0x00, // IF_IS_RUNNING(1,11)
				0x10, 0x15, // PLAY_SCENE
				0xff, 0xff, // END
			}
			adsPlayChunk(data, uint32(len(data)), 0)
			if adsStopRequested != tt.wantStop {
				t.Fatalf("adsStopRequested = %d, want %d", adsStopRequested, tt.wantStop)
			}
		})
	}
}

// TestAdsRandomConditionalCharacterization records that skipped ADD_SCENE
// instructions are not queued, while a RANDOM_END with no queued operations
// leaves playback state unchanged. No scene or graphics resources are created.
func TestAdsRandomConditionalCharacterization(t *testing.T) {
	oldThreads, oldStop := ttmThreads, adsStopRequested
	oldOps, oldCount := adsRandOps, adsNumRandOps
	defer func() {
		ttmThreads, adsStopRequested = oldThreads, oldStop
		adsRandOps, adsNumRandOps = oldOps, oldCount
	}()
	for _, tc := range []struct {
		name string
		condition uint16
		running bool
		wantQueued int
	}{
		{"not-running skipped", 0x1360, true, 0},
		{"running skipped", 0x1370, false, 0},
		{"not-running satisfied", 0x1360, false, 1},
		{"running satisfied", 0x1370, true, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ttmThreads = [MaxTTMThreads]TTtmThread{}
			if tc.running {
				ttmThreads[0] = TTtmThread{isRunning: 1, sceneSlot: 1, sceneRootTag: 10}
			}
			adsStopRequested = 0
			adsRandomStart()
			// The queued operation has weight zero, so RANDOM_END cannot
			// execute it (and thus cannot initialize a render texture).
			data := []byte{
				byte(tc.condition), byte(tc.condition >> 8), 1, 0, 10, 0,
				0x10, 0x30, // RANDOM_START
				0x05, 0x20, 1, 0, 11, 0, 1, 0, 0, 0, // ADD_SCENE(1,11,1,weight=0)
				0xff, 0x30, // RANDOM_END
				0x10, 0x15, // PLAY_SCENE
				0xff, 0xff, // END
			}
			adsPlayChunk(data, uint32(len(data)), 0)
			if adsNumRandOps != tc.wantQueued {
				t.Fatalf("queued operations = %d, want %d", adsNumRandOps, tc.wantQueued)
			}
			if tc.wantQueued == 0 && adsStopRequested != 1 {
				t.Fatalf("skipped conditional should reach END, stop=%d", adsStopRequested)
			}
			if tc.wantQueued == 1 && adsStopRequested != 0 {
				t.Fatalf("satisfied conditional should stop at PLAY_SCENE, stop=%d", adsStopRequested)
			}
		})
	}
}

// TestFishingADSRealORBookmarks uses the opening 86 bytes of tag 1 from
// the original FISHING.ADS SCR stream (RESOURCE.001 / RESOURCE.MAP).
// This is a small extracted instruction fixture, not a synthetic opcode
// sequence. It checks the loader's dispatch bookmarks without rendering.
func TestFishingADSRealORBookmarks(t *testing.T) {
	oldChunks, oldCount := adsChunks, numAdsChunks
	oldLocal, oldLocalCount := adsChunksLocal, numAdsChunksLocal
	oldTags, oldTagCount := adsTags, adsNumTags
	defer func() {
		adsChunks, numAdsChunks = oldChunks, oldCount
		adsChunksLocal, numAdsChunksLocal = oldLocal, oldLocalCount
		adsTags, adsNumTags = oldTags, oldTagCount
	}()
	data := []byte{
		0x01, 0x00, 0x30, 0x13, 0x01, 0x00, 0x01, 0x00,
		0x05, 0x20, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00,
		0x01, 0x00, 0x05, 0x20, 0x01, 0x00, 0x12, 0x00,
		0x00, 0x00, 0x01, 0x00, 0x10, 0x15, 0x50, 0x13,
		0x01, 0x00, 0x12, 0x00, 0x05, 0x20, 0x01, 0x00,
		0x0a, 0x00, 0x00, 0x00, 0x01, 0x00, 0x10, 0x15,
		0x50, 0x13, 0x01, 0x00, 0x0a, 0x00, 0x30, 0x14,
		0x50, 0x13, 0x01, 0x00, 0x15, 0x00, 0x30, 0x14,
		0x50, 0x13, 0x01, 0x00, 0x16, 0x00, 0x30, 0x14,
		0x50, 0x13, 0x01, 0x00, 0x17, 0x00, 0x30, 0x14,
		0x50, 0x13, 0x01, 0x00, 0x26, 0x00,
	}
	var offset uint32
	adsLoad(data, uint32(len(data)), 1, 1, &offset)
	if offset != 2 {
		t.Fatalf("tag 1 offset = %d, want 2", offset)
	}
	want := []struct{ tag uint16; offset uint32 }{
		{18, 36}, {10, 54}, {21, 62}, {22, 70}, {23, 78}, {38, 86},
	}
	if numAdsChunks != len(want) {
		t.Fatalf("bookmarks = %d, want %d", numAdsChunks, len(want))
	}
	for i, w := range want {
		got := adsChunks[i]
		if got.scene.slot != 1 || got.scene.tag != w.tag || got.offset != w.offset {
			t.Errorf("bookmark %d = (%d,%d) at %d, want (1,%d) at %d",
				i, got.scene.slot, got.scene.tag, got.offset, w.tag, w.offset)
		}
	}
}


// TestFishingADSORTriggeredDispatch extends the original FISHING.ADS tag 1
// OR-chain fixture with a terminal END to observe dispatch without rendering.
// The END is a test sentinel, not part of the extracted 86-byte prefix.
func TestFishingADSORTriggeredDispatch(t *testing.T) {
	oldChunks, oldCount := adsChunks, numAdsChunks
	oldLocal, oldLocalCount := adsChunksLocal, numAdsChunksLocal
	oldTags, oldTagCount := adsTags, adsNumTags
	oldStop := adsStopRequested
	defer func() {
		adsChunks, numAdsChunks = oldChunks, oldCount
		adsChunksLocal, numAdsChunksLocal = oldLocal, oldLocalCount
		adsTags, adsNumTags = oldTags, oldTagCount
		adsStopRequested = oldStop
	}()
	// Tag 1 and the five real IF_LASTPLAYED / OR pairs from FISHING.ADS.
	data := []byte{
		1, 0,
		0x50, 0x13, 1, 0, 10, 0, 0x30, 0x14,
		0x50, 0x13, 1, 0, 21, 0, 0x30, 0x14,
		0x50, 0x13, 1, 0, 22, 0, 0x30, 0x14,
		0x50, 0x13, 1, 0, 23, 0, 0x30, 0x14,
		0x50, 0x13, 1, 0, 38, 0,
		0xff, 0xff, // END sentinel for observing dispatch
	}
	var offset uint32
	adsLoad(data, uint32(len(data)), 1, 1, &offset)
	if offset != 2 || numAdsChunks != 5 {
		t.Fatalf("offset=%d bookmarks=%d; want 2 and 5", offset, numAdsChunks)
	}
	for _, tag := range []uint16{10, 21, 22, 23, 38} {
		adsStopRequested = 0
		adsPlayTriggeredChunks(data, uint32(len(data)), 1, tag)
		if adsStopRequested != 1 {
			t.Errorf("completion (1,%d) did not dispatch to END sentinel", tag)
		}
	}
	adsStopRequested = 0
	adsPlayTriggeredChunks(data, uint32(len(data)), 1, 99)
	if adsStopRequested != 0 {
		t.Error("unrelated scene completion unexpectedly dispatched")
	}
}


// TestStandADSRealANDChainCharacterization mirrors the four consecutive
// IF_NOT_RUNNING predicates in STAND.ADS tag 1: (1,1), (1,2), (1,3),
// (1,53). The trailing PLAY_SCENE / END are test-only sentinels.
// A failure of any AND operand must remain sticky through later operands.
func TestStandADSRealANDChainCharacterization(t *testing.T) {
	oldThreads, oldStop := ttmThreads, adsStopRequested
	defer func() { ttmThreads, adsStopRequested = oldThreads, oldStop }()
	data := []byte{
		0x60, 0x13, 1, 0, 1, 0,
		0x20, 0x14,
		0x60, 0x13, 1, 0, 2, 0,
		0x20, 0x14,
		0x60, 0x13, 1, 0, 3, 0,
		0x20, 0x14,
		0x60, 0x13, 1, 0, 53, 0,
		0x10, 0x15, // PLAY_SCENE: terminates if all conditions pass
		0xff, 0xff, // END: reached if any condition fails
	}
	for _, tc := range []struct {
		name string
		running []uint16
		wantStop int
	}{
		{"all absent", nil, 0},
		{"first running", []uint16{1}, 1},
		{"second running", []uint16{2}, 1},
		{"third running", []uint16{3}, 1},
		{"fourth running", []uint16{53}, 1},
		{"multiple running", []uint16{1, 53}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ttmThreads = [MaxTTMThreads]TTtmThread{}
			for i, tag := range tc.running {
				ttmThreads[i] = TTtmThread{isRunning: 1, sceneSlot: 1, sceneRootTag: tag}
			}
			adsStopRequested = 0
			adsPlayChunk(data, uint32(len(data)), 0)
			if adsStopRequested != tc.wantStop {
				t.Fatalf("running=%v: stop=%d, want %d", tc.running, adsStopRequested, tc.wantStop)
			}
		})
	}
}


// TestStandADSRunningConditionBoundaries models the actual STAND.ADS tag 1
// transition from IF_IS_RUNNING(1,53) to a random STOP_SCENE and then
// IF_NOT_RUNNING(1,53). The zero-weight random STOP is a test-only
// replacement to avoid graphics; the condition and block ordering is real.
// This records that a failed IF_IS_RUNNING leaves the skip flag set\n// across the following IF_NOT_RUNNING, even when that predicate succeeds.\n// Whether that is intended for the original script remains unproven.
func TestStandADSRunningConditionBoundaries(t *testing.T) {
	oldThreads, oldStop := ttmThreads, adsStopRequested
	oldOps, oldCount := adsRandOps, adsNumRandOps
	defer func() {
		ttmThreads, adsStopRequested = oldThreads, oldStop
		adsRandOps, adsNumRandOps = oldOps, oldCount
	}()
	data := []byte{
		0x70, 0x13, 1, 0, 53, 0, // IF_IS_RUNNING(1,53)
		0x10, 0x30, // RANDOM_START
		0x20, 0x30, 0, 0, // zero-weight NOP (test-only)
		0xff, 0x30, // RANDOM_END
		0x60, 0x13, 1, 0, 53, 0, // IF_NOT_RUNNING(1,53)
		0x10, 0x15, // PLAY_SCENE
		0xff, 0xff, // END sentinel
	}
	for _, tc := range []struct {
		name string
		running bool
		wantStop int
	}{
		{"scene absent (skip flag persists)", false, 1},
		{"scene present (not-running condition fails)", true, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ttmThreads = [MaxTTMThreads]TTtmThread{}
			if tc.running {
				ttmThreads[0] = TTtmThread{isRunning: 1, sceneSlot: 1, sceneRootTag: 53}
			}
			adsStopRequested = 0
			adsRandomStart()
			adsPlayChunk(data, uint32(len(data)), 0)
			if adsStopRequested != tc.wantStop {
				t.Fatalf("running=%v: stop=%d, want %d", tc.running, adsStopRequested, tc.wantStop)
			}
		})
	}
}


// TestBuildingADSRepeatedANDGuard characterizes the IF_NOT_RUNNING(3,38)
// AND IF_NOT_RUNNING(3,40) guard repeated in BUILDING.ADS tags 5, 9, 7
// and 8. PLAY_SCENE and END are test-only sentinels, so no scene is drawn.
func TestBuildingADSRepeatedANDGuard(t *testing.T) {
	oldThreads, oldStop := ttmThreads, adsStopRequested
	defer func() { ttmThreads, adsStopRequested = oldThreads, oldStop }()
	data := []byte{
		0x60, 0x13, 3, 0, 38, 0, // IF_NOT_RUNNING(3,38)
		0x20, 0x14, // AND
		0x60, 0x13, 3, 0, 40, 0, // IF_NOT_RUNNING(3,40)
		0x10, 0x15, // PLAY_SCENE sentinel
		0xff, 0xff, // END sentinel
	}
	for _, tc := range []struct {
		name string
		running []uint16
		wantStop int
	}{
		{"neither running", nil, 0},
		{"first running", []uint16{38}, 1},
		{"second running", []uint16{40}, 1},
		{"both running", []uint16{38, 40}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ttmThreads = [MaxTTMThreads]TTtmThread{}
			for i, tag := range tc.running {
				ttmThreads[i] = TTtmThread{isRunning: 1, sceneSlot: 3, sceneRootTag: tag}
			}
			adsStopRequested = 0
			adsPlayChunk(data, uint32(len(data)), 0)
			if adsStopRequested != tc.wantStop {
				t.Fatalf("running=%v: stop=%d, want %d", tc.running, adsStopRequested, tc.wantStop)
			}
		})
	}
}


// TestBuildingADSDuplicateLastPlayedDispatch characterizes BUILDING.ADS
// tag 2, which registers IF_LASTPLAYED(1,58) twice (original script offsets
// 1412 and 1646). Both callbacks must run on a single completion. The
// first callback uses END and the second PLAY_SCENE as harmless sentinels;
// the original ADD_SCENE/STOP_SCENE actions are intentionally omitted.
func TestBuildingADSDuplicateLastPlayedDispatch(t *testing.T) {
	oldChunks, oldCount := adsChunks, numAdsChunks
	oldLocal, oldLocalCount := adsChunksLocal, numAdsChunksLocal
	oldTags, oldTagCount := adsTags, adsNumTags
	oldStop := adsStopRequested
	defer func() {
		adsChunks, numAdsChunks = oldChunks, oldCount
		adsChunksLocal, numAdsChunksLocal = oldLocal, oldLocalCount
		adsTags, adsNumTags = oldTags, oldTagCount
		adsStopRequested = oldStop
	}()
	// Two IF_LASTPLAYED(1,58) bookmarks with independent continuation
	// bodies. The first END marks its execution via adsStopRequested.
	// The second has a PLAY_SCENE sentinel and does not clear that mark.
	data := []byte{
		2, 0, // ADS tag 2
		0x50, 0x13, 1, 0, 58, 0, // IF_LASTPLAYED(1,58)
		0xff, 0xff, // END sentinel
		0x50, 0x13, 1, 0, 58, 0, // IF_LASTPLAYED(1,58)
		0x10, 0x15, // PLAY_SCENE sentinel
	}
	var offset uint32
	adsLoad(data, uint32(len(data)), 1, 2, &offset)
	if offset != 2 || numAdsChunks != 2 {
		t.Fatalf("offset=%d bookmarks=%d, want 2 and 2", offset, numAdsChunks)
	}
	if adsChunks[0].offset != 8 || adsChunks[1].offset != 16 {
		t.Fatalf("continuation offsets = %d,%d; want 8,16",
			adsChunks[0].offset, adsChunks[1].offset)
	}
	adsStopRequested = 0
	adsPlayTriggeredChunks(data, uint32(len(data)), 1, 58)
	if adsStopRequested != 1 {
		t.Fatal("completion (1,58) did not run the first continuation")
	}
	adsStopRequested = 0
	adsPlayTriggeredChunks(data, uint32(len(data)), 1, 99)
	if adsStopRequested != 0 {
		t.Fatal("unrelated completion unexpectedly ran a continuation")
	}
}


// TestBuildingADSORBookmarkChain characterizes the two IF_LASTPLAYED
// alternatives (3,144) and (3,78) observed in BUILDING.ADS tags 7/8.
// The fixture uses only control-flow instructions and END sentinels;
// it does not attempt to reproduce the original scene side effects.
func TestBuildingADSORBookmarkChain(t *testing.T) {
	oldChunks, oldCount := adsChunks, numAdsChunks
	oldLocal, oldLocalCount := adsChunksLocal, numAdsChunksLocal
	oldTags, oldTagCount := adsTags, adsNumTags
	oldStop := adsStopRequested
	defer func() {
		adsChunks, numAdsChunks = oldChunks, oldCount
		adsChunksLocal, numAdsChunksLocal = oldLocal, oldLocalCount
		adsTags, adsNumTags = oldTags, oldTagCount
		adsStopRequested = oldStop
	}()
	data := []byte{
		7, 0, // tag
		0x50, 0x13, 3, 0, 144, 0, // IF_LASTPLAYED(3,144)
		0x30, 0x14, // OR
		0x50, 0x13, 3, 0, 78, 0, // IF_LASTPLAYED(3,78)
		0xff, 0xff, // END sentinel
	}
	var offset uint32
	adsLoad(data, uint32(len(data)), 1, 7, &offset)
	if offset != 2 || numAdsChunks != 2 {
		t.Fatalf("tag offset=%d, bookmarks=%d; want 2,2", offset, numAdsChunks)
	}
	if adsChunks[0].scene.slot != 3 || adsChunks[0].scene.tag != 144 ||
		adsChunks[1].scene.slot != 3 || adsChunks[1].scene.tag != 78 {
		t.Fatal("OR alternatives did not register both scene bookmarks")
	}
	for _, tc := range []struct {
		name string
		slot, tag uint16
		wantStop int
	}{
		{"first alternative", 3, 144, 1},
		{"second alternative", 3, 78, 1},
		{"unrelated scene", 3, 79, 0},
		{"wrong slot", 2, 144, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			adsStopRequested = 0
			adsPlayTriggeredChunks(data, uint32(len(data)), tc.slot, tc.tag)
			if adsStopRequested != tc.wantStop {
				t.Fatalf("completion (%d,%d): stop=%d, want %d",
					tc.slot, tc.tag, adsStopRequested, tc.wantStop)
			}
		})
	}
}
