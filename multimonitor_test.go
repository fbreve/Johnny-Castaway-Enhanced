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


func TestSelectedMonitorIndex(t *testing.T) {
	tests := []struct {
		name string
		requested int
		hasRequested bool
		current int
		count int
		want int
	}{
		{"requested valid", 2, true, 0, 3, 2},
		{"requested too high falls back current", 9, true, 1, 3, 1},
		{"requested negative falls back current", -1, true, 2, 3, 2},
		{"invalid current falls back primary", 9, false, 9, 3, 0},
		{"no monitors reported", 4, true, 2, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := selectedMonitorIndex(tt.requested, tt.hasRequested, tt.current, tt.count); got != tt.want {
				t.Fatalf("got %d, want %d", got, tt.want)
			}
		})
	}
}
