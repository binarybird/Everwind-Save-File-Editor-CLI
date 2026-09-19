package gvas

import (
	"strings"
	"testing"
)

// findObjectProperty recursively searches props (descending through
// Struct, NestedFile, and Array.Structs) for the first ObjectProperty
// whose Str is non-nil and non-empty (a populated reference, not a null
// one), so tests exercise a real, representative BaseData-style value.
func findObjectProperty(props []*Property) *Property {
	for _, p := range props {
		if p.Type == "ObjectProperty" && p.Str != nil && *p.Str != "" {
			return p
		}
		if p.Struct != nil {
			if found := findObjectProperty(p.Struct); found != nil {
				return found
			}
		}
		if p.NestedFile != nil {
			if found := findObjectProperty(p.NestedFile.Root); found != nil {
				return found
			}
		}
		if p.Array != nil {
			for _, elem := range p.Array.Structs {
				if found := findObjectProperty(elem); found != nil {
					return found
				}
			}
		}
	}
	return nil
}

func TestObjectPropertyDecodesAsObjectPath(t *testing.T) {
	f, err := Unmarshal(readTestdata(t, "Player_Local.sav"))
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	p := findObjectProperty(f.Root)
	if p == nil {
		t.Fatal("no populated ObjectProperty found in Player_Local.sav")
	}
	if !strings.HasPrefix(*p.Str, "/Game/") {
		t.Errorf("ObjectProperty %q value = %q, want a /Game/... object path", p.Name, *p.Str)
	}
	if !strings.Contains(*p.Str, ".") {
		t.Errorf("ObjectProperty %q value = %q, want a PackagePath.AssetName path", p.Name, *p.Str)
	}
}

func TestObjectPropertyNullReferenceDecodesAsEmptyString(t *testing.T) {
	f, err := Unmarshal(readTestdata(t, "WorldInfo.sav"))
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	// WorldInfo.sav has hundreds of null ObjectProperty references
	// (empty FString, 4 zero bytes) — find one and confirm it decoded to
	// Str="" rather than falling back to Raw.
	var found *Property
	var walk func(props []*Property)
	walk = func(props []*Property) {
		for _, p := range props {
			if found != nil {
				return
			}
			if p.Type == "ObjectProperty" {
				found = p
				return
			}
			if p.Struct != nil {
				walk(p.Struct)
			}
			if p.Array != nil {
				for _, elem := range p.Array.Structs {
					walk(elem)
				}
			}
		}
	}
	walk(f.Root)
	if found == nil {
		t.Fatal("no ObjectProperty found in WorldInfo.sav")
	}
	if found.Str == nil {
		t.Fatalf("ObjectProperty %q: Str is nil, want a decoded (possibly empty) string; Raw=%x", found.Name, found.Raw)
	}
}

func TestSetObjectPropertyAndRoundTrip(t *testing.T) {
	original := readTestdata(t, "Player_Local.sav")
	f, err := Unmarshal(original)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	p := findObjectProperty(f.Root)
	if p == nil {
		t.Fatal("no populated ObjectProperty found")
	}
	newPath := "/Game/Data/Items/Resources_2500-2999/2553_IDA_RepairKit.2553_IDA_RepairKit_EDITED"
	if err := p.SetString(newPath); err != nil {
		t.Fatalf("SetString: %v", err)
	}

	out, err := Marshal(f)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if len(out) == len(original) {
		t.Error("expected file length to change (new path is a different length)")
	}

	f2, err := Unmarshal(out)
	if err != nil {
		t.Fatalf("re-Unmarshal: %v", err)
	}
	got := findObjectProperty(f2.Root)
	if got == nil || *got.Str != newPath {
		t.Fatalf("ObjectProperty after edit: %+v", got)
	}

	// A sibling top-level property must be unaffected by the edit.
	island := findProp(f2.Root, "IslandID")
	wantIsland := findProp(f.Root, "IslandID")
	if island == nil || wantIsland == nil || *island.Str != *wantIsland.Str {
		t.Errorf("IslandID changed unexpectedly: got %+v, want %+v", island, wantIsland)
	}
}

func TestParseFStringExact(t *testing.T) {
	cases := []struct {
		name string
		raw  []byte
		want string
		ok   bool
	}{
		{"empty string", []byte{0, 0, 0, 0}, "", true},
		{"ascii string", append([]byte{4, 0, 0, 0}, []byte("Foo\x00")...), "Foo", true},
		{"trailing garbage", append(append([]byte{4, 0, 0, 0}, []byte("Foo\x00")...), 0xFF), "", false},
		{"too short for length prefix", []byte{1, 2}, "", false},
		{"random non-fstring bytes", []byte{0x4b, 0x00, 0x00, 0x00, 0xDE, 0xAD, 0xBE, 0xEF}, "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := parseFStringExact(c.raw)
			if ok != c.ok {
				t.Fatalf("ok = %v, want %v (got %q)", ok, c.ok, got)
			}
			if ok && got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestObjectPropertyFallsBackToRawWhenNotAnFString(t *testing.T) {
	// Directly exercise decodeValue's ObjectProperty branch with bytes
	// that don't parse as a clean FString, confirming it falls back to
	// Raw rather than misdecoding or erroring.
	raw := []byte{0x4b, 0x00, 0x00, 0x00, 0xDE, 0xAD, 0xBE, 0xEF} // length prefix claims 75 bytes follow; only 4 present
	p := &Property{Type: "ObjectProperty"}
	r := NewReader(raw)
	if err := decodeValue(r, p, len(raw)); err != nil {
		t.Fatalf("decodeValue: %v", err)
	}
	if p.Str != nil {
		t.Fatalf("expected fallback to Raw, got Str=%q", *p.Str)
	}
	if string(p.Raw) != string(raw) {
		t.Errorf("Raw = %x, want %x", p.Raw, raw)
	}
	// And it must still round-trip byte-exact through encodeValue.
	back, err := encodeValue(p)
	if err != nil {
		t.Fatalf("encodeValue: %v", err)
	}
	if string(back) != string(raw) {
		t.Errorf("round-trip mismatch: got %x, want %x", back, raw)
	}
}
