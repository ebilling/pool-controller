package main

import (
	"fmt"
	"log"
	"log/syslog"
	"os"
	"os/user"
	"path"
	"runtime"
	rtdebug "runtime/debug"
	"strings"
)

var (
	syslogWriter *syslog.Writer
	doDebug      bool
	priority     = syslog.LOG_USER
)

func init() {
	if syslogWriter == nil {
		u, _ := user.Current()
		if u.Username == "root" {
			syslogWriter, _ = syslog.New(syslog.LOG_DAEMON, "pool-controller")
		} else {
			syslogWriter, _ = syslog.New(syslog.LOG_USER, "pool-controller")
		}
	}
}

// NewLogger creates a logger
func NewLogger() *log.Logger {
	logger, _ := syslog.NewLogger(priority, log.LstdFlags)
	return logger
}

// EnableDebug - enables all calls to {#Debug()} that follow to go to syslog.
func EnableDebug() {
	doDebug = true
}

// DisableDebug - disables all calls to {#Debug()} that follow.  No output will go syslog.
func DisableDebug() {
	doDebug = false
}

func captureLine(format string) string {
	depth := 2 //exclude this function and the logging function
	_, file, line, ok := runtime.Caller(depth)
	_, file = path.Split(file)
	if !ok {
		return format
	}
	return fmt.Sprintf("%s:%d - %s", file, line, format)
}

// Alert sends a syslog message at the Alert level
func Alert(format string, a ...interface{}) error {
	format = captureLine(format)
	return syslogWriter.Alert(fmt.Sprintf(format, a...))
}

// Crit sends a syslog message at the Crit level
func Crit(format string, a ...interface{}) error {
	format = captureLine(format)
	return syslogWriter.Crit(fmt.Sprintf(format, a...))
}

// Fatal sends a syslog message at the Fatal level
func Fatal(format string, a ...interface{}) {
	format = captureLine(format)
	Crit(format, a...)
	os.Exit(1)
}

// Emerg sends a syslog message at the Emerg level
func Emerg(format string, a ...interface{}) error {
	format = captureLine(format)
	return syslogWriter.Emerg(fmt.Sprintf(format, a...))
}

// Error sends a syslog message at the Error level
func Error(format string, a ...interface{}) error {
	format = captureLine(format)
	return syslogWriter.Err(fmt.Sprintf(format, a...))
}

// Notice sends a syslog message at the Notice level
func Notice(format string, a ...interface{}) error {
	format = captureLine(format)
	return syslogWriter.Notice(fmt.Sprintf(format, a...))
}

// Warn sends a syslog message at the Warn level
func Warn(format string, a ...interface{}) error {
	format = captureLine(format)
	return syslogWriter.Warning(fmt.Sprintf(format, a...))
}

// Info sends a syslog message at the Info level
func Info(format string, a ...interface{}) error {
	format = captureLine(format)
	return syslogWriter.Info(fmt.Sprintf(format, a...))
}

// Debug sends a syslog message at the Debug level
func Debug(format string, a ...interface{}) error {
	if doDebug {
		format = captureLine(format)
		return syslogWriter.Debug(fmt.Sprintf(format, a...))
	}
	return nil
}

// Log sends a syslog message at the Info level
func Log(format string, a ...interface{}) error {
	format = captureLine(format)
	return Info(fmt.Sprintf(format, a...))
}

func callerTraceback() string {
	s := strings.Split(string(rtdebug.Stack()), "\n")
	out := "{\n"
	for i := 7; i < len(s); i++ {
		if i%2 == 0 {
			out += s[i] + "\n"
		}
	}
	return out + "}"
}

// Trace sends a syslog message and full stack trace at the Debug level
func Trace(format string, a ...interface{}) error {
	format = captureLine(format)
	return Debug(fmt.Sprintf(format, a...) + fmt.Sprintf(": TraceBack -> %s", callerTraceback()))
}

// TraceInfo sends a syslog message and full stack trace at the Info level
func TraceInfo(format string, a ...interface{}) error {
	format = captureLine(format)
	return Info(fmt.Sprintf(format, a...) + fmt.Sprintf(": TraceBack -> %s", callerTraceback()))
}
