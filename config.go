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
	cfgLock sync.Mutex
)

type TConfig struct {
	CurrentDay  int
	CurrentDate int
}

const (
	CfgFileName   = ".johnny_castaway_2026"
	CurrentDayKey = "currentDay="
	DateKey       = "date="
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
	}
	defer func() {
		_ = f.Close()
	}()

	_, err = fmt.Fprintf(f, "%s%d\n", CurrentDayKey, cfg.CurrentDay)
	if err != nil {
		panic(fmt.Errorf("fprintln: %w", err))
	}

	_, err = fmt.Fprintf(f, "%s%d\n", DateKey, cfg.CurrentDate)
	if err != nil {
		panic(fmt.Errorf("fprintln: %w", err))
	}
}

func cfgFileRead(cfg *TConfig) {
	cfgLock.Lock()
	defer cfgLock.Unlock()

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
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "reading standard input:", err)
	}
}
