package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunGetScalar(t *testing.T) {
	var buf bytes.Buffer
	if err := runGet(&buf, "../../testdata/WorldInfo.sav", "WorldName"); err != nil {
		t.Fatalf("runGet: %v", err)
	}
	if strings.TrimSpace(buf.String()) != "jff" {
		t.Errorf("got %q", buf.String())
	}
}

func TestRunGetNestedPath(t *testing.T) {
	var buf bytes.Buffer
	if err := runGet(&buf, "../../testdata/Player_Local.sav", "Components[0].ComponentName"); err != nil {
		t.Fatalf("runGet: %v", err)
	}
	if strings.TrimSpace(buf.String()) == "" {
		t.Error("expected a non-empty component name")
	}
}

func TestRunGetMissingPath(t *testing.T) {
	var buf bytes.Buffer
	if err := runGet(&buf, "../../testdata/WorldInfo.sav", "DoesNotExist"); err == nil {
		t.Fatal("expected an error for a missing property path")
	}
}
