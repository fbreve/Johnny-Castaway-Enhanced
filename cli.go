package main

import (
	"strconv"
	"strings"
)

type cliOptions struct {
	settings, preview, run, test, bench, help bool
	testAds string
	testTag int
	hotKeys, child bool
	hasMonitor bool
	monitor int
	holidayArg string
	holiday int
	forcedNight int
	dayNightArg string
}

func parseCLI(args []string) cliOptions {
	o := cliOptions{forcedNight: -1}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		lower := strings.ToLower(arg)
		switch {
		case lower == "/c" || lower == "-c" || strings.HasPrefix(lower, "/c:"):
			o.settings = true
		case lower == "/p" || lower == "-p" || strings.HasPrefix(lower, "/p:"):
			o.preview = true
		case lower == "/s" || lower == "-s":
			o.run = true
		case lower == "/t" || lower == "-t":
			o.test = true
			if i+1 < len(args) && !looksLikeOption(args[i+1]) {
				o.testAds = args[i+1]
				i++
				if i+1 < len(args) && !looksLikeOption(args[i+1]) {
					if n, err := strconv.Atoi(args[i+1]); err == nil {
						o.testTag = n
						i++
					}
				}
			}
		case lower == "/b" || lower == "-b":
			o.bench = true
		case lower == "/k" || lower == "-k":
			o.hotKeys = true
		case lower == "--child" || lower == "-child" || lower == "/child":
			o.child = true
		case lower == "/1" || lower == "-1" || lower == "--1" ||
			lower == "/main" || lower == "-main" || lower == "--main" ||
			lower == "/single" || lower == "-single" || lower == "--single" ||
			lower == "/primary" || lower == "-primary" || lower == "--primary":
			o.hasMonitor, o.monitor = true, 0
		case lower == "-h" || lower == "--help" || lower == "-?" || lower == "/h" || lower == "/?":
			o.help = true
		case lower == "-d" || lower == "--day" || lower == "/d":
			o.forcedNight, o.dayNightArg = 0, "day"
		case lower == "-n" || lower == "--night" || lower == "/n":
			o.forcedNight, o.dayNightArg = 1, "night"
		case lower == "-x" || lower == "/x" || lower == "--holiday" ||
			strings.HasPrefix(lower, "-x:") || strings.HasPrefix(lower, "-x=") ||
			strings.HasPrefix(lower, "/x:") || strings.HasPrefix(lower, "/x=") ||
			strings.HasPrefix(lower, "--holiday:") || strings.HasPrefix(lower, "--holiday=") ||
			(strings.HasPrefix(lower, "-x") && len(lower) > 2):
			value := optionValue(arg)
			if value == "" && i+1 < len(args) && !looksLikeOption(args[i+1]) {
				value = strings.ToLower(args[i+1])
				i++
			}
			if value != "" {
				if h := holidayFromArg(value); h != 0 {
					o.holidayArg, o.holiday = value, h
				}
			}
		case lower == "-m" || lower == "/m" || lower == "--monitor" ||
			strings.HasPrefix(lower, "-m:") || strings.HasPrefix(lower, "-m=") ||
			strings.HasPrefix(lower, "/m:") || strings.HasPrefix(lower, "/m=") ||
			strings.HasPrefix(lower, "--monitor:") || strings.HasPrefix(lower, "--monitor="):
			value := optionValue(arg)
			if value == "" && i+1 < len(args) && !looksLikeOption(args[i+1]) {
				value = args[i+1]
				i++
			}
			if n, err := strconv.Atoi(value); err == nil && n >= 0 {
				o.hasMonitor, o.monitor = true, n
			}
		}
	}
	return o
}

func looksLikeOption(s string) bool {
	return strings.HasPrefix(s, "-") || strings.HasPrefix(s, "/")
}

func optionValue(arg string) string {
	if p := strings.IndexAny(arg, ":="); p >= 0 && p+1 < len(arg) {
		return strings.ToLower(arg[p+1:])
	}
	lower := strings.ToLower(arg)
	if strings.HasPrefix(lower, "-x") && !strings.HasPrefix(lower, "--") && len(arg) > 2 {
		return strings.ToLower(arg[2:])
	}
	return ""
}
