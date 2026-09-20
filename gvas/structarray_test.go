package gvas

import "testing"

// findNamedStructArray recursively searches props for the first property
// named name whose Array is a StructProperty-inner array with exactly
// wantLen elements -- used to locate a real, representative "Items" array
// (an inventory slot) in testdata without hardcoding its exact path.
func findNamedStructArray(props []*Property, name string, wantLen int) *Property {
	for _, p := range props {
		if p.Name == name && p.Array != nil && p.Array.InnerType.Value == "StructProperty" && len(p.Array.Structs) == wantLen {
			return p
		}
		if p.Struct != nil {
			if found := findNamedStructArray(p.Struct, name, wantLen); found != nil {
				return found
			}
		}
		if p.NestedFile != nil {
			if found := findNamedStructArray(p.NestedFile.Root, name, wantLen); found != nil {
				return found
			}
		}
		if p.Array != nil {
			for _, elem := range p.Array.Structs {
				if found := findNamedStructArray(elem, name, wantLen); found != nil {
					return found
				}
			}
		}
	}
	return nil
}

func TestAppendStructElementRoundTrip(t *testing.T) {
	original := readTestdata(t, "Player_Local.sav")
	f, err := Unmarshal(original)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	target := findNamedStructArray(f.Root, "Items", 0)
	if target == nil {
		t.Fatal("no empty \"Items\" struct array found in Player_Local.sav")
	}

	newPath := "/Game/Data/Items/Resources_2500-2999/2553_IDA_RepairKit.2553_IDA_RepairKit"
	elem := []*Property{
		{Name: "BaseData", Type: "ObjectProperty", Str: &newPath},
	}
	if err := target.AppendStructElement(elem); err != nil {
		t.Fatalf("AppendStructElement: %v", err)
	}
	if len(target.Array.Structs) != 1 {
		t.Fatalf("len(Structs) = %d, want 1 after append", len(target.Array.Structs))
	}

	out, err := Marshal(f)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if len(out) == len(original) {
		t.Error("expected file length to change after appending an element")
	}

	f2, err := Unmarshal(out)
	if err != nil {
		t.Fatalf("re-Unmarshal: %v", err)
	}
	got := findNamedStructArray(f2.Root, "Items", 1)
	if got == nil {
		t.Fatal("no 1-element \"Items\" struct array found after round-trip")
	}
	base := findProp(got.Array.Structs[0], "BaseData")
	if base == nil || base.Str == nil || *base.Str != newPath {
		t.Fatalf("appended element's BaseData = %+v, want %q", base, newPath)
	}

	island := findProp(f2.Root, "IslandID")
	wantIsland := findProp(f.Root, "IslandID")
	if island == nil || wantIsland == nil || *island.Str != *wantIsland.Str {
		t.Errorf("IslandID changed unexpectedly: got %+v, want %+v", island, wantIsland)
	}
}

func TestRemoveStructElementRoundTrip(t *testing.T) {
	original := readTestdata(t, "Player_Local.sav")
	f, err := Unmarshal(original)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	target := findNamedStructArray(f.Root, "Items", 1)
	if target == nil {
		t.Fatal("no 1-element \"Items\" struct array found in Player_Local.sav")
	}

	if err := target.RemoveStructElement(0); err != nil {
		t.Fatalf("RemoveStructElement: %v", err)
	}
	if len(target.Array.Structs) != 0 {
		t.Fatalf("len(Structs) = %d, want 0 after remove", len(target.Array.Structs))
	}

	out, err := Marshal(f)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if len(out) == len(original) {
		t.Error("expected file length to change after removing an element")
	}

	f2, err := Unmarshal(out)
	if err != nil {
		t.Fatalf("re-Unmarshal: %v", err)
	}
	island := findProp(f2.Root, "IslandID")
	wantIsland := findProp(f.Root, "IslandID")
	if island == nil || wantIsland == nil || *island.Str != *wantIsland.Str {
		t.Errorf("IslandID changed unexpectedly: got %+v, want %+v", island, wantIsland)
	}
}

func TestStructArrayMethodsRejectNonStructArray(t *testing.T) {
	scalar := int32(5)
	p := &Property{Name: "NotAnArray", Type: "IntProperty", Int32: &scalar}
	if err := p.AppendStructElement(nil); err == nil {
		t.Error("AppendStructElement on a non-array property: expected an error")
	}
	if err := p.RemoveStructElement(0); err == nil {
		t.Error("RemoveStructElement on a non-array property: expected an error")
	}
}

func TestRemoveStructElementRejectsOutOfRange(t *testing.T) {
	f, err := Unmarshal(readTestdata(t, "Player_Local.sav"))
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	target := findNamedStructArray(f.Root, "Items", 1)
	if target == nil {
		t.Fatal("no 1-element \"Items\" struct array found")
	}
	if err := target.RemoveStructElement(5); err == nil {
		t.Error("RemoveStructElement(5) on a 1-element array: expected an error")
	}
	if err := target.RemoveStructElement(-1); err == nil {
		t.Error("RemoveStructElement(-1): expected an error")
	}
}
