package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skyversesave/gvas"
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

// TestRunComponentsEmptyStructArray builds a synthetic save file whose
// Components property is a correctly-typed but zero-length
// ArrayProperty<StructProperty>. runComponents should print just the
// header line (0 rows), not report it as "not a struct array" -- an empty
// array.Structs slice is legitimately nil/empty, and is not distinguishable
// from "wrong inner type" by nil-checking Array.Structs alone.
func TestRunComponentsEmptyStructArray(t *testing.T) {
	w := gvas.NewWriter()
	w.WriteU8(0) // header byte

	w.WriteFString("Components")
	w.WriteFString("ArrayProperty")
	w.WriteFName(gvas.FName{Value: "StructProperty"})
	w.WriteFName(gvas.FName{Value: "SomeStruct"})
	w.WriteFName(gvas.FName{}) // inner struct package path
	w.WriteI32(0)              // ArrayIndex

	valueW := gvas.NewWriter()
	valueW.WriteI32(0) // element count: zero structs
	value := valueW.Bytes()

	w.WriteI32(int32(len(value))) // Size
	w.WriteU8(0)                  // GuidMarker (none)
	w.WriteBytes(value)

	w.WriteFString("None")
	w.WriteBytes([]byte{0, 0, 0, 0}) // footer

	path := filepath.Join(t.TempDir(), "empty_components.sav")
	if err := os.WriteFile(path, w.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := runComponents(&buf, path); err != nil {
		t.Fatalf("runComponents: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1 (header only):\n%s", len(lines), buf.String())
	}
}
