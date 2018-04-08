package main

import "testing"

func checkErr(t *testing.T, err error) {
	if err != nil {
		t.Errorf("Unexpected Error: %v", err)
	}
}

func TestLog(t *testing.T) {
	t.Run("Alert", func(t *testing.T) {
		checkErr(t, Alert("testing %s", "alert"))
	})
	t.Run("Crit", func(t *testing.T) {
		checkErr(t, Crit("testing %s", "crit"))
	})
	t.Run("Emerg", func(t *testing.T) {
		checkErr(t, Emerg("testing %s", "emerg"))
	})
	t.Run("Error", func(t *testing.T) {
		checkErr(t, Error("testing %s", "error"))
	})
	t.Run("Notice", func(t *testing.T) {
		checkErr(t, Notice("testing %s", "notice"))
	})
	t.Run("Warn", func(t *testing.T) {
		checkErr(t, Warn("testing %s", "warn"))
	})
	t.Run("Info", func(t *testing.T) {
		checkErr(t, Info("testing %s", "info"))
	})
	t.Run("Log", func(t *testing.T) {
		checkErr(t, Log("testing %s", "log"))
	})
	t.Run("Trace", func(t *testing.T) {
		checkErr(t, Trace("testing %s", "trace"))
	})
}
