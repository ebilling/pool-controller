package main

import (
	"github.com/brutella/hc/accessory"
	"github.com/brutella/hc/characteristic"
)

const (
	thermostatMinC  = 10.0
	thermostatMaxC  = 38.0
	thermostatStepC = 0.5
)

// PoolThermostat is the HomeKit thermostat for the pool: current water
// temperature, a writable target, and Off/Heat/Cool/Auto.
type PoolThermostat struct {
	acc    *accessory.Thermostat
	config *Config
	ppc    *PoolPumpController
}

// NewPoolThermostat publishes the pool as a HomeKit thermostat.
func NewPoolThermostat(ppc *PoolPumpController) *PoolThermostat {
	target := ppc.config.cfg.Target
	acc := accessory.NewThermostat(
		AccessoryInfo("Pool", mftr),
		target,
		thermostatMinC,
		thermostatMaxC,
		thermostatStepC,
	)
	acc.Thermostat.CurrentTemperature.SetMinValue(0)
	acc.Thermostat.CurrentTemperature.SetMaxValue(100)
	acc.Thermostat.TemperatureDisplayUnits.SetValue(characteristic.TemperatureDisplayUnitsFahrenheit)
	t := &PoolThermostat{acc: acc, config: ppc.config, ppc: ppc}
	acc.Thermostat.TargetTemperature.OnValueRemoteUpdate(t.onTargetTemperature)
	acc.Thermostat.TargetHeatingCoolingState.OnValueRemoteUpdate(t.onTargetMode)
	t.Sync()
	return t
}

// Accessory is the HomeKit accessory to add to the HAP bridge.
func (t *PoolThermostat) Accessory() *accessory.Accessory {
	return t.acc.Accessory
}

func (t *PoolThermostat) onTargetTemperature(celsius float64) {
	t.ppc.mu.Lock()
	defer t.ppc.mu.Unlock()
	Info("HomeKit set pool target to %0.2fC", celsius)
	t.config.cfg.Target = celsius
	if err := t.config.Save(); err != nil {
		Error("could not persist HomeKit target: %s", err)
	}
	t.Sync()
}

func (t *PoolThermostat) onTargetMode(mode int) {
	t.ppc.mu.Lock()
	defer t.ppc.mu.Unlock()
	Info("HomeKit set pool thermostat mode to %d", mode)
	applyThermostatMode(t.config.cfg, mode)
	if err := t.config.Save(); err != nil {
		Error("could not persist HomeKit thermostat mode: %s", err)
	}
	t.Sync()
}

// Sync copies controller state onto the HomeKit characteristics.
func (t *PoolThermostat) Sync() {
	cur := t.ppc.runningTemp.Temperature()
	if cur < 0 {
		cur = 0
	}
	if cur > 100 {
		cur = 100
	}
	t.acc.Thermostat.CurrentTemperature.SetValue(cur)

	target := t.config.cfg.Target
	if target < thermostatMinC {
		target = thermostatMinC
	}
	if target > thermostatMaxC {
		target = thermostatMaxC
	}
	t.acc.Thermostat.TargetTemperature.SetValue(target)
	t.acc.Thermostat.TargetHeatingCoolingState.SetValue(thermostatModeFromConfig(t.config.cfg))
	t.acc.Thermostat.CurrentHeatingCoolingState.SetValue(t.currentHeatingCooling())
}

func (t *PoolThermostat) currentHeatingCooling() int {
	st := t.ppc.switches.State()
	if st < SOLAR {
		return characteristic.CurrentHeatingCoolingStateOff
	}
	if t.ppc.shouldCool() {
		return characteristic.CurrentHeatingCoolingStateCool
	}
	if t.ppc.shouldWarm() {
		return characteristic.CurrentHeatingCoolingStateHeat
	}
	// Solar is on but we are no longer outside the band; still circulating
	// through the panels counts as heat if the roof is warmer than the pool.
	if t.ppc.pumpTemp.Temperature() < t.ppc.roofTemp.Temperature() {
		return characteristic.CurrentHeatingCoolingStateHeat
	}
	return characteristic.CurrentHeatingCoolingStateCool
}

// thermostatModeName is the HomeKit target mode as shown in the web UI.
func thermostatModeName(cfg *PersistedConfig) string {
	switch thermostatModeFromConfig(cfg) {
	case characteristic.TargetHeatingCoolingStateOff:
		return "Off"
	case characteristic.TargetHeatingCoolingStateHeat:
		return "Heat"
	case characteristic.TargetHeatingCoolingStateCool:
		return "Cool"
	default:
		return "Auto"
	}
}

func thermostatModeFromConfig(cfg *PersistedConfig) int {
	if cfg.Disabled {
		return characteristic.TargetHeatingCoolingStateOff
	}
	switch configuredThermostatMode(cfg) {
	case ThermostatOff:
		return characteristic.TargetHeatingCoolingStateOff
	case ThermostatHeat:
		return characteristic.TargetHeatingCoolingStateHeat
	case ThermostatCool:
		return characteristic.TargetHeatingCoolingStateCool
	default:
		return characteristic.TargetHeatingCoolingStateAuto
	}
}

func applyThermostatMode(cfg *PersistedConfig, mode int) {
	switch mode {
	case characteristic.TargetHeatingCoolingStateOff:
		cfg.ThermostatMode = ThermostatOff
	case characteristic.TargetHeatingCoolingStateHeat:
		cfg.ThermostatMode = ThermostatHeat
	case characteristic.TargetHeatingCoolingStateCool:
		cfg.ThermostatMode = ThermostatCool
	default:
		cfg.ThermostatMode = ThermostatAuto
	}
}
