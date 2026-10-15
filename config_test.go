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


func TestFormatConfigIncludesAllFields(t *testing.T) {
	cfg := TConfig{
		CurrentDay: 3, CurrentDate: 42, Background: true, Sounds: false,
		Password: true, StartTime: 930, UseMesa: true, MultiInstance: true,
		Widescreen: true, FilterMode: 6, Scanlines: true, SilentTime: true,
		SilentStart: 2200, SilentEnd: 700,
	}
	want := "currentDay=3\ndate=42\nbackground=true\nsounds=false\npassword=true\nstartTime=930\n" +
		"useMesa=true\nmultiInstance=true\nwidescreen=true\nfilterMode=6\nscanlines=true\n" +
		"silentTime=true\nsilentStart=2200\nsilentEnd=700\n"
	if got := formatConfig(&cfg); got != want {
		t.Fatalf("formatConfig() = %q, want %q", got, want)
	}
}
