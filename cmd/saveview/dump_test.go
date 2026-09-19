package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunDumpWorldInfo(t *testing.T) {
	var buf bytes.Buffer
	if err := runDump(&buf, "../../testdata/WorldInfo.sav"); err != nil {
		t.Fatalf("runDump: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"WorldName", "GameVersion", "UDSData", "ItemPickupStruct"} {
		if !strings.Contains(out, want) {
			t.Errorf("dump output missing %q", want)
		}
	}
}

func TestRunDumpMissingFile(t *testing.T) {
	var buf bytes.Buffer
	if err := runDump(&buf, "../../testdata/does-not-exist.sav"); err == nil {
		t.Fatal("expected an error for a missing file")
	}
}
