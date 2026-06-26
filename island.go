package main

import (
	"fmt"
	"math/rand"
)

var (
	// In c, these are static, so I hoisted them to be global vars.
	counter1 int = 0
	counter2 int = 0

	islandState = TIslandState{
		lowTide: 0,
		night:   0,
		raft:    0,
		holiday: 0,
		xPos:    0,
		yPos:    0,
	}
)

type TCloudState struct {
	numClouds     int32
	windDirection int32
	windSpeed     [5]int32
	cloudNo       [5]int32
	xPos          [5]int32
	yPos          [5]int32
}

type TIslandState struct {
	lowTide int
	night   int
	raft    int
	holiday int
	xPos    int
	yPos    int
	clouds  TCloudState
}

func islandInit(ttmThread *TTtmThread) {
	ttmSlot := ttmThread.ttmSlot

	if islandState.night != 0 {
		grLoadScreen("NIGHT.SCR")
	} else {
		scrName := fmt.Sprintf("OCEAN0%d.SCR", rand.Int()%3)
		grLoadScreen(scrName)
	}

	ttmThread.ttmLayer = grBackgroundSur

	grDx = islandState.xPos
	grDy = islandState.yPos

	// Raft

	grLoadBmp(ttmSlot, 0, "MRAFT.BMP")

	var xRaft int
	var yRaft int
	if islandState.lowTide != 0 {
		xRaft = 529
	} else {
		xRaft = 512
	}
	if islandState.lowTide != 0 {
		yRaft = 281
	} else {
		yRaft = 266
	}

	switch islandState.raft {
	case 1:
		// raft-1
		grDrawSprite(grBackgroundSur, ttmSlot, int16(xRaft), int16(yRaft), 0, 0)
	case 2:
		// raft-2
		grDrawSprite(grBackgroundSur, ttmSlot, int16(xRaft), int16(yRaft), 1, 0)
	case 3:
		// raft-3
		grDrawSprite(grBackgroundSur, ttmSlot, int16(xRaft), int16(yRaft), 2, 0)
	case 4:
		// raft-4
		grDrawSprite(grBackgroundSur, ttmSlot, int16(xRaft), int16(yRaft), 3, 0)
	case 5:
		// raft-5
		grDrawSprite(grBackgroundSur, ttmSlot, int16(xRaft), int16(yRaft), 4, 0)
	}

	// Clouds
	grLoadBmp(ttmSlot, 0, "BACKGRND.BMP")

	grDx = 0
	grDy = 0

	cloudX := uint16(0)
	cloudY := uint16(0)

	numClouds := int32(rand.Int() % 6)
	windDirection := int32(rand.Int() % 2)

	islandState.clouds.numClouds = numClouds
	islandState.clouds.windDirection = windDirection

	for i := range numClouds {
		cloudNo := rand.Int() % 3
		switch cloudNo {
		case 0:
			cloudX = uint16(rand.Int() % (640 - 129))
			cloudY = uint16(rand.Int()%(100-36) + 25)

		case 1:
			cloudX = uint16(rand.Int() % (640 - 192))
			cloudY = uint16(rand.Int()%(100-57) + 25)
		case 2:
			cloudX = uint16(rand.Int() % (640 - 264))
			cloudY = uint16(rand.Int()%(100-76) + 25)
		}
		islandState.clouds.windSpeed[i] = int32(rand.Int()%2 + 1)
		islandState.clouds.cloudNo[i] = int32(cloudNo)
		islandState.clouds.xPos[i] = int32(cloudX)
		islandState.clouds.yPos[i] = int32(cloudY)
	}

	grDx = islandState.xPos
	grDy = islandState.yPos

	// The island itself
	grDrawSprite(grBackgroundSur, ttmSlot, 288, 279, 0, 0)  // island
	grDrawSprite(grBackgroundSur, ttmSlot, 442, 148, 13, 0) // trunk
	grDrawSprite(grBackgroundSur, ttmSlot, 365, 122, 12, 0) // leafs
	grDrawSprite(grBackgroundSur, ttmSlot, 396, 279, 14, 0) // palmtree's shadow

	if islandState.lowTide != 0 {
		grDrawSprite(grBackgroundSur, ttmSlot, 249, 303, 1, 0) // low tide shore
		grDrawSprite(grBackgroundSur, ttmSlot, 150, 328, 2, 0) // rock
	}

	// Initial waves on the shore
	for i := 0; i < 4; i++ {
		islandAnimate(ttmThread)
	}

	// Waves animation thread
	const val = 8
	ttmThread.delay = val
	ttmThread.timer = val
}

func islandAnimate(ttmThread *TTtmThread) {
	ttmSlot := ttmThread.ttmSlot

	grDx = islandState.xPos
	grDy = islandState.yPos

	if islandState.lowTide != 0 {
		counter2++
		counter2 %= 4

		switch counter2 {
		case 0:
			grDrawSprite(grBackgroundSur, ttmSlot, 129, 340, 39+uint16(counter1), 0)
			// rock waves (40)
		case 1:
			grDrawSprite(grBackgroundSur, ttmSlot, 233, 323, 30+uint16(counter1), 0)
			// low tide waves - left (31)
		case 2:
			grDrawSprite(grBackgroundSur, ttmSlot, 367, 356, 33+uint16(counter1), 0)
			// low tide waves - center (33)
		case 3:
			grDrawSprite(grBackgroundSur, ttmSlot, 558, 323, 36+uint16(counter1), 0)
			// low tide waves - right (36)
		}
	} else {
		counter2++
		counter2 %= 3

		switch counter2 {
		case 0:
			grDrawSprite(grBackgroundSur, ttmSlot, 270, 306, 3+uint16(counter1), 0)
			// high tide waves - left (3)
		case 1:
			grDrawSprite(grBackgroundSur, ttmSlot, 364, 319, 6+uint16(counter1), 0)
			// high tide waves - center (6)
		case 2:
			grDrawSprite(grBackgroundSur, ttmSlot, 518, 303, 9+uint16(counter1), 0)
			// high tide waves - right (9)
		}
	}

	if counter2 == 0 {
		counter1++
		counter1 %= 2
	}
}

func islandInitHoliday(ttmThread *TTtmThread) {
	ttmSlot := ttmThread.ttmSlot

	if islandState.holiday != 0 {
		ttmThread.ttmLayer = grNewLayer()
		ttmThread.isRunning = 3

		grDx = islandState.xPos
		grDy = islandState.yPos

		grLoadBmp(ttmSlot, 0, "HOLIDAY.BMP")

		switch islandState.holiday {
		case 1:
			grDrawSprite(ttmThread.ttmLayer, ttmSlot, 410, 298, 0, 0) // Halloween
		case 2:
			grDrawSprite(ttmThread.ttmLayer, ttmSlot, 333, 286, 1, 0) // St Patrick
		case 3:
			grDrawSprite(ttmThread.ttmLayer, ttmSlot, 404, 267, 2, 0) // Christmas
		case 4:
			grDrawSprite(ttmThread.ttmLayer, ttmSlot, 361, 155, 3, 0) // New year
		}

		grReleaseBmp(ttmSlot, 0)
	} else {
		ttmThread.isRunning = 0
	}
}

func islandAnimateClouds(ttmThread *TTtmThread) {
	// r.c. - re-enabled. The actual cost was the grLoadBmp() call that used
	// to run here every tick (a full resource decompress + GPU texture
	// upload), not the position/draw logic below. BACKGRND.BMP is now
	// loaded once at island init time instead (see adsInitIsland), so this
	// function only updates cloud positions and draws -- cheap per tick.
	ttmSlot := ttmThread.ttmSlot
	grClearScreen(ttmThread.ttmLayer)
	if islandState.clouds.numClouds > 0 {
		ttmThread.isRunning = 3

		// animate clouds x position
		for i := int32(0); i < islandState.clouds.numClouds; i++ {
			cloudNo := islandState.clouds.cloudNo[i]
			cloudX := islandState.clouds.xPos[i]
			cloudY := islandState.clouds.yPos[i]

			if cloudX > 640+264 {
				cloudX = -264
			} else if cloudX < -264 {
				cloudX = 640 + 264
			} else {
				if islandState.clouds.windDirection > 0 {
					cloudX -= islandState.clouds.windSpeed[i]
				} else {
					cloudX += islandState.clouds.windSpeed[i]
				}
			}

			//fmt.Printf("Clouds Pos: %d, %d\n", cloudX, cloudY)
			if islandState.clouds.windDirection > 0 {
				grDrawSprite(ttmThread.ttmLayer, ttmSlot, int16(cloudX), int16(cloudY), uint16(15+cloudNo), 0)
			} else {
				grDrawSpriteFlip(ttmThread.ttmLayer, ttmSlot, int16(cloudX), int16(cloudY), uint16(15+cloudNo), 0)
			}

			islandState.clouds.xPos[i] = cloudX
			islandState.clouds.yPos[i] = cloudY
		}
	} else {
		ttmThread.isRunning = 0
	}
}
