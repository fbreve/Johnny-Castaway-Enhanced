package main

import "fmt"

var (
	walkPath     int = -1
	currentSpot  int
	currentHdg   int
	nextSpot     int
	nextHdg      int
	finalSpot    int
	finalHdg     int
	increment    int
	lastTurn     int
	hasArrived   int
	isBehindTree int
)

var (

	// In the C code this is a pointer to a row of 4 uint16, and it references walkData
	// I will just make it be an index to walkData.
	// In the original it's also a static field, so we make it global.
	data = int(-1)
)

func walkInit(fromSpot, fromHdg, toSpot, toHdg int) {
	walkPath = calcPath(fromSpot, toSpot)

	currentSpot = fromSpot
	currentHdg = fromHdg
	finalSpot = toSpot
	finalHdg = toHdg
	hasArrived = 0
	isBehindTree = 0

	if currentSpot == finalSpot {
		nextSpot = -1
		nextHdg = finalHdg
		lastTurn = 1
	} else {
		// Instead of this:
		//nextSpot = *(++walkPath);
		// We do this:
		walkPath++
		nextSpot = paths[walkPath][0]

		nextHdg = walkDataStartHeadings[currentSpot][nextSpot]
		lastTurn = 0
	}

	increment = (nextHdg - currentHdg) & 0x07
	if increment != 0 {
		if increment < 4 {
			increment = 1
		} else {
			increment = -1
		}
	}
}

func walkAnimate(ttmThread *TTtmThread, ttmBgSlot *TTtmSlot) int {
	ttmSlot := ttmThread.ttmSlot
	sur := ttmThread.ttmLayer
	delay := 0

	if hasArrived == 0 {

		// Are we turning ?
		if nextHdg != -1 {

			// More than one iteration left? yes, so let's turn
			if (((nextHdg - currentHdg) & 0x07) % 7) > 1 {
				currentHdg = (currentHdg + increment) & 7
				//data = &walkData[walkDataBookmarksTurns[currentSpot]+currentHdg]
				data = walkDataBookmarksTurns[currentSpot] + currentHdg
				if lastTurn != 0 {
					data += 9
				}

				// The turn is over
			} else {

				// Do we have another spot to walk to ?
				if currentSpot != finalSpot {
					nextHdg = -1
					if (currentSpot == 3 && nextSpot == 4) ||
						(currentSpot == 4 && nextSpot == 3) {
						isBehindTree = 1
					} else {
						isBehindTree = 0
					}
					//data = &walkData[walkDataBookmarks[currentSpot][nextSpot]]
					data = walkDataBookmarks[currentSpot][nextSpot]

				} else { // Else, we arrived to destination
					//data = &walkData[walkDataBookmarksTurns[finalSpot] + finalHdg];
					data = walkDataBookmarksTurns[finalSpot] + finalHdg
					data += 9 // hands in pockets
					hasArrived = 1
				}
			}

			// Walking forward
		} else {

			data++

			// Have we reached a spot ? So let's begin a turn...
			//if (!(*data)[1]) {
			if walkData[data][1] == 0 {
				currentHdg = walkDataEndHeadings[currentSpot][nextSpot]
				currentSpot = nextSpot

				// What's the next heading ?
				// And the next spot of the path to reach ?
				if currentSpot != finalSpot {
					// Instead of this:
					//nextSpot = *(++walkPath);
					// We do this:
					walkPath++
					nextSpot = paths[walkPath][0]

					nextHdg = walkDataStartHeadings[currentSpot][nextSpot]
				} else {
					nextHdg = finalHdg
					lastTurn = 1
				}

				// Turning: left or right ?
				increment = (nextHdg - currentHdg) & 0x07
				if increment != 0 {
					if increment < 4 {
						increment = 1
					} else {
						increment = -1
					}
				}

				currentHdg = (currentHdg + increment) & 7
				//data = &walkData[walkDataBookmarksTurns[currentSpot] + currentHdg]
				data = walkDataBookmarksTurns[currentSpot] + currentHdg

				if lastTurn > 0 {
					data += 9 // hands in pockets
					if currentHdg == finalHdg {
						hasArrived = 1
					}
				}
			}
		}

		fmt.Printf("WALKING:  spot=%d hdg=%d next=%d - data %d %d %d %d\n",
			currentSpot, currentHdg, nextHdg,
			walkData[data][0], walkData[data][1], walkData[data][2], walkData[data][3])

		grClearScreen(sur)

		//if ((*data)[0]){
		if walkData[data][0] != 0 {
			grDrawSpriteFlip(sur, ttmSlot,
				int16(walkData[data][1])-1, int16(walkData[data][2]), walkData[data][3], 0)
		} else {
			grDrawSprite(sur, ttmSlot,
				int16(walkData[data][1])-1, int16(walkData[data][2]), walkData[data][3], 0)
		}

		if isBehindTree != 0 {
			grDrawSprite(sur, ttmBgSlot, 442, 148, 13, 0) // trunk
			grDrawSprite(sur, ttmBgSlot, 365, 122, 12, 0) // leafs
		}

		if hasArrived > 0 {
			delay = 80
		} else {
			delay = 6
		}
	} else {
		fmt.Println("WALKING: end walk")
		delay = 0
	}

	return delay
}
