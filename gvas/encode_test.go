package gvas

import (
	"bytes"
	"testing"
)

func TestMarshalUnmarshalRoundTrip(t *testing.T) {
	for _, name := range []string{
		"Player_Local.sav",
		"Player_Remote_021fa7902e50eeeb8c6ebdbe4fc922fe.sav",
		"WorldInfo.sav",
	} {
		t.Run(name, func(t *testing.T) {
			original := readTestdata(t, name)
			f, err := Unmarshal(original)
			if err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			out, err := Marshal(f)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			if !bytes.Equal(out, original) {
				// Find the first differing byte to make failures diagnosable.
				n := len(out)
				if len(original) < n {
					n = len(original)
				}
				i := 0
				for i < n && out[i] == original[i] {
					i++
				}
				t.Fatalf("round-trip mismatch: len(out)=%d len(original)=%d, first diff at byte %d (out=%x original=%x)",
					len(out), len(original), i, out[max(0, i-4):min(len(out), i+4)], original[max(0, i-4):min(len(original), i+4)])
			}
		})
	}
}

// min/max are the Go 1.21+ builtins; go.mod (Task 1) is initialized
// against the installed toolchain (1.21+), so no local definitions needed.

func TestSetStringAndMarshal(t *testing.T) {
	original := readTestdata(t, "WorldInfo.sav")
	f, err := Unmarshal(original)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	worldName := findProp(f.Root, "WorldName")
	if worldName == nil {
		t.Fatal("WorldName not found")
	}
	if err := worldName.SetString("MyRenamedWorld"); err != nil {
		t.Fatalf("SetString: %v", err)
	}

	out, err := Marshal(f)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	f2, err := Unmarshal(out)
	if err != nil {
		t.Fatalf("re-Unmarshal after edit: %v", err)
	}
	got := findProp(f2.Root, "WorldName")
	if got == nil || got.Str == nil || *got.Str != "MyRenamedWorld" {
		t.Fatalf("WorldName after round-trip: %+v", got)
	}

	// Every other top-level property must be unaffected: same names, same
	// count, and every scalar/struct we can cheaply compare still decodes.
	if len(f2.Root) != len(f.Root) {
		t.Fatalf("top-level property count changed: got %d, want %d", len(f2.Root), len(f.Root))
	}
	version := findProp(f2.Root, "GameVersion")
	wantVersion := findProp(f.Root, "GameVersion")
	if version == nil || wantVersion == nil || *version.Str != *wantVersion.Str {
		t.Errorf("GameVersion changed unexpectedly: got %+v, want %+v", version, wantVersion)
	}
}

func TestSetOnWrongKindErrors(t *testing.T) {
	f, err := Unmarshal(readTestdata(t, "WorldInfo.sav"))
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	worldName := findProp(f.Root, "WorldName") // a StrProperty
	if err := worldName.SetBool(true); err == nil {
		t.Fatal("expected an error setting a bool on a StrProperty, got nil")
	}
}

// TestSetInsideNestedBlobPropagates edits a string that lives *inside* a
// component's embedded blob (Components[1].Data.Data, an
// ArrayProperty<ByteProperty> that decodes as a NestedFile -- see
// docs/FORMAT.md, "Nested blobs") and checks that re-marshaling recomputes
// every ancestor's Size correctly: NestedFile -> its ArrayProperty<Byte> ->
// the containing StructProperty -> the Components array element -> the
// top-level file length.
//
// NB: ComponentName (a sibling of Data at the ComponentSaveData level, per
// docs/FORMAT.md's Components diagram) is NOT inside the nested blob, so
// editing it would not exercise nested-blob propagation at all -- this test
// instead edits MarkerData.MarkerName, which decode confirms sits inside
// component 1's NestedFile.
func TestSetInsideNestedBlobPropagates(t *testing.T) {
	original := readTestdata(t, "Player_Local.sav")
	f, err := Unmarshal(original)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	comps := findProp(f.Root, "Components")
	component := comps.Array.Structs[1]
	data := findProp(component, "Data")
	if data == nil || data.Struct == nil {
		t.Fatal("component 1 missing Data struct")
	}
	innerData := findProp(data.Struct, "Data")
	if innerData == nil || innerData.NestedFile == nil {
		t.Fatal("component 1's Data.Data did not decode as a nested file")
	}
	markerData := findProp(innerData.NestedFile.Root, "MarkerData")
	if markerData == nil || markerData.Struct == nil {
		t.Fatal("component 1's nested file missing MarkerData")
	}
	markerName := findProp(markerData.Struct, "MarkerName")
	if markerName == nil || markerName.Str == nil {
		t.Fatal("component 1's MarkerData missing MarkerName")
	}
	if err := markerName.SetString("EditedMarkerName"); err != nil {
		t.Fatalf("SetString: %v", err)
	}

	out, err := Marshal(f)
	if err != nil {
		t.Fatalf("Marshal after nested edit: %v", err)
	}
	if len(out) == len(original) {
		t.Error("expected file length to change (new name is a different length than the original)")
	}
	f2, err := Unmarshal(out)
	if err != nil {
		t.Fatalf("re-Unmarshal: %v", err)
	}
	gotData := findProp(findProp(f2.Root, "Components").Array.Structs[1], "Data")
	if gotData == nil || gotData.Struct == nil {
		t.Fatal("re-decoded component 1 missing Data struct")
	}
	gotInnerData := findProp(gotData.Struct, "Data")
	if gotInnerData == nil || gotInnerData.NestedFile == nil {
		t.Fatal("re-decoded component 1's Data.Data did not decode as a nested file")
	}
	gotMarkerData := findProp(gotInnerData.NestedFile.Root, "MarkerData")
	if gotMarkerData == nil || gotMarkerData.Struct == nil {
		t.Fatal("re-decoded component 1's nested file missing MarkerData")
	}
	gotMarkerName := findProp(gotMarkerData.Struct, "MarkerName")
	if gotMarkerName == nil || gotMarkerName.Str == nil || *gotMarkerName.Str != "EditedMarkerName" {
		t.Fatalf("MarkerName after edit: %+v", gotMarkerName)
	}
}
