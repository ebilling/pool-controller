package main

import "github.com/stianeikeland/go-rpio"

type PiPin interface {
	Input()
	Output()
	High()
	Low()
	Read() rpio.State
	PullUp()
	PullDown()
	PullOff()
}
