package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRunJSONWorldInfo(t *testing.T) {
	var buf bytes.Buffer
	if err := runJSON(&buf, "../../testdata/WorldInfo.sav"); err != nil {
		t.Fatalf("runJSON: %v", err)
	}
	var decoded []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	found := false
	for _, entry := range decoded {
		if entry["name"] == "WorldName" {
			found = true
			if entry["value"] != "jff" {
				t.Errorf("WorldName value = %v", entry["value"])
			}
		}
	}
	if !found {
		t.Error("WorldName not present in JSON output")
	}
}

func TestRunJSONToFileWritesOutputFile(t *testing.T) {
	out := filepath.Join(t.TempDir(), "out.json")
	if err := runJSONToFile("../../testdata/WorldInfo.sav", out); err != nil {
		t.Fatalf("runJSONToFile: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("reading output file: %v", err)
	}
	var decoded []map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("output file is not valid JSON: %v\n%s", err, data)
	}
	found := false
	for _, entry := range decoded {
		if entry["name"] == "WorldName" {
			found = true
			if entry["value"] != "jff" {
				t.Errorf("WorldName value = %v", entry["value"])
			}
		}
	}
	if !found {
		t.Error("WorldName not present in JSON output file")
	}
}

func TestRunJSONNestedBlobExpanded(t *testing.T) {
	var buf bytes.Buffer
	if err := runJSON(&buf, "../../testdata/Player_Local.sav"); err != nil {
		t.Fatalf("runJSON: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("ComponentName")) {
		t.Error("expected nested component fields (e.g. ComponentName) to be expanded inline in JSON output")
	}
}

// findJSONProp walks a decoded jsonProp entry's "fields" (or a top-level
// list) looking for an entry with the given name.
func findJSONProp(entries []any, name string) (map[string]any, bool) {
	for _, e := range entries {
		m, ok := e.(map[string]any)
		if !ok {
			continue
		}
		if m["name"] == name {
			return m, true
		}
	}
	return nil, false
}

func fieldsOf(m map[string]any) []any {
	f, _ := m["fields"].([]any)
	return f
}

// TestRunJSONIntPropertyIsNumber confirms IntProperty leaves are emitted as
// real JSON numbers, not the dump-style string formatValue produces (e.g.
// "value": "12", not 12). WorldInfo.sav's UDSData.CurrentTime.Month is a
// known IntProperty.
func TestRunJSONIntPropertyIsNumber(t *testing.T) {
	var buf bytes.Buffer
	if err := runJSON(&buf, "../../testdata/WorldInfo.sav"); err != nil {
		t.Fatalf("runJSON: %v", err)
	}
	var decoded []any
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	uds, ok := findJSONProp(decoded, "UDSData")
	if !ok {
		t.Fatal("UDSData not present in JSON output")
	}
	curTime, ok := findJSONProp(fieldsOf(uds), "CurrentTime")
	if !ok {
		t.Fatal("UDSData.CurrentTime not present in JSON output")
	}
	month, ok := findJSONProp(fieldsOf(curTime), "Month")
	if !ok {
		t.Fatal("UDSData.CurrentTime.Month not present in JSON output")
	}
	if month["type"] != "IntProperty" {
		t.Fatalf("Month type = %v, want IntProperty", month["type"])
	}
	v, isNumber := month["value"].(float64)
	if !isNumber {
		t.Fatalf("Month value = %#v (%T), want a JSON number", month["value"], month["value"])
	}
	if v != float64(int32(v)) {
		t.Errorf("Month value = %v, want an integral value", v)
	}
}

// TestRunJSONBoolArrayExpanded confirms a populated scalar
// (ArrayValue.Bools) array round-trips as a real JSON array with the
// correct element count and values, rather than the useless
// "<N elements>" placeholder string. Player_Local.sav's
// PlayerMarkerSettings.Filters is a known BoolProperty-inner array with 8
// populated elements (all true).
func TestRunJSONBoolArrayExpanded(t *testing.T) {
	var buf bytes.Buffer
	if err := runJSON(&buf, "../../testdata/Player_Local.sav"); err != nil {
		t.Fatalf("runJSON: %v", err)
	}
	var decoded []any
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	settings, ok := findJSONProp(decoded, "PlayerMarkerSettings")
	if !ok {
		t.Fatal("PlayerMarkerSettings not present in JSON output")
	}
	filters, ok := findJSONProp(fieldsOf(settings), "Filters")
	if !ok {
		t.Fatal("PlayerMarkerSettings.Filters not present in JSON output")
	}
	arr, ok := filters["value"].([]any)
	if !ok {
		t.Fatalf("Filters value = %#v (%T), want a JSON array", filters["value"], filters["value"])
	}
	if len(arr) != 8 {
		t.Fatalf("Filters array has %d elements, want 8", len(arr))
	}
	for i, v := range arr {
		b, ok := v.(bool)
		if !ok || !b {
			t.Errorf("Filters[%d] = %#v, want true", i, v)
		}
	}
}
