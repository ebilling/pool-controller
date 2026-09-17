//go:build linux

package gpiocdev

import (
	"errors"
	"unsafe"

	"golang.org/x/sys/unix"
)

// These are the only platform-specific operations in the package. Every one of
// them is a raw syscall: no fd is ever wrapped in an os.File, so the Go runtime
// poller is never involved.

func sysOpen(path string) (int, error) {
	return unix.Open(path, unix.O_RDWR|unix.O_CLOEXEC, 0)
}

func sysClose(fd int) error {
	return unix.Close(fd)
}

func sysIoctl(fd int, req uintptr, arg unsafe.Pointer) error {
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), req, uintptr(arg))
	if errno != 0 {
		return errno
	}
	return nil
}

// sysPoll waits for fd to become readable. timeoutMS follows poll(2): a
// negative value blocks indefinitely, zero returns immediately. A signal during
// the wait is reported as errInterrupted so the caller can retry with the time
// it has left.
func sysPoll(fd int, timeoutMS int) (bool, error) {
	fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
	n, err := unix.Poll(fds, timeoutMS)
	switch err {
	case nil:
	case unix.EINTR:
		return false, errInterrupted
	default:
		return false, err
	}
	return n > 0, nil
}

func sysRead(fd int, b []byte) (int, error) {
	return unix.Read(fd, b)
}

// sysIsBusy reports whether the line is already claimed by someone else.
func sysIsBusy(err error) bool {
	return errors.Is(err, unix.EBUSY)
}

// sysMonotonicNS reads CLOCK_MONOTONIC, the same clock the GPIO chardev uses to
// stamp line events unless EVENT_CLOCK_REALTIME or _HTE was requested. Values
// from here are therefore directly comparable to lineEvent.TimestampNS.
func sysMonotonicNS() (uint64, error) {
	var ts unix.Timespec
	if err := unix.ClockGettime(unix.CLOCK_MONOTONIC, &ts); err != nil {
		return 0, err
	}
	return uint64(ts.Sec)*1e9 + uint64(ts.Nsec), nil
}
