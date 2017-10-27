package main

import (
	"fmt"
	"log"
	"log/syslog"
	"os"
	"os/user"
	rtdebug "runtime/debug"
	"strings"
)

var __syslog__ *syslog.Writer = nil
var __debug__ bool = false
var __test__ bool = false
var priority syslog.Priority = syslog.LOG_USER

func _init() {
	if __syslog__ == nil {
		u, _ := user.Current()
		if u.Username == "root" {
			__syslog__, _ = syslog.New(syslog.LOG_DAEMON, "")
		}
	}
}

func Logger() *log.Logger {
	logger, _ := syslog.NewLogger(priority, log.LstdFlags)
	return logger
}

// Enables all calls to {#Debug()} that follow to go to syslog.
func EnableDebug() {
	__debug__ = true
}

// Disables all calls to {#Debug()} that follow.  No output will go syslog.
func DisableDebug() {
	__debug__ = false
}

// Enables Debug logging and sends all log output to Stdout
func LogTestMode() {
	__test__ = true
}

// Disables Debug logging and no longer sends output to Stdout
func EndTestMode() {
	__test__ = false
}

func check(err error, format string, a ...interface{}) error {
	s := fmt.Sprintf(format, a...)
	if err != nil {
		Error("%s: Error(%s)", s, err.Error())
		return err
	}
	return nil
}

func checkfatal(err error, format string, a ...interface{}) {
	s := fmt.Sprintf(format, a...)
	Fatal("%s: Error(%s)", s, err.Error())
}

func Alert(format string, a ...interface{}) error {
	if __test__ {
		_, err := fmt.Printf("Alert: "+format+"\n", a...)
		return err
	}
	_init()
	return __syslog__.Alert(fmt.Sprintf(format, a...))
}

func Crit(format string, a ...interface{}) error {
	if __test__ {
		_, err := fmt.Printf("Crit: "+format+"\n", a...)
		return err
	}
	_init()
	return __syslog__.Crit(fmt.Sprintf(format, a...))
}

func Fatal(format string, a ...interface{}) {
	Crit(format, a...)
	os.Exit(1)
}

func Emerg(format string, a ...interface{}) error {
	if __test__ {
		_, err := fmt.Printf("Emerg: "+format+"\n", a...)
		return err
	}
	_init()
	return __syslog__.Emerg(fmt.Sprintf(format, a...))
}

func Error(format string, a ...interface{}) error {
	if __test__ {
		_, err := fmt.Printf("Error: "+format+"\n", a...)
		return err
	}
	_init()
	return __syslog__.Err(fmt.Sprintf(format, a...))
}

func Notice(format string, a ...interface{}) error {
	if __test__ {
		_, err := fmt.Printf("Notice: "+format+"\n", a...)
		return err
	}
	_init()
	return __syslog__.Notice(fmt.Sprintf(format, a...))
}

func Warn(format string, a ...interface{}) error {
	if __test__ {
		_, err := fmt.Printf("Warn: "+format+"\n", a...)
		return err
	}
	_init()
	return __syslog__.Warning(fmt.Sprintf(format, a...))
}

func Info(format string, a ...interface{}) error {
	if __test__ {
		_, err := fmt.Printf("Info: "+format+"\n", a...)
		return err
	}
	_init()
	return __syslog__.Info(fmt.Sprintf(format, a...))
}

func Debug(format string, a ...interface{}) error {
	if __debug__ == false {
		return nil
	}
	if __test__ {
		_, err := fmt.Printf("Debug: "+format+"\n", a...)
		return err
	}
	_init()
	return __syslog__.Debug(fmt.Sprintf(format, a...))
}

func Log(format string, a ...interface{}) error {
	return Info(fmt.Sprintf(format, a...))
}

func traceback() string {
	return string(rtdebug.Stack())
}

func caller_traceback() string {
	s := strings.Split(string(rtdebug.Stack()), "\n")
	out := "{\n"
	for i := 7; i < len(s); i++ {
		if i%2 == 0 {
			out += s[i] + "\n"
		}
	}
	return out + "}"
}

func Trace(format string, a ...interface{}) error {
	return Debug(fmt.Sprintf(format, a...) + fmt.Sprintf(": TraceBack -> %s", caller_traceback()))
}
