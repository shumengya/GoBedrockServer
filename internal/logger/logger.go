package logger

import (
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

var (
	mu      sync.Mutex
	debugOn = os.Getenv("GO_MC_DEBUG") != ""
	out     = os.Stderr
)

func init() {
	log.SetFlags(0)
	log.SetOutput(out)
}

func timestamp() string {
	return time.Now().Format("2006/01/02 15:04:05")
}

func caller(skip int) string {
	_, file, line, ok := runtime.Caller(skip)
	if !ok {
		return "???:0"
	}
	return fmt.Sprintf("%s:%d", filepath.Base(file), line)
}

func write(level, module, msg string, skip int) {
	mu.Lock()
	defer mu.Unlock()
	fmt.Fprintf(out, "%s %s %s [%s] %s\n", timestamp(), level, caller(skip), module, msg)
}

func Info(module, format string, args ...any) {
	write("INFO ", module, fmt.Sprintf(format, args...), 3)
}

func Warn(module, format string, args ...any) {
	write("WARN ", module, fmt.Sprintf(format, args...), 3)
}

func Error(module, format string, args ...any) {
	write("ERROR", module, fmt.Sprintf(format, args...), 3)
}

func Debug(module, format string, args ...any) {
	if !debugOn {
		return
	}
	write("DEBUG", module, fmt.Sprintf(format, args...), 3)
}

func Fatal(module, format string, args ...any) {
	write("FATAL", module, fmt.Sprintf(format, args...), 3)
	os.Exit(1)
}

func Recover(module string) {
	r := recover()
	if r == nil {
		return
	}
	stack := strings.TrimSpace(string(debug.Stack()))
	write("PANIC", module, fmt.Sprintf("%v\n%s", r, stack), 3)
}

func Go(module string, fn func()) {
	go func() {
		defer Recover(module)
		fn()
	}()
}

func HexPreview(b []byte, max int) string {
	if len(b) == 0 {
		return ""
	}
	if max <= 0 {
		max = 16
	}
	if len(b) > max {
		return hex.EncodeToString(b[:max]) + "..."
	}
	return hex.EncodeToString(b)
}

// HexDump is an alias used by protocol/raknet logs.
func HexDump(b []byte, max int) string { return HexPreview(b, max) }
