package gvas

import "testing"

func TestIsNativeStructName(t *testing.T) {
	for _, name := range []string{"Vector", "IntVector", "Rotator", "DateTime", "Timespan", "Color", "LinearColor", "Guid", "Transform"} {
		if !isNativeStructName(name) {
			t.Errorf("%q should be a native struct name", name)
		}
	}
	if isNativeStructName("WorldLocation") {
		t.Error("WorldLocation is a game-defined struct, not native")
	}
}

func TestDecodeEncodeVector(t *testing.T) {
	w := NewWriter()
	w.WriteF64(1.5)
	w.WriteF64(-2.5)
	w.WriteF64(100.0)
	raw := w.Bytes()

	nv := decodeNativeStruct("Vector", raw)
	if nv.Vector == nil {
		t.Fatal("expected decoded Vector")
	}
	if nv.Vector.X != 1.5 || nv.Vector.Y != -2.5 || nv.Vector.Z != 100.0 {
		t.Errorf("got %+v", nv.Vector)
	}

	back := encodeNativeValue(nv)
	if string(back) != string(raw) {
		t.Errorf("round-trip mismatch: got %x, want %x", back, raw)
	}
}

func TestDecodeEncodeIntVector(t *testing.T) {
	w := NewWriter()
	w.WriteI32(-248)
	w.WriteI32(-19)
	w.WriteI32(-1)
	raw := w.Bytes()

	nv := decodeNativeStruct("IntVector", raw)
	if nv.IntVector == nil || *nv.IntVector != (IntVector3{-248, -19, -1}) {
		t.Errorf("got %+v", nv.IntVector)
	}
	if string(encodeNativeValue(nv)) != string(raw) {
		t.Error("round-trip mismatch")
	}
}

func TestDecodeEncodeDateTime(t *testing.T) {
	w := NewWriter()
	w.WriteI64(639199138241280000)
	raw := w.Bytes()

	nv := decodeNativeStruct("DateTime", raw)
	if nv.DateTimeTicks == nil || *nv.DateTimeTicks != 639199138241280000 {
		t.Errorf("got %+v", nv.DateTimeTicks)
	}
	if string(encodeNativeValue(nv)) != string(raw) {
		t.Error("round-trip mismatch")
	}
}

func TestDecodeNativeStructUnknownSizeFallsBackToRaw(t *testing.T) {
	// Vector is documented as 24 bytes; feed it something else and it must
	// fall back to Raw rather than panic or misdecode.
	raw := []byte{1, 2, 3}
	nv := decodeNativeStruct("Vector", raw)
	if nv.Vector != nil {
		t.Fatal("expected fallback to Raw for malformed length")
	}
	if string(nv.Raw) != string(raw) {
		t.Errorf("Raw = %x, want %x", nv.Raw, raw)
	}
	if string(encodeNativeValue(nv)) != string(raw) {
		t.Error("round-trip through Raw fallback mismatch")
	}
}

func TestDecodeNativeStructUnrecognizedNameFallsBackToRaw(t *testing.T) {
	raw := []byte{9, 9, 9, 9}
	nv := decodeNativeStruct("Quat", raw) // Quat is native per docs/FORMAT.md's list but has no dedicated codec here
	if nv.StructName != "Quat" {
		t.Errorf("StructName = %q", nv.StructName)
	}
	if string(nv.Raw) != string(raw) {
		t.Error("expected raw fallback for Quat")
	}
}
