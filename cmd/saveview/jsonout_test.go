package main

import (
	"bytes"
	"encoding/json"
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

func TestRunJSONNestedBlobExpanded(t *testing.T) {
	var buf bytes.Buffer
	if err := runJSON(&buf, "../../testdata/Player_Local.sav"); err != nil {
		t.Fatalf("runJSON: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("ComponentName")) {
		t.Error("expected nested component fields (e.g. ComponentName) to be expanded inline in JSON output")
	}
}
