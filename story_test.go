package main

import "testing"

func TestStoryCalculateNightFromHourTransitions(t *testing.T) {
	oldForcedNight := forcedNight
	oldNight := islandState.night
	defer func() {
		forcedNight = oldForcedNight
		islandState.night = oldNight
	}()

	forcedNight = -1

	tests := []struct {
		name string
		hour int
		want int
	}{
		{"midnight", 0, 1},
		{"before dawn", 5, 1},
		{"day starts", 6, 0},
		{"midday", 12, 0},
		{"before night", 17, 0},
		{"night starts", 18, 1},
		{"late night", 23, 1},
	}

	// Deliberately reuse islandState across cases so this also catches stale
	// night state when transitioning from a nighttime calculation to daytime.
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storyCalculateNightFromHour(tt.hour)
			if islandState.night != tt.want {
				t.Fatalf("hour %d: night = %d, want %d", tt.hour, islandState.night, tt.want)
			}
		})
	}
}

func TestStoryCalculateNightFromHourForcedOverrides(t *testing.T) {
	oldForcedNight := forcedNight
	oldNight := islandState.night
	defer func() {
		forcedNight = oldForcedNight
		islandState.night = oldNight
	}()

	forcedNight = 0
	storyCalculateNightFromHour(23)
	if islandState.night != 0 {
		t.Fatalf("forced day at 23:00: night = %d, want 0", islandState.night)
	}

	forcedNight = 1
	storyCalculateNightFromHour(12)
	if islandState.night != 1 {
		t.Fatalf("forced night at 12:00: night = %d, want 1", islandState.night)
	}
}


func TestUpdateStoryDay(t *testing.T) {
	tests := []struct {
		name       string
		day, date  int
		today      int
		wantDay    int
		wantDate   int
		wantChange bool
	}{
		{"same date", 4, 100, 100, 4, 100, false},
		{"next date", 4, 100, 101, 5, 101, true},
		{"wrap day", 11, 100, 101, 1, 101, true},
		{"repair invalid day", 0, 100, 100, 1, 100, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := TConfig{CurrentDay: tt.day, CurrentDate: tt.date}
			got := updateStoryDay(&cfg, tt.today)
			if got != tt.wantChange || cfg.CurrentDay != tt.wantDay || cfg.CurrentDate != tt.wantDate {
				t.Fatalf("updateStoryDay() = %v, day=%d date=%d; want %v, day=%d date=%d",
					got, cfg.CurrentDay, cfg.CurrentDate, tt.wantChange, tt.wantDay, tt.wantDate)
			}
		})
	}
}
