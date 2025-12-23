package main

import (
	"fmt"
	rl "github.com/gen2brain/raylib-go/raylib"
	"math/rand"
)

// r.c. This lame "sound engine" was fundamentally different and way simpler than the C original.

var (
	sfx = []string{
		"sound0.wav",
		"sound1.wav",
		"sound2.wav",
		"sound3.wav",
		"sound4.wav",
		"sound5.wav",
		"sound6.wav",
		"sound7.wav",
		"sound8.wav",
		"sound9.wav",
		"sound10.wav",
		"missing",
		"sound12.wav",
		"missing",
		"sound14.wav",
		"sound15.wav",
		"sound16.wav",
		"sound17.wav",
		"sound18.wav",
		"sound19.wav",
		"sound20.wav",
		"sound21.wav",
		"sound22.wav",
		"sound23.wav",
		"sound24.wav",
	}
	soundSfx = make([]rl.Sound, len(sfx))
)

func loadSfx() {
	for i, filename := range sfx {
		if filename == "missing" {
			continue
		}
		snd := rl.LoadSound("resources/" + filename)
		soundSfx[i] = snd
	}
}

func unloadSfx() {
	for i, snd := range soundSfx {
		if sfx[i] == "missing" {
			continue
		}
		rl.UnloadSound(snd)
	}
}

func soundPlay(id uint16) {
	if int(id) > len(soundSfx)-1 {
		fmt.Printf("sound id index out of range:%d\n", id)
		return
	}
	if sfx[id] == "missing" {
		fmt.Println("missing audio for this id =>", id)
		return
	}
	snd := soundSfx[id]

	f := float32(rand.Float64()*0.5 - 0.25)
	rl.SetSoundPitch(snd, 1.0+f)
	rl.PlaySound(snd)
}
