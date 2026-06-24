package main

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
)

// r.c. This is not idiomatic Go, but a mostly direct C port of the original source.
// A second pass refactor can be done to fix this garbage.
var (
	// r.c. - added by me in case someone tries to run multiple instances of the screensaver.
	cfgLock      sync.Mutex
	activeConfig TConfig
)

type TConfig struct {
	CurrentDay  int
	CurrentDate int
	Background  bool
	Sounds      bool
	Password    bool
	StartTime   int
}

const (
	CfgFileName   = ".johnny_castaway_2026"
	CurrentDayKey = "currentDay="
	DateKey       = "date="
	BackgroundKey = "background="
	SoundsKey     = "sounds="
	PasswordKey   = "password="
	StartTimeKey  = "startTime="
)

func cfgFullPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic(fmt.Errorf("user home dir: %w", err))
	}

	fullPath := path.Join(homeDir, CfgFileName)
	return fullPath
}

func cfgFileWrite(cfg *TConfig) {
	cfgLock.Lock()
	defer cfgLock.Unlock()

	f, err := os.Create(cfgFullPath())
	if err != nil {
		fmt.Println("WARN: failed to create file with err: ", err.Error())
		return
	}
	defer func() {
		_ = f.Close()
	}()

	_, _ = fmt.Fprintf(f, "%s%d\n", CurrentDayKey, cfg.CurrentDay)
	_, _ = fmt.Fprintf(f, "%s%d\n", DateKey, cfg.CurrentDate)
	_, _ = fmt.Fprintf(f, "%s%t\n", BackgroundKey, cfg.Background)
	_, _ = fmt.Fprintf(f, "%s%t\n", SoundsKey, cfg.Sounds)
	_, _ = fmt.Fprintf(f, "%s%t\n", PasswordKey, cfg.Password)
	_, _ = fmt.Fprintf(f, "%s%d\n", StartTimeKey, cfg.StartTime)
}

func cfgFileRead(cfg *TConfig) {
	cfgLock.Lock()
	defer cfgLock.Unlock()

	// Default values
	cfg.Background = true
	cfg.Sounds = true
	cfg.Password = false
	cfg.StartTime = 900

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
			}
			cfg.CurrentDay = currentDay
		} else if strings.HasPrefix(line, DateKey) {
			d, err := strconv.Atoi(line[len(DateKey):])
			if err != nil {
				fmt.Fprintln(os.Stderr, "failed to parse date with err: ", err.Error())
			}
			cfg.CurrentDate = d
		} else if strings.HasPrefix(line, BackgroundKey) {
			cfg.Background = line[len(BackgroundKey):] == "true"
		} else if strings.HasPrefix(line, SoundsKey) {
			cfg.Sounds = line[len(SoundsKey):] == "true"
		} else if strings.HasPrefix(line, PasswordKey) {
			cfg.Password = line[len(PasswordKey):] == "true"
		} else if strings.HasPrefix(line, StartTimeKey) {
			st, err := strconv.Atoi(line[len(StartTimeKey):])
			if err == nil {
				cfg.StartTime = st
			}
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "reading standard input:", err)
	}
}
