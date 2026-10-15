package main

import "testing"

func TestParseCLIConsumesValuesAndAvoidsPrefixMatches(t *testing.T) {
	o := parseCLI([]string{"-m", "2", "-x", "christmas", "-d"})
	if !o.hasMonitor || o.monitor != 2 { t.Fatalf("monitor = %v/%d", o.hasMonitor, o.monitor) }
	if o.holiday != 3 || o.holidayArg != "christmas" { t.Fatalf("holiday = %d/%q", o.holiday, o.holidayArg) }
	if o.forcedNight != 0 || o.dayNightArg != "day" { t.Fatalf("day mode not parsed") }

	for _, arg := range []string{"-settings", "-config", "-previewing", "-testing", "-benchmark", "-keyboard"} {
		o := parseCLI([]string{arg})
		if o.settings || o.preview || o.run || o.test || o.bench || o.hotKeys {
			t.Fatalf("%q accidentally matched a short option: %+v", arg, o)
		}
	}
}

func TestParseCLICompactForms(t *testing.T) {
	o := parseCLI([]string{"-m:3", "-xhalloween"})
	if !o.hasMonitor || o.monitor != 3 || o.holiday != 1 { t.Fatalf("compact forms: %+v", o) }
}

func TestParseCLIInvalidValuesDoNotActivateOverrides(t *testing.T) {
	o := parseCLI([]string{"-m", "-1", "-x", "bogus"})
	if o.hasMonitor { t.Fatalf("invalid monitor activated override: %+v", o) }
	if o.holiday != 0 || o.holidayArg != "" { t.Fatalf("invalid holiday propagated: %+v", o) }
}

func TestParseCLITestArgumentsAreConsumed(t *testing.T) {
	o := parseCLI([]string{"/t", "BUILDING.ADS", "2", "-k"})
	if !o.test || o.testAds != "BUILDING.ADS" || o.testTag != 2 || !o.hotKeys {
		t.Fatalf("test args: %+v", o)
	}
}

func TestParseCLIWindowsSaverForms(t *testing.T) {
	if o := parseCLI([]string{"/c:123"}); !o.settings { t.Fatal("/c: handle not recognized") }
	if o := parseCLI([]string{"/p:456"}); !o.preview { t.Fatal("/p: handle not recognized") }
}
