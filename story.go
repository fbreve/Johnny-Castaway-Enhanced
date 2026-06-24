package main

import (
	"fmt"
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

	return &storyScenes[scenes[rand.Intn(numScenes)]]
}

func storyUpdateCurrentDay() {
	today := getDayOfYear()
	hasChanged := false

	if today != activeConfig.CurrentDate {
		fmt.Println("System date has changed since last sequence -> next day of the story")
		activeConfig.CurrentDate = today
		activeConfig.CurrentDay += 1
		hasChanged = true
	}

	if activeConfig.CurrentDay < 1 || activeConfig.CurrentDay > 11 {
		activeConfig.CurrentDay = 1
		hasChanged = true
	}

	if hasChanged {
		cfgFileWrite(&activeConfig)
	}

	storyCurrentDay = activeConfig.CurrentDay
	fmt.Printf("The day of the story is: %d\n", storyCurrentDay)
}

func storyCalculateIslandFromDateAndTime() {
	// Night ?
	hour := getHour()
	if hour < 6 || hour >= 18 {
		islandState.night = 1
	}

	// Holidays ?
	islandState.holiday = 0
	month, day := getMonthAndDay()

	if month == 10 && (day >= 29 && day <= 31) {
		// Halloween : 29/10 to 31/10
		islandState.holiday = 1
	} else if month == 3 && (day >= 15 && day <= 17) {
		// St Patrick: 15/03 to 17/03
		islandState.holiday = 2
	} else if month == 12 && (day >= 23 && day <= 25) {
		// Christmas : 23/12 to 25/12
		islandState.holiday = 3
	} else if (month == 12 && day >= 29) || (month == 1 && day == 1) {
		// New year  : 29/12 to 01/01
		islandState.holiday = 4
	}
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
	grFadeIn()
	adsPlayIntro()
	if shouldExitApp {
		return
	}

	for {
		if shouldExitApp {
			return
		}
		storyUpdateCurrentDay()
		storyCalculateIslandFromDateAndTime()
		unwantedFlags = 0

		finalScene := storyPickScene(FINAL, unwantedFlags)

		if finalScene.flags&ISLAND == ISLAND && activeConfig.Background {
			storyCalculateIslandFromScene(finalScene)
			adsInitIsland()
		} else {
			adsNoIsland()
		}
		grFadeIn()
		if shouldExitApp {
			return
		}

		prevSpot := -1
		prevHdg := -1

		if finalScene.flags&FIRST == 0 {
			wantedFlags = 0
			unwantedFlags |= FINAL

			if islandState.lowTide != 0 {
				wantedFlags |= LOWTIDE_OK
			}

			if islandState.xPos != 0 || islandState.yPos != 0 {
				wantedFlags |= VARPOS_OK
			}

			if finalScene.flags&LEFT_ISLAND == 0 {
				unwantedFlags |= LEFT_ISLAND
			}

			// r.c. I think this logic is simply to pick the next scene's starting spot so that the walk animation
			// will flow together (from one scene to the next...)
			for i := 0; i < 6+rand.Intn(14); i++ {
				if shouldExitApp {
					return
				}
				scene := storyPickScene(wantedFlags, unwantedFlags)

				if prevSpot != -1 {
					adsPlayWalk(prevSpot, prevHdg, scene.spotStart, scene.hdgStart)
				}
				if shouldExitApp {
					return
				}

				var xOffset = 0
				if scene.flags&LEFT_ISLAND == LEFT_ISLAND {
					xOffset = 272
				}
				ttmDx = islandState.xPos + xOffset
				ttmDy = islandState.yPos

				if scene.dayNo != 0 {
					soundPlay(17)
				}

				adsPlay(scene.adsName, uint16(scene.adsTagNo))
				if shouldExitApp {
					return
				}

				unwantedFlags |= FIRST
				prevSpot = scene.spotEnd
				prevHdg = scene.hdgEnd
			}
		}

		if prevSpot != -1 {
			adsPlayWalk(prevSpot, prevHdg, finalScene.spotStart, finalScene.hdgStart)
		}
		if shouldExitApp {
			return
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
			soundPlay(17)
		}

		adsPlay(finalScene.adsName, uint16(finalScene.adsTagNo))
		if shouldExitApp {
			return
		}

		grFadeOut()
		if shouldExitApp {
			return
		}

		if finalScene.flags&ISLAND == ISLAND && activeConfig.Background {
			adsReleaseIsland()
		}
	}
}
