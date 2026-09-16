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
		facility := syslog.LOG_USER
		if u != nil && u.Username == "root" {
			facility = syslog.LOG_DAEMON
		}
		syslogWriter, _ = syslog.New(facility, "pool-controller")
	}
}

func writeLog(send func(string) error, format string, a ...interface{}) error {
	msg := fmt.Sprintf(format, a...)
	if syslogWriter == nil {
		_, err := fmt.Fprintln(os.Stderr, msg)
		return err
	}
	return send(msg)
}

// NewLogger creates a logger
func NewLogger() *log.Logger {
	logger, err := syslog.NewLogger(priority, log.LstdFlags)
	if err != nil || logger == nil {
		return log.New(os.Stderr, "pool-controller: ", log.LstdFlags)
	}
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
	return writeLog(syslogWriter.Alert, format, a...)
}

// Crit sends a syslog message at the Crit level
func Crit(format string, a ...interface{}) error {
	format = captureLine(format)
	return writeLog(syslogWriter.Crit, format, a...)
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
	return writeLog(syslogWriter.Emerg, format, a...)
}

// Error sends a syslog message at the Error level
func Error(format string, a ...interface{}) error {
	format = captureLine(format)
	return writeLog(syslogWriter.Err, format, a...)
}

// Notice sends a syslog message at the Notice level
func Notice(format string, a ...interface{}) error {
	format = captureLine(format)
	return writeLog(syslogWriter.Notice, format, a...)
}

// Warn sends a syslog message at the Warn level
func Warn(format string, a ...interface{}) error {
	format = captureLine(format)
	return writeLog(syslogWriter.Warning, format, a...)
}

// Info sends a syslog message at the Info level
func Info(format string, a ...interface{}) error {
	format = captureLine(format)
	return writeLog(syslogWriter.Info, format, a...)
}

// Debug sends a syslog message at the Debug level
func Debug(format string, a ...interface{}) error {
	if doDebug {
		format = captureLine(format)
		return writeLog(syslogWriter.Debug, format, a...)
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
