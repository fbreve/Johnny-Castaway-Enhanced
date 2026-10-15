package main

import "testing"

func TestValidHHMM(t *testing.T) {
	valid := []int{0, 30, 100, 900, 1230, 2300, 2359}
	for _, v := range valid {
		if !validHHMM(v) {
			t.Errorf("validHHMM(%d) = false, want true", v)
		}
	}

	invalid := []int{-1, 60, 99, 1260, 2360, 2400, 9999}
	for _, v := range invalid {
		if validHHMM(v) {
			t.Errorf("validHHMM(%d) = true, want false", v)
		}
	}
}
