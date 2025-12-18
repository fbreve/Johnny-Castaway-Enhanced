package main

import (
	"math/rand"
	"time"
)

var (
	storyCurrentDay int = 1
)

func storyPickScene(wantedFlags uint16, unwantedFlags uint16) *TStoryScene {
	var scenes [NUM_SCENES]int
	var numScenes = 0

	for i := 0; i < NUM_SCENES; i++ {
		scene := storyScenes[i]

		if uint16(scene.flags)&wantedFlags == wantedFlags &&
			(uint16(scene.flags)&unwantedFlags) != 0 &&
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
	// determines island x/y pos and whether low or high tide
	// and raft state

	// just hacking for now - r.c.
	islandState.lowTide = 0
	islandState.xPos = -222 + rand.Int()%109
	islandState.yPos = -44 + rand.Int()%128

	islandState.raft = 0
}

// main story entry point
func storyPlay() {
	adsInit()
	//adsPlayIntro()

	for {
		time.Sleep(time.Millisecond * 100)
	}
}
