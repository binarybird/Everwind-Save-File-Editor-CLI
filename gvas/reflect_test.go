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
	IslandID   string
	IsOnIsland bool `save:"bIsOnIsland"`
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

// componentDataSubset mirrors a Player_Local.sav Components[] element's
// Data.Data field: an ArrayProperty<ByteProperty>, i.e. a []byte in the Go
// struct. Together with playerLocalComponentsSubset below this exercises
// decodeSliceValue's reflect.Uint8 case against real testdata.
type componentDataSubset struct {
	Data []byte `save:"Data"`
}

type componentSubset struct {
	ComponentName string
	Data          componentDataSubset `save:"Data"`
}

type playerLocalComponentsSubset struct {
	Components []componentSubset
}

func TestDecodeIntoByteSlice(t *testing.T) {
	f, err := Unmarshal(readTestdata(t, "Player_Local.sav"))
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	var p playerLocalComponentsSubset
	if err := DecodeInto(f, &p); err != nil {
		t.Fatalf("DecodeInto: %v", err)
	}
	if len(p.Components) != 10 {
		t.Fatalf("Components has %d elements, want 10", len(p.Components))
	}
	for i, c := range p.Components {
		if len(c.Data.Data) == 0 {
			t.Errorf("component %d: Data.Data []byte is empty, want the component's byte blob", i)
		}
	}
}

// int64AndFloat64Slices is decoded against a synthetic property list (real
// testdata has no populated Int64Property/DoubleProperty arrays) to
// exercise decodeSliceValue's reflect.Int64 and reflect.Float64 cases.
type int64AndFloat64Slices struct {
	Int64Arr  []int64
	DoubleArr []float64
}

func TestDecodeIntoInt64AndFloat64Slices(t *testing.T) {
	w := NewWriter()
	w.WriteU8(0) // header byte

	writeScalarArrayProp(w, "Int64Arr", "Int64Property", 3, func(vw *Writer) {
		vw.WriteI64(1)
		vw.WriteI64(2)
		vw.WriteI64(-3)
	})
	writeScalarArrayProp(w, "DoubleArr", "DoubleProperty", 2, func(vw *Writer) {
		vw.WriteF64(1.5)
		vw.WriteF64(-2.25)
	})

	w.WriteFString("None")
	w.WriteBytes([]byte{0, 0, 0, 0}) // footer

	f, err := Unmarshal(w.Bytes())
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	var v int64AndFloat64Slices
	if err := DecodeInto(f, &v); err != nil {
		t.Fatalf("DecodeInto: %v", err)
	}
	wantInt64 := []int64{1, 2, -3}
	if len(v.Int64Arr) != len(wantInt64) {
		t.Fatalf("Int64Arr = %v, want %v", v.Int64Arr, wantInt64)
	}
	for i, want := range wantInt64 {
		if v.Int64Arr[i] != want {
			t.Errorf("Int64Arr[%d] = %d, want %d", i, v.Int64Arr[i], want)
		}
	}
	wantDouble := []float64{1.5, -2.25}
	if len(v.DoubleArr) != len(wantDouble) {
		t.Fatalf("DoubleArr = %v, want %v", v.DoubleArr, wantDouble)
	}
	for i, want := range wantDouble {
		if v.DoubleArr[i] != want {
			t.Errorf("DoubleArr[%d] = %g, want %g", i, v.DoubleArr[i], want)
		}
	}
}

// writeScalarArrayProp writes one ArrayProperty<innerType> property tag
// (name/type/InnerType/ArrayIndex/Size/GuidMarker) followed by its value
// (an element count then the elements written by writeElems) to w, in the
// shape readProperty/decodeValue/decodeArrayValue expect.
func writeScalarArrayProp(w *Writer, name, innerType string, count int32, writeElems func(*Writer)) {
	w.WriteFString(name)
	w.WriteFString("ArrayProperty")
	w.WriteFName(FName{Value: innerType})
	w.WriteI32(0) // ArrayIndex

	valueW := NewWriter()
	valueW.WriteI32(count)
	writeElems(valueW)
	value := valueW.Bytes()

	w.WriteI32(int32(len(value))) // Size
	w.WriteU8(0)                  // GuidMarker (none)
	w.WriteBytes(value)
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
