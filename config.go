package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// r.c. This is not idiomatic Go, but a mostly direct C port of the original source.
// A second pass refactor can be done to fix this garbage.
var (
	// r.c. - added by me in case someone tries to run multiple instances of the screensaver.
	cfgLock      sync.Mutex
	activeConfig TConfig
)

type TConfig struct {
	CurrentDay    int
	CurrentDate   int
	Background    bool
	Sounds        bool
	Password      bool
	StartTime     int
	UseMesa       bool
	MultiInstance bool
	Widescreen    bool
	FilterMode    int
	Scanlines     bool
	SilentTime    bool
	SilentStart   int
	SilentEnd     int
}

const (
	CfgFileName      = ".johnny_castaway_2026"
	CurrentDayKey    = "currentDay="
	DateKey          = "date="
	BackgroundKey    = "background="
	SoundsKey        = "sounds="
	PasswordKey      = "password="
	StartTimeKey     = "startTime="
	UseMesaKey       = "useMesa="
	MultiInstanceKey = "multiInstance="
	WidescreenKey    = "widescreen="
	FilterModeKey    = "filterMode="
	ScanlinesKey     = "scanlines="
	SilentTimeKey    = "silentTime="
	SilentStartKey   = "silentStart="
	SilentEndKey     = "silentEnd="
)

func cfgFullPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic(fmt.Errorf("user home dir: %w", err))
	}

	fullPath := filepath.Join(homeDir, CfgFileName)
	return fullPath
}

func formatConfig(cfg *TConfig) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s%d\n", CurrentDayKey, cfg.CurrentDay)
	fmt.Fprintf(&b, "%s%d\n", DateKey, cfg.CurrentDate)
	fmt.Fprintf(&b, "%s%t\n", BackgroundKey, cfg.Background)
	fmt.Fprintf(&b, "%s%t\n", SoundsKey, cfg.Sounds)
	fmt.Fprintf(&b, "%s%t\n", PasswordKey, cfg.Password)
	fmt.Fprintf(&b, "%s%d\n", StartTimeKey, cfg.StartTime)
	fmt.Fprintf(&b, "%s%t\n", UseMesaKey, cfg.UseMesa)
	fmt.Fprintf(&b, "%s%t\n", MultiInstanceKey, cfg.MultiInstance)
	fmt.Fprintf(&b, "%s%t\n", WidescreenKey, cfg.Widescreen)
	fmt.Fprintf(&b, "%s%d\n", FilterModeKey, cfg.FilterMode)
	fmt.Fprintf(&b, "%s%t\n", ScanlinesKey, cfg.Scanlines)
	fmt.Fprintf(&b, "%s%t\n", SilentTimeKey, cfg.SilentTime)
	fmt.Fprintf(&b, "%s%d\n", SilentStartKey, cfg.SilentStart)
	fmt.Fprintf(&b, "%s%d\n", SilentEndKey, cfg.SilentEnd)
	return b.String()
}

func cfgFileWrite(cfg *TConfig) {
	cfgLock.Lock()
	defer cfgLock.Unlock()

	f, err := os.Create(cfgFullPath())
	if err != nil {
		fmt.Println("WARN: failed to create file with err: ", err.Error())
		return
	}

	if _, err := f.WriteString(formatConfig(cfg)); err != nil {
		fmt.Println("WARN: failed to write config file with err: ", err.Error())
		_ = f.Close()
		return
	}
	if err := f.Close(); err != nil {
		fmt.Println("WARN: failed to close config file with err: ", err.Error())
	}
}

func cfgFileRead(cfg *TConfig) {
	cfgLock.Lock()
	defer cfgLock.Unlock()

	// Default values
	cfg.Background = true
	cfg.Sounds = true
	cfg.Password = false
	cfg.StartTime = 900
	cfg.UseMesa = false
	cfg.MultiInstance = false
	cfg.Widescreen = false
	cfg.FilterMode = 0
	cfg.Scanlines = false
	cfg.SilentTime = false
	cfg.SilentStart = 2200
	cfg.SilentEnd = 700

	f, err := os.Open(cfgFullPath())
	if err != nil {
		fmt.Println("WARN: failed to read file with err: ", err.Error())
		return
	}

	defer func() {
		_ = f.Close()
	}()

	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, CurrentDayKey) {
			currentDay, err := strconv.Atoi(line[len(CurrentDayKey):])
			if err != nil {
				fmt.Fprintln(os.Stderr, "failed to parse currentDay with err: ", err.Error())
			} else {
				cfg.CurrentDay = currentDay
			}
		} else if strings.HasPrefix(line, DateKey) {
			d, err := strconv.Atoi(line[len(DateKey):])
			if err != nil {
				fmt.Fprintln(os.Stderr, "failed to parse date with err: ", err.Error())
			} else {
				cfg.CurrentDate = d
			}
		} else if strings.HasPrefix(line, BackgroundKey) {
			parseConfigBool(line[len(BackgroundKey):], &cfg.Background)
		} else if strings.HasPrefix(line, SoundsKey) {
			parseConfigBool(line[len(SoundsKey):], &cfg.Sounds)
		} else if strings.HasPrefix(line, PasswordKey) {
			parseConfigBool(line[len(PasswordKey):], &cfg.Password)
		} else if strings.HasPrefix(line, StartTimeKey) {
			st, err := strconv.Atoi(line[len(StartTimeKey):])
			if err == nil && validHHMM(st) {
				cfg.StartTime = st
			}
		} else if strings.HasPrefix(line, UseMesaKey) {
			parseConfigBool(line[len(UseMesaKey):], &cfg.UseMesa)
		} else if strings.HasPrefix(line, MultiInstanceKey) {
			parseConfigBool(line[len(MultiInstanceKey):], &cfg.MultiInstance)
		} else if strings.HasPrefix(line, WidescreenKey) {
			parseConfigBool(line[len(WidescreenKey):], &cfg.Widescreen)
		} else if strings.HasPrefix(line, FilterModeKey) {
			fm, err := strconv.Atoi(line[len(FilterModeKey):])
			if err == nil && fm >= 0 && fm < 7 {
				cfg.FilterMode = fm
			}
		} else if strings.HasPrefix(line, ScanlinesKey) {
			parseConfigBool(line[len(ScanlinesKey):], &cfg.Scanlines)
		} else if strings.HasPrefix(line, SilentTimeKey) {
			parseConfigBool(line[len(SilentTimeKey):], &cfg.SilentTime)
		} else if strings.HasPrefix(line, SilentStartKey) {
			ss, err := strconv.Atoi(line[len(SilentStartKey):])
			if err == nil && validHHMM(ss) {
				cfg.SilentStart = ss
			}
		} else if strings.HasPrefix(line, SilentEndKey) {
			se, err := strconv.Atoi(line[len(SilentEndKey):])
			if err == nil && validHHMM(se) {
				cfg.SilentEnd = se
			}
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "failed to read config file:", err)
	}
}

func isSilentTime() bool {
	if !activeConfig.SilentTime {
		return false
	}
	now := time.Now()
	current := now.Hour()*100 + now.Minute()
	start := activeConfig.SilentStart
	end := activeConfig.SilentEnd

	if start == end {
		return true
	}
	if start < end {
		return current >= start && current < end
	}
	return current >= start || current < end
}

func validHHMM(v int) bool {
	if v < 0 || v > 2359 {
		return false
	}
	return v%100 < 60
}

func parseConfigBool(value string, dst *bool) {
	v, err := strconv.ParseBool(value)
	if err == nil {
		*dst = v
	}
}
