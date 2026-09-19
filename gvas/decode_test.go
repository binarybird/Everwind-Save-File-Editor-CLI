package gvas

import (
	"bytes"
	"os"
	"testing"
)

func readTestdata(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("../testdata/" + name)
	if err != nil {
		t.Fatalf("reading testdata/%s: %v", name, err)
	}
	return data
}

func TestUnmarshalPlayerLocal(t *testing.T) {
	f, err := Unmarshal(readTestdata(t, "Player_Local.sav"))
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if f.HeaderByte != 0 {
		t.Errorf("HeaderByte = %d, want 0", f.HeaderByte)
	}
	if len(f.Footer) != 4 {
		t.Errorf("Footer len = %d, want 4", len(f.Footer))
	}

	island := findProp(f.Root, "IslandID")
	if island == nil || island.Type != "StrProperty" || island.Str == nil {
		t.Fatalf("IslandID: got %+v", island)
	}
	if (*island.Str)[0] != '-' {
		t.Errorf("IslandID = %q, want it to start with '-'", *island.Str)
	}

	onIsland := findProp(f.Root, "bIsOnIsland")
	if onIsland == nil || onIsland.Type != "BoolProperty" || onIsland.Bool == nil || *onIsland.Bool != true {
		t.Fatalf("bIsOnIsland: got %+v", onIsland)
	}

	comps := findProp(f.Root, "Components")
	if comps == nil || comps.Type != "ArrayProperty" || comps.Array == nil {
		t.Fatalf("Components: got %+v", comps)
	}
	if len(comps.Array.Structs) != 10 {
		t.Fatalf("Components has %d elements, want 10", len(comps.Array.Structs))
	}

	// Every component's Data.Data byte blob should have decoded as a nested file.
	for i, elem := range comps.Array.Structs {
		data := findProp(elem, "Data")
		if data == nil || data.Struct == nil {
			t.Fatalf("component %d: missing Data struct", i)
		}
		innerData := findProp(data.Struct, "Data")
		if innerData == nil || innerData.NestedFile == nil {
			t.Errorf("component %d: Data.Data did not decode as a nested file", i)
		}
	}
}

func TestUnmarshalWorldInfo(t *testing.T) {
	f, err := Unmarshal(readTestdata(t, "WorldInfo.sav"))
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	name := findProp(f.Root, "WorldName")
	if name == nil || name.Str == nil || *name.Str != "jff" {
		t.Fatalf("WorldName: got %+v", name)
	}
	version := findProp(f.Root, "GameVersion")
	if version == nil || version.Str == nil || *version.Str != "0.4.715" {
		t.Fatalf("GameVersion: got %+v", version)
	}

	uds := findProp(f.Root, "UDSData")
	if uds == nil || uds.Struct == nil {
		t.Fatalf("UDSData: got %+v", uds)
	}
	curTime := findProp(uds.Struct, "CurrentTime")
	if curTime == nil || curTime.Struct == nil {
		t.Fatalf("UDSData.CurrentTime: got %+v", curTime)
	}
	month := findProp(curTime.Struct, "Month")
	if month == nil || month.Int32 == nil {
		t.Fatalf("Month: got %+v", month)
	}

	// EnumProperty decode: at least one CustomModificators[].Type should
	// have decoded to a "Type::Value" string.
	itemPickups := findProp(f.Root, "ItemPickupStruct")
	if itemPickups == nil || itemPickups.Array == nil {
		t.Fatal("ItemPickupStruct missing")
	}
	foundEnum := false
	for _, elem := range itemPickups.Array.Structs {
		itemData := findProp(elem, "ItemData")
		if itemData == nil {
			continue
		}
		customMods := findProp(itemData.Struct, "CustomModificators")
		if customMods == nil || customMods.Array == nil {
			continue
		}
		for _, modElem := range customMods.Array.Structs {
			typ := findProp(modElem, "Type")
			if typ != nil && typ.Str != nil && *typ.Str != "" {
				foundEnum = true
			}
		}
	}
	if !foundEnum {
		t.Error("expected at least one populated CustomModificators[].Type enum value")
	}
}

func TestUnmarshalPlayerRemote(t *testing.T) {
	f, err := Unmarshal(readTestdata(t, "Player_Remote_021fa7902e50eeeb8c6ebdbe4fc922fe.sav"))
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if findProp(f.Root, "IslandID") == nil {
		t.Error("expected IslandID at top level")
	}
}

func TestUnmarshalTruncatedInputErrors(t *testing.T) {
	full := readTestdata(t, "WorldInfo.sav")
	_, err := Unmarshal(full[:100])
	if err == nil {
		t.Fatal("expected an error decoding a truncated file, got nil")
	}
}

// TestNestedBlobFalsePositiveNotPromoted builds a synthetic save file whose
// only property is an ArrayProperty<ByteProperty> with a 9-byte payload
// (00 00000000 AABBCCDD) that *parses* as a valid nested [header][empty
// property list][footer] blob -- because readPropertyList accepts an empty
// FString ("") as a list terminator, not just "None" -- but does not
// round-trip, since writePropertyList always writes a real "None"
// terminator on re-encode. Promoting this to NestedFile would silently
// corrupt the byte array on the next Marshal. See gvas/decode.go's
// ArrayProperty/SetProperty handling in decodeValue.
func TestNestedBlobFalsePositiveNotPromoted(t *testing.T) {
	payload := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0xAA, 0xBB, 0xCC, 0xDD}

	w := NewWriter()
	w.WriteU8(0) // top-level header byte

	// The one property: an ArrayProperty<ByteProperty> named "TestArr".
	w.WriteFString("TestArr")
	w.WriteFString("ArrayProperty")
	w.WriteFName(FName{Value: "ByteProperty"})
	w.WriteI32(0) // ArrayIndex

	valueW := NewWriter()
	valueW.WriteI32(int32(len(payload))) // array element count
	valueW.WriteBytes(payload)
	value := valueW.Bytes()

	w.WriteI32(int32(len(value))) // Size
	w.WriteU8(0)                  // GuidMarker (none)
	w.WriteBytes(value)

	w.WriteFString("None")           // property-list terminator
	w.WriteBytes([]byte{0, 0, 0, 0}) // top-level footer

	f, err := Unmarshal(w.Bytes())
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	p := findProp(f.Root, "TestArr")
	if p == nil || p.Array == nil {
		t.Fatalf("TestArr: got %+v", p)
	}
	if p.NestedFile != nil {
		t.Errorf("NestedFile should not be promoted for a false-positive nested parse, got %+v", p.NestedFile)
	}
	if !bytes.Equal(p.Array.Bytes, payload) {
		t.Errorf("Array.Bytes = %x, want unchanged %x", p.Array.Bytes, payload)
	}
}
