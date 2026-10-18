package main

import (
 "fmt"
 "os"
 "path/filepath"
 "sync"
 "time"
)

var hdrTraceMu sync.Mutex
var hdrTraceFile *os.File
var hdrTraceEnabled = "" // -ldflags="-X main.hdrTraceEnabled=1"

func hdrTraceStart() {
 if hdrTraceEnabled != "1" { return }
 dir := filepath.Join(os.TempDir(), "JohnnyCastawayHDR")
 if err := os.MkdirAll(dir, 0700); err != nil { return }
 file, err := os.OpenFile(filepath.Join(dir, fmt.Sprintf("hdr-%d.log", os.Getpid())), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
 if err != nil { return }
 hdrTraceMu.Lock()
 hdrTraceFile = file
 hdrTraceMu.Unlock()
 hdrTrace("process.start args=%q", os.Args)
}

func hdrTrace(format string, args ...interface{}) {
 if hdrTraceEnabled != "1" { return }
 hdrTraceMu.Lock()
 defer hdrTraceMu.Unlock()
 if hdrTraceFile == nil { return }
 fmt.Fprintf(hdrTraceFile, "%s pid=%d %s\n", time.Now().Format("2006-01-02T15:04:05.000-07:00"), os.Getpid(), fmt.Sprintf(format, args...))
 _ = hdrTraceFile.Sync()
}

func hdrTraceStop() {
 if hdrTraceEnabled != "1" { return }
 hdrTrace("process.end")
 hdrTraceMu.Lock()
 defer hdrTraceMu.Unlock()
 if hdrTraceFile != nil {
  _ = hdrTraceFile.Close()
  hdrTraceFile = nil
 }
}
