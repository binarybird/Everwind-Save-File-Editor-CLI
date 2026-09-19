package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunComponentsPlayerLocal(t *testing.T) {
	var buf bytes.Buffer
	if err := runComponents(&buf, "../../testdata/Player_Local.sav"); err != nil {
		t.Fatalf("runComponents: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	// header + 10 components
	if len(lines) != 11 {
		t.Fatalf("got %d lines, want 11 (1 header + 10 components):\n%s", len(lines), buf.String())
	}
}

func TestRunComponentsNoArrayField(t *testing.T) {
	var buf bytes.Buffer
	err := runComponents(&buf, "../../testdata/WorldInfo.sav")
	if err == nil {
		t.Fatal("expected an error: WorldInfo.sav has no top-level Components array")
	}
}
