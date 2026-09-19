package gvas

import "testing"

type worldInfoSubset struct {
	WorldName   string
	GameVersion string
	NewGame     bool `save:"bNewGame"`
}

func TestDecodeIntoStruct(t *testing.T) {
	f, err := Unmarshal(readTestdata(t, "WorldInfo.sav"))
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	var w worldInfoSubset
	if err := DecodeInto(f, &w); err != nil {
		t.Fatalf("DecodeInto: %v", err)
	}
	if w.WorldName != "jff" {
		t.Errorf("WorldName = %q", w.WorldName)
	}
	if w.GameVersion != "0.4.715" {
		t.Errorf("GameVersion = %q", w.GameVersion)
	}
}

type playerLocalSubset struct {
	IslandID    string
	IsOnIsland  bool `save:"bIsOnIsland"`
}

func TestDecodeIntoOtherFile(t *testing.T) {
	f, err := Unmarshal(readTestdata(t, "Player_Local.sav"))
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	var p playerLocalSubset
	if err := DecodeInto(f, &p); err != nil {
		t.Fatalf("DecodeInto: %v", err)
	}
	if p.IsOnIsland != true {
		t.Errorf("IsOnIsland = %v", p.IsOnIsland)
	}
	if p.IslandID == "" {
		t.Error("IslandID is empty")
	}
}

func TestDecodeIntoRequiresPointer(t *testing.T) {
	f, err := Unmarshal(readTestdata(t, "WorldInfo.sav"))
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	var w worldInfoSubset
	if err := DecodeInto(f, w); err == nil {
		t.Fatal("expected an error passing a non-pointer, got nil")
	}
}

func TestEncodeIntoRoundTrip(t *testing.T) {
	f, err := Unmarshal(readTestdata(t, "WorldInfo.sav"))
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	var w worldInfoSubset
	if err := DecodeInto(f, &w); err != nil {
		t.Fatalf("DecodeInto: %v", err)
	}
	w.WorldName = "EncodedBack"
	w.NewGame = true
	if err := EncodeInto(f, &w); err != nil {
		t.Fatalf("EncodeInto: %v", err)
	}
	out, err := Marshal(f)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	f2, err := Unmarshal(out)
	if err != nil {
		t.Fatalf("re-Unmarshal: %v", err)
	}
	name := findProp(f2.Root, "WorldName")
	if name == nil || *name.Str != "EncodedBack" {
		t.Fatalf("WorldName after EncodeInto round-trip: %+v", name)
	}
	newGame := findProp(f2.Root, "bNewGame")
	if newGame == nil || *newGame.Bool != true {
		t.Fatalf("bNewGame after EncodeInto round-trip: %+v", newGame)
	}
}

func TestEncodeIntoUnmatchedFieldErrors(t *testing.T) {
	f, err := Unmarshal(readTestdata(t, "WorldInfo.sav"))
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	type withMissing struct {
		DoesNotExist string
	}
	v := withMissing{DoesNotExist: "x"}
	if err := EncodeInto(f, &v); err == nil {
		t.Fatal("expected an error encoding a field with no matching property")
	}
}
