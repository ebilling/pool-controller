package main

import (
	"testing"

	"github.com/brutella/hc/characteristic"
)

func TestThermostatModeMapping(t *testing.T) {
	cfg := &PersistedConfig{}
	if got := thermostatModeFromConfig(cfg); got != characteristic.TargetHeatingCoolingStateAuto {
		t.Fatalf("default mode %d, want Auto", got)
	}

	applyThermostatMode(cfg, characteristic.TargetHeatingCoolingStateOff)
	if cfg.Disabled || cfg.ThermostatMode != ThermostatOff ||
		thermostatModeFromConfig(cfg) != characteristic.TargetHeatingCoolingStateOff {
		t.Fatalf("off: %+v", cfg)
	}

	applyThermostatMode(cfg, characteristic.TargetHeatingCoolingStateHeat)
	if cfg.Disabled || cfg.ThermostatMode != ThermostatHeat {
		t.Fatalf("heat: %+v", cfg)
	}
	if thermostatModeFromConfig(cfg) != characteristic.TargetHeatingCoolingStateHeat {
		t.Fatal("expected heat")
	}

	applyThermostatMode(cfg, characteristic.TargetHeatingCoolingStateCool)
	if cfg.Disabled || cfg.ThermostatMode != ThermostatCool {
		t.Fatalf("cool: %+v", cfg)
	}

	applyThermostatMode(cfg, characteristic.TargetHeatingCoolingStateAuto)
	if cfg.Disabled || cfg.ThermostatMode != ThermostatAuto {
		t.Fatalf("auto: %+v", cfg)
	}
}

func TestCoolModeBlocksWarming(t *testing.T) {
	SetGpioProvider(NewTestPin)
	trp := NewTestRunPumps()
	trp.setConditions(30.0, 15.0, 50.0, 33.0, OFF)
	if !trp.ppc.shouldWarm() {
		t.Fatal("should warm before heat is disabled")
	}
	trp.ppc.config.cfg.ThermostatMode = ThermostatCool
	if trp.ppc.shouldWarm() {
		t.Fatal("Cool mode must block warming")
	}
}

func TestHeatModeBlocksCooling(t *testing.T) {
	SetGpioProvider(NewTestPin)
	trp := NewTestRunPumps()
	trp.setConditions(30.0, 40.0, 15.0, 10.0, OFF)
	if !trp.ppc.shouldCool() {
		t.Fatal("should cool before cool is disabled")
	}
	trp.ppc.config.cfg.ThermostatMode = ThermostatHeat
	if trp.ppc.shouldCool() {
		t.Fatal("Heat mode must block cooling")
	}
}

func TestLegacyDisableFlagsMigrateToMode(t *testing.T) {
	cfg := &PersistedConfig{HeatDisabled: true}
	normalizeThermostatMode(cfg)
	if cfg.ThermostatMode != ThermostatCool {
		t.Fatalf("legacy HeatDisabled migrated to %q, want cool", cfg.ThermostatMode)
	}
}
