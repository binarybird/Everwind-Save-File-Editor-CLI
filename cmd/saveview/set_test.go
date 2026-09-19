package main

import (
	"os"
	"path/filepath"
	"testing"

	"skyversesave/gvas"
)

func TestRunSetString(t *testing.T) {
	out := filepath.Join(t.TempDir(), "out.sav")
	if err := runSet("../../testdata/WorldInfo.sav", "WorldName", "NewName", out); err != nil {
		t.Fatalf("runSet: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}
	f, err := gvas.Unmarshal(data)
	if err != nil {
		t.Fatalf("parsing written file: %v", err)
	}
	p, err := gvas.Lookup(f, "WorldName")
	if err != nil || p.Str == nil || *p.Str != "NewName" {
		t.Fatalf("WorldName after set: %+v, err=%v", p, err)
	}
}

func TestRunSetBool(t *testing.T) {
	out := filepath.Join(t.TempDir(), "out.sav")
	if err := runSet("../../testdata/WorldInfo.sav", "bNewGame", "true", out); err != nil {
		t.Fatalf("runSet: %v", err)
	}
	data, _ := os.ReadFile(out)
	f, err := gvas.Unmarshal(data)
	if err != nil {
		t.Fatalf("parsing written file: %v", err)
	}
	p, err := gvas.Lookup(f, "bNewGame")
	if err != nil || p.Bool == nil || *p.Bool != true {
		t.Fatalf("bNewGame after set: %+v, err=%v", p, err)
	}
}

func TestRunSetInt(t *testing.T) {
	out := filepath.Join(t.TempDir(), "out.sav")
	if err := runSet("../../testdata/WorldInfo.sav", "UDSData.CurrentTime.Month", "12", out); err != nil {
		t.Fatalf("runSet: %v", err)
	}
	data, _ := os.ReadFile(out)
	f, err := gvas.Unmarshal(data)
	if err != nil {
		t.Fatalf("parsing written file: %v", err)
	}
	p, err := gvas.Lookup(f, "UDSData.CurrentTime.Month")
	if err != nil || p.Int32 == nil || *p.Int32 != 12 {
		t.Fatalf("Month after set: %+v, err=%v", p, err)
	}
}

func TestRunSetInvalidValueErrors(t *testing.T) {
	out := filepath.Join(t.TempDir(), "out.sav")
	if err := runSet("../../testdata/WorldInfo.sav", "UDSData.CurrentTime.Month", "not-a-number", out); err == nil {
		t.Fatal("expected an error parsing a non-numeric value for an int property")
	}
}

func TestRunSetNeverTouchesInput(t *testing.T) {
	inputCopy := filepath.Join(t.TempDir(), "input.sav")
	original, err := os.ReadFile("../../testdata/WorldInfo.sav")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inputCopy, original, 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "out.sav")
	if err := runSet(inputCopy, "WorldName", "Whatever", out); err != nil {
		t.Fatalf("runSet: %v", err)
	}
	after, err := os.ReadFile(inputCopy)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(original) {
		t.Fatal("runSet modified the input file")
	}
}
