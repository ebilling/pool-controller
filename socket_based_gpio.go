package main

import (
	"fmt"
	"syscall"
	"os"
	"time"
)

type Edge int
const (
	NONE = iota
	RISING
	FALLING
	BOTH
)

func (e *Edge) String() string {
	switch *e {
	case RISING:
		return "rising"
	case FALLING:
		return "falling"
	case BOTH:
		return "both"
	}
	return "none"
}

type SocketButton struct {
	gpio       uint8
	callback   func ()
	bouncetime time.Duration
	file       *os.File
	done       chan bool
}

func newSocketButton(pin uint8, callback func ()) (*SocketButton) {
	b := SocketButton{
		gpio:         pin,
		callback:     callback,
		bouncetime:    250 * time.Millisecond,
		done:         make(chan bool),
	}
	return &b
}

func (b *SocketButton) Start() {
	err := b.export() ; if err != nil {
		Fatal("Export[%d] failed: %s", b.gpio, err) }
	err = b.set_direction(true) ; if err != nil {
		Fatal("Set direction[%d] failed: %s", b.gpio, err) }
	err = b.set_edge(RISING) ; if err != nil {
		Fatal("Set Edge[%d] failed: %s", b.gpio, err) }
	err = b.valueFile() ; if err != nil {
		Fatal("Open Value[%d] failed: %s", b.gpio, err) }
	
	go b.RunLoop()
}

func (b *SocketButton) RunLoop() {
	for true {
		select {
		case done := <- b.done:
			if done {break}
		}
		// block on change
		// if value high, run callback
	}
	// Cleanup
	b.file.Close()
	b.unexport()
}

func (b *SocketButton) Stop() {
	b.done <- true
}

func writeString(path, value string) error {
	file, err := os.OpenFile(path, os.O_WRONLY, 0644)
	if err != nil {
		Error("Error opening %s: %s", path, err.Error())
		return err
	}
	defer file.Close()
	_, err = file.WriteString(value)
	return err
}

func (b *SocketButton) valueFile() error {
	path := fmt.Sprintf("/sys/class/gpio/gpio%d/value", b.gpio)
	file, err := os.OpenFile(path, os.O_RDONLY | syscall.O_NONBLOCK, 0644)
	if err != nil {
		b.file = file
		Error("Could not get value file: %s", err.Error())
		return err
	}
	return nil
}

func (b *SocketButton) export() error {
	return writeString("/sys/class/gpio/export", fmt.Sprintf("%d", b.gpio))
}

func (b *SocketButton) unexport() error {
	return writeString("/sys/class/gpio/unexport", fmt.Sprintf("%d", b.gpio))

}

func (b *SocketButton) set_direction(in bool) error {
	path := fmt.Sprintf("/sys/class/gpio/gpio%d/direction", b.gpio)
	value := "in"
	if !in {
		value = "out"
	}
	var err error
	// Wait for permissions to be set in udev, retry for up to 1 sec
	for i := 0; i< 100 ; i++ {
		err = writeString(path, value)
		if err == nil {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return fmt.Errorf("Could not write direction (%s) in %s: %s",
		value, path, err.Error())
}

func (b *SocketButton) set_edge(edge Edge) error {
	path := fmt.Sprintf("/sys/class/gpio/gpio%d/edge", b.gpio)
	return writeString(path, edge.String())
}
