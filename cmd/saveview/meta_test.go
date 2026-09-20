package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skyversesave/gvas"
)

func TestRunMetaDefaultOutputPath(t *testing.T) {
	dir := t.TempDir()
	savPath := filepath.Join(dir, "Player_Local.sav")
	data, err := os.ReadFile("../../testdata/Player_Local.sav")
	if err != nil {
		t.Fatalf("reading testdata: %v", err)
	}
	if err := os.WriteFile(savPath, data, 0o644); err != nil {
		t.Fatalf("writing sav copy: %v", err)
	}

	var out bytes.Buffer
	if err := runMeta(&out, savPath, ""); err != nil {
		t.Fatalf("runMeta: %v", err)
	}

	wantChecksum := gvas.MetaChecksum(data)
	metaPath := savPath + ".meta"
	metaData, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("reading %s: %v", metaPath, err)
	}
	if string(metaData) != wantChecksum {
		t.Errorf("%s content = %q, want %q", metaPath, metaData, wantChecksum)
	}
	if !strings.Contains(out.String(), wantChecksum) {
		t.Errorf("stdout = %q, want it to contain %q", out.String(), wantChecksum)
	}
}

func TestRunMetaExplicitOutputPath(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "custom.meta")
	var out bytes.Buffer
	if err := runMeta(&out, "../../testdata/WorldInfo.sav", outPath); err != nil {
		t.Fatalf("runMeta: %v", err)
	}
	data, err := os.ReadFile("../../testdata/WorldInfo.sav")
	if err != nil {
		t.Fatalf("reading testdata: %v", err)
	}
	metaData, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("reading %s: %v", outPath, err)
	}
	if string(metaData) != gvas.MetaChecksum(data) {
		t.Errorf("%s content = %q, want %q", outPath, metaData, gvas.MetaChecksum(data))
	}
}

func TestRunMetaMissingFile(t *testing.T) {
	var out bytes.Buffer
	if err := runMeta(&out, "/nonexistent/path.sav", ""); err == nil {
		t.Fatal("expected an error for a nonexistent input file")
	}
}
