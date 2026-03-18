package main

import "embed"

//go:embed assets/RESOURCE.MAP
var embeddedMap []byte

//go:embed assets/RESOURCE.001
var embeddedRes []byte

//go:embed resources/*.wav
var embeddedSounds embed.FS
