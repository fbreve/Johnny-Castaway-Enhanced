package main

import "testing"

func TestIsWaylandPlatform(t *testing.T) {
	for _, tt := range []struct {
		name string
		platform int
		want bool
	}{
		{"Wayland", 0x00060003, true},
		{"X11", 0x00060004, false},
		{"uninitialized", 0, false},
		{"other", 0x00060005, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := isWaylandPlatform(tt.platform); got != tt.want {
				t.Fatalf("platform %#x: got %t, want %t", tt.platform, got, tt.want)
			}
		})
	}
}
