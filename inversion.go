package main

// These inversion functions below are basically refactored so that I can advance the game one frame at a time
// and allow Ebiten to control the event loop.

//func inverAdsPlaySingleTtmStart(ttmName string) {
//	adsInit()
//	ttmLoadTTM(&ttmSlots[0], ttmName)
//	adsAddScene(0, 0, 0)
//	ttmThreads[0].ip = 0
//	gGame.ChangeState(TTMSingleModePoll)
//}
//
//func inverAdsPlaySingleTtmPoll() {
//
//	ttmPlay(&ttmThreads[0])
//	ttmThreads[0].isRunning = 1
//	grUpdateDisplay(nil, ttmThreads[:], nil)
//	grUpdateDelay = int(ttmThreads[0].delay)
//
//	if ttmThreads[0].ip == ttmSlots[0].dataSize {
//		// r.c. I'm manually clearing out grUpdateDelay to guarantee we truly transition to End.
//		grUpdateDelay = 0
//		gGame.ChangeState(TTMSingleModeEnd)
//	}
//}
//
//func inverAdsPlaySingleTtmEnd() {
//	adsStopScene(0)
//	ttmResetSlot(&ttmSlots[0])
//
//	gGame.ChangeState(None)
//}
