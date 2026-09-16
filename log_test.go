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

func TestIsRoutineTLSHandshake(t *testing.T) {
	if !isRoutineTLSHandshake("http: TLS handshake error from 192.168.0.153:55052: EOF") {
		t.Fatal("EOF handshake should be treated as routine")
	}
	if isRoutineTLSHandshake("http: TLS handshake error from 1.2.3.4:443: no certificates") {
		t.Fatal("unexpected TLS errors should still be reported")
	}
	if isRoutineTLSHandshake("listen tcp :443: bind: address already in use") {
		t.Fatal("non-TLS server errors should still be reported")
	}
}
