package main

type PiPin interface {
	Input()
	Output()
	High()
	Low()
	Read() GpioState
	PullUp()
	PullDown()
	PullOff()
}
