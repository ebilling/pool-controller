//go:build !linux

package gpiocdev

import "unsafe"

// Stubs so the package, and anything that imports it, still builds on a
// development Mac. Open and Default reject the call before any of these run.

func sysOpen(string) (int, error)                 { return -1, ErrUnsupported }
func sysClose(int) error                          { return ErrUnsupported }
func sysIoctl(int, uintptr, unsafe.Pointer) error { return ErrUnsupported }
func sysPoll(int, int) (bool, error)              { return false, ErrUnsupported }
func sysRead(int, []byte) (int, error)            { return 0, ErrUnsupported }
func sysIsBusy(error) bool                        { return false }
func sysMonotonicNS() (uint64, error)             { return 0, ErrUnsupported }
