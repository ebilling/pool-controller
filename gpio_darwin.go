package main

// OpenGPIO opens the GPIO interface.
func OpenGPIO() error {
	return nil
}

// CloseGPIO closes the GPIO interface.
func CloseGPIO() error {
	return nil
}

type testPin struct {
	number uint8
}

// NewGpio creates a new PiPin for a given gpio value.
func NewGpio(pin uint8) PiPin {
	return &testPin{
		number: pin,
	}
}

// Input sets the pin to be read from.
func (t *testPin) Input() {
	// Unimplemented
}

// Output sets the pin to be written to and sets the initial state.
func (t *testPin) Output(GpioState) {
	// Unimplemented

}

// Read returns the current state of the pin
func (t *testPin) Read() (GpioState, error) {
	// Unimplemented
	return nil
}

// Write sets the state of the pin
func (t *testPin) Write(GpioState) error {
	// Unimplemented
	return nil
}

// Pin returns the GPIO number of the pin.
func (t *testPin) Pin() uint8 {
	return t.number
}

// Close releases the resources related to the pin.
func (t *testPin) Close() {
	// Unimplemented
}

// Notifications returns a channel of notifications for the pin.
func (t *testPin) Notifications(Edge, GpioState) <-chan Notification {
	// Unimplemented
	return nil
}

// Watch registers a handler to be called when a notification is received.
func (t *testPin) Watch(NotificationHandler, Pull, Edge, GpioState) error {
	// Unimplemented
	return nil
}
