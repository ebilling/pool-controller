package main

import (
	"github.com/brutella/hc/accessory"
	"github.com/brutella/hc/characteristic"
)

const (
	// ThermOff is the state of the thermostat when it is off
	ThermOff = ThermoState(characteristic.CurrentHeatingCoolingStateOff)
	// ThermHeat is the state of the thermostat when it is heating
	ThermHeat = ThermoState(characteristic.CurrentHeatingCoolingStateHeat)
	// ThermCool is the state of the thermostat when it is cooling
	ThermCool = ThermoState(characteristic.CurrentHeatingCoolingStateCool)
)

// ThermoState is the state of the thermostat
type ThermoState int

// Thermostat reads a thermal resistance thermometer using the timings of a capacitor charge/discharge cycle
type Thermostat interface {
	Name() string
	Temperature() float64
	SetTargetTemperature(float64)
	TargetTemperatureCooling() float64
	TargetTemperatureHeating() float64
	Accessory() *accessory.Accessory
	ExpectedState() ThermoState
	SetState(ThermoState)
}

// PoolThermostat uses two thermometers to control a solar pool heater
type PoolThermostat struct {
	name      string
	accessory *accessory.Thermostat
	Pool      Thermometer
	Solar     Thermometer
}

// NewThermostat creates a new PoolThermostat
func NewThermostat(name string, manufacturer string, target float64, tolerance float64, pool Thermometer, solar Thermometer) Thermostat {
	acc := accessory.NewThermostat(AccessoryInfo(name, manufacturer), pool.Temperature(), target-tolerance, target+tolerance, 0.1)
	return &PoolThermostat{
		name:      name,
		accessory: acc,
		Pool:      pool,
		Solar:     solar,
	}
}

// Name returns the name of the PoolThermostat
func (t *PoolThermostat) Name() string {
	return t.name
}

// Temperature returns the current temperature
func (t *PoolThermostat) Temperature() float64 {
	return t.Pool.Temperature()
}

// SetTargetTemperature sets the target temperature
func (t *PoolThermostat) SetTargetTemperature(temp float64) {
	t.accessory.Thermostat.TargetTemperature.SetValue(temp)
}

// TargetTemperatureCooling returns the target temperature for cooling
func (t *PoolThermostat) TargetTemperatureCooling() float64 {
	return t.accessory.Thermostat.TargetTemperature.GetMinValue()
}

// TargetTemperatureHeating returns the target temperature for heating
func (t *PoolThermostat) TargetTemperatureHeating() float64 {
	return t.accessory.Thermostat.TargetTemperature.GetMaxValue()
}

// Accessory returns the Apple HomeKit accessory
func (t *PoolThermostat) Accessory() *accessory.Accessory {
	return t.accessory.Accessory
}

// ExpectedState returns the expected heating/cooling state
func (t *PoolThermostat) ExpectedState() ThermoState {
	if t.Pool.Temperature() < t.TargetTemperatureCooling() && t.Solar.Temperature() > HOTROOF {
		return ThermoState(characteristic.CurrentHeaterCoolerStateHeating)
	}
	if t.Pool.Temperature() > t.TargetTemperatureHeating() && t.Solar.Temperature() < COLDROOF {
		return ThermoState(characteristic.CurrentHeatingCoolingStateCool)
	}
	return ThermoState(characteristic.CurrentHeatingCoolingStateOff)
}

// SetState sets the heating/cooling state on homekit
func (t *PoolThermostat) SetState(state ThermoState) {
	t.accessory.Thermostat.CurrentHeatingCoolingState.SetValue(int(state))
}
