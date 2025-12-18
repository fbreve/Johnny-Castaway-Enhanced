package main

func inverAdsPlaySingleTtmStart(ttmName string) {
	adsInit()
	ttmLoadTTM(&ttmSlots[0], ttmName)
	adsAddScene(0, 0, 0)
	ttmThreads[0].ip = 0
	gGame.ChangeState(TTMSingleModePoll)
}

func inverAdsPlaySingleTtmPoll() {

	ttmPlay(&ttmThreads[0])
	ttmThreads[0].isRunning = 1
	grUpdateDisplay(nil, ttmThreads[:], nil)
	grUpdateDelay = int(ttmThreads[0].delay)

	// it crashes...not sure why.
	if ttmThreads[0].ip == (ttmSlots[0].dataSize - 1) {
		gGame.ChangeState(TTMSingleModeEnd)
	}
}

func inverAdsPlaySingleTtmEnd() {
	adsStopScene(0)
	ttmResetSlot(&ttmSlots[0])

	gGame.ChangeState(None)
}
