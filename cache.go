package main

import rl "github.com/gen2brain/raylib-go/raylib"

const (
	cloudBmpKey = "BACKGRND.BMP"
)

var (
	bmpCache = make(map[string]*rl.Texture2D)
)

func Get(key string) *rl.Texture2D {
	if val, ok := bmpCache[key]; ok {
		return val
	}
	return nil
}

func Set(key string, texture *rl.Texture2D) {
	bmpCache[key] = texture
}
