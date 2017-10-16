package main

import (
	"fmt"
	"log/syslog"
	rtdebug "runtime/debug"
	"os/user"
	"os"
)

var __logger__ *syslog.Writer = nil
var __debug__ bool = true

func _init() {
	if __logger__ == nil {
		u, _ := user.Current()
		if u.Username == "root" {
			__logger__, _ = syslog.New(syslog.LOG_DAEMON, "")
		} else {
			__logger__, _ = syslog.New(syslog.LOG_USER, "")
		}
	}
}

func Alert(format string, a ...interface{}) (error) {
	_init()
	return __logger__.Alert(fmt.Sprintf(format, a...))
}

func Crit(format string, a ...interface{}) (error) {
	_init()
	return __logger__.Crit(fmt.Sprintf(format, a...))
}

func Fatal(format string, a ...interface{}) {
	Crit(format, a...)
	os.Exit(1)
}

func Emerg(format string, a ...interface{}) (error) {
	_init()
	return __logger__.Emerg(fmt.Sprintf(format, a...))
}

func Error(format string, a ...interface{}) (error) {
	_init()
	return __logger__.Err(fmt.Sprintf(format, a...))
}

func Notice(format string, a ...interface{}) (error) {
	_init()
	return __logger__.Notice(fmt.Sprintf(format, a...))
}

func Warn(format string, a ...interface{}) (error) {
	_init()
	return __logger__.Warning(fmt.Sprintf(format, a...))
}

func Info(format string, a ...interface{}) (error) {
	_init()
	return __logger__.Info(fmt.Sprintf(format, a...))
}

func EnableDebug(shouldEnable bool) {
	__debug__ = shouldEnable
}

func Debug(format string, a ...interface{}) (error) {
	if __debug__ == false {
		return nil
	}
	_init()
	return __logger__.Debug(fmt.Sprintf(format, a...))
}

func Log(format string, a ...interface{}) (error) {
	return Info(fmt.Sprintf(format, a...))
}

func Trace(format string, a ...interface{}) (error) {
	tb := rtdebug.Stack()
	return Debug(fmt.Sprintf(format, a...) + fmt.Sprintf("\n %+v",tb))
}

