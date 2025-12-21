package main

import (
	"math/rand"
)

var (
	storyCurrentDay int = 1
)

func storyPickScene(wantedFlags uint16, unwantedFlags uint16) *TStoryScene {
	var scenes [NUM_SCENES]int
	var numScenes = 0

	for i := 0; i < NUM_SCENES; i++ {
		scene := storyScenes[i]
		flags := uint16(scene.flags)
		if (flags&wantedFlags) == wantedFlags &&
			(flags&unwantedFlags) == 0 &&
			(scene.dayNo == 0 || scene.dayNo == storyCurrentDay) {
			scenes[numScenes] = i
			numScenes++
		}
	}

	return &storyScenes[scenes[rand.Int()%numScenes]]
}

func storyUpdateCurrentDay() {
	// TODO: writes to some local config so it tracks over the long term.
}

func storyCalculateIslandFromDateAndTime() {
	// determines holidays and whether it's nighttime.
	// just hacking for now - r.c.
	islandState.night = 0
	islandState.holiday = 0
}

func storyCalculateIslandFromScene(scene *TStoryScene) {
	// Low tide ?
	if (scene.flags&LOWTIDE_OK == LOWTIDE_OK) && (rand.Int()%2 != 0) {
		islandState.lowTide = 1
	} else {
		islandState.lowTide = 0
	}

	// Randomize the position of the island
	if scene.flags&VARPOS_OK == VARPOS_OK {
		if rand.Int()%2 != 0 {
			islandState.xPos = -222 + (rand.Int() % 109)
			islandState.yPos = -44 + (rand.Int() % 128)
		} else if rand.Int()%2 != 0 {
			islandState.xPos = -114 + (rand.Int() % 134)
			islandState.yPos = -14 + (rand.Int() % 99)
		} else {
			islandState.xPos = -114 + (rand.Int() % 119)
			islandState.yPos = -73 + (rand.Int() % 60)
		}
	} else {
		if scene.flags&LEFT_ISLAND == LEFT_ISLAND {
			islandState.xPos = -272
			islandState.yPos = 0
		} else {
			islandState.xPos = 0
			islandState.yPos = 0
		}
	}

	// How much of the raft was John able to build ?
	if scene.flags&NORAFT == NORAFT {
		islandState.raft = 0
	} else {
		switch storyCurrentDay {
		case 0, 1, 2:
			islandState.raft = 1
		case 3, 4, 5:
			islandState.raft = storyCurrentDay - 1
		default:
			islandState.raft = 5
		}
	}

	// For scene VISITOR.ADS#3 (cargo), never display holiday items - or they
	// will be drawn over the hull when it fills the screen at the end. This
	// conforms to the behavior of the original - which, moreover, freezes
	// the shore animation while we dont
	if scene.flags&HOLIDAY_NOK == HOLIDAY_NOK {
		islandState.holiday = 0
	}
}

// main story entry point
func storyPlay() {
	var (
		wantedFlags   = uint16(0)
		unwantedFlags = uint16(0)
	)

	adsInit()
	//adsPlayIntro() // todo: r.c.

	for {
		storyUpdateCurrentDay()
		storyCalculateIslandFromDateAndTime()
		unwantedFlags = 0

		finalScene := storyPickScene(FINAL, unwantedFlags)

		if finalScene.flags&ISLAND == ISLAND {
			storyCalculateIslandFromScene(finalScene)
			adsInitIsland()
		} else {
			adsNoIsland()
		}

		prevSpot := -1
		prevHdg := -1
		_ = prevHdg // because I don't have adsWalk enabled just yet.

		if !(finalScene.flags&FIRST == FIRST) {
			wantedFlags = 0
			unwantedFlags |= FINAL

			if islandState.lowTide != 0 {
				wantedFlags |= LOWTIDE_OK
			}

			if islandState.xPos != 0 || islandState.yPos != 0 {
				wantedFlags |= VARPOS_OK
			}

			// r.c. I think this logic is simply to pick the next scene's starting spot so that the walk animation
			// will flow together (from one scene to the next...)
			for i := 0; i < 6+rand.Int()%14; i++ {
				scene := storyPickScene(wantedFlags, unwantedFlags)

				if prevSpot != -1 {
					//adsPlayWalk(prevSpot, prevHdg, scene.spotStart, scene.hdgStart)
				}

				var xOffset = 0
				if scene.flags&LEFT_ISLAND == LEFT_ISLAND {
					xOffset = 272
				}
				ttmDx = islandState.xPos + xOffset
				ttmDy = islandState.yPos

				if scene.dayNo != 0 {
					//soundPlay(0) //r.c. todo
				}

				unwantedFlags |= FIRST
				prevSpot = scene.spotEnd
				prevHdg = scene.hdgEnd
			}
		}

		if prevSpot != -1 {
			//adsPlayWalk(prevSpot, prevHdg, finalScene.spotStart, finalScene.hdgStart)
		}

		if finalScene.flags&ISLAND == ISLAND {
			xOffset := 0
			if finalScene.flags&LEFT_ISLAND == LEFT_ISLAND {
				xOffset = 272
			}
			ttmDx = islandState.xPos + xOffset
			ttmDy = islandState.yPos
		} else {
			ttmDx = 0
			ttmDy = 0
		}

		if finalScene.dayNo != 0 {
			//soundPlay(0) //todo: r.c.
		}

		adsPlay(finalScene.adsName, uint16(finalScene.adsTagNo))

		grFadeOut()

		if finalScene.flags&ISLAND == ISLAND {
			adsReleaseIsland()
		}
	}
}
