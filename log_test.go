package main

import (
	"testing"
)

func TestAlert(t *testing.T) {
	err := Alert("testing %s", "testval")
	if err != nil {
		t.Fatal(err)
	}
}

func TestCrit(t *testing.T) {
	err := Crit("testing %s", "testval")
	if err != nil {
		t.Fatal(err)
	}
}

func TestEmerg(t *testing.T) {
	err := Emerg("testing %s", "testval")
	if err != nil {
		t.Fatal(err)
	}
}

func TestError(t *testing.T) {
	err := Error("testing %s", "testval")
	if err != nil {
		t.Fatal(err)
	}
}

func TestNotice(t *testing.T) {
	err := Notice("testing %s", "testval")
	if err != nil {
		t.Fatal(err)
	}
}

func TestWarn(t *testing.T) {
	err := Warn("testing %s", "testval")
	if err != nil {
		t.Fatal(err)
	}
}

func TestInfo(t *testing.T) {
	err := Info("testing %s", "testval")
	if err != nil {
		t.Fatal(err)
	}
}

func TestDebug(t *testing.T) {
	err := Info("testing %s", "testval")
	if err != nil {
		t.Fatal(err)
	}
}

func TestLog(t *testing.T) {
	err := Log("testing %s", "testval")
	if err != nil {
		t.Fatal(err)
	}
}

func TestTrace(t *testing.T) {
	err := Trace("testing %s", "testval")
	if err != nil {
		t.Fatal(err)
	}
}
