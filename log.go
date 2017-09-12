import (
	"log/syslog"
	"runtime/debug"
	"os/user"
)

__logger__ := nil

func alert(msg) {
	__logger__.Alert(msg)
}

func crit(msg) {
	__logger__.Crit(msg)
}

func emerg(msg) {
	__logger__.Emerg(msg)
}

func error(msg) {
	__logger__.Err(msg)
}

func notice(msg) {
	__logger__.Notice(msg)
}

func warn(msg) {
	__logger__.Warning(msg)
}

func info(msg) {
	__logger__.Info(msg)
}

func debug(msg) {
	__logger__.Debug(msg)
}

func log(msg) {
    info(msg)
}

func trace(msg) {
	tb := debug.Stack()
	debug(msg + "\n" + string(tb))
}

if __logger__ == nil {
	u, _ := user.Current()
	if u.Username == "root" {
		__logger__ = syslog.New(sysLog.LOG_DAEMON, "")
	} else {
		__logger__ = syslog.New(sysLog.LOG_USER, "")
	}
}
