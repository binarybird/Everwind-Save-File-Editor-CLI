package gvas

import "testing"

func TestWriteReadFStringRoundTrip(t *testing.T) {
	cases := []string{"", "IslandID", "-15846-3123-1845", "héllo"} // last one forces UTF-16 path
	for _, s := range cases {
		w := NewWriter()
		w.WriteFString(s)
		r := NewReader(w.Bytes())
		got, err := r.ReadFString()
		if err != nil {
			t.Fatalf("round-trip %q: %v", s, err)
		}
		if got != s {
			t.Errorf("round-trip %q: got %q", s, got)
		}
		if r.Pos() != len(w.Bytes()) {
			t.Errorf("round-trip %q: reader left %d unread bytes", s, len(w.Bytes())-r.Pos())
		}
	}
}

func TestWriteFStringAsciiExactBytes(t *testing.T) {
	// Matches the exact encoding documented in docs/FORMAT.md for "IslandID".
	w := NewWriter()
	w.WriteFString("IslandID")
	want := append([]byte{9, 0, 0, 0}, []byte("IslandID\x00")...)
	got := w.Bytes()
	if len(got) != len(want) {
		t.Fatalf("got %d bytes, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("byte %d: got 0x%02x, want 0x%02x", i, got[i], want[i])
		}
	}
}

func TestWriteReadFNameRoundTrip(t *testing.T) {
	w := NewWriter()
	w.WriteFName(FName{Number: 2, Value: "EModificatorType"})
	r := NewReader(w.Bytes())
	got, err := r.ReadFName()
	if err != nil {
		t.Fatalf("ReadFName: %v", err)
	}
	if got.Number != 2 || got.Value != "EModificatorType" {
		t.Errorf("got %+v", got)
	}
}

func TestWriteReadPrimitivesRoundTrip(t *testing.T) {
	w := NewWriter()
	w.WriteU8(200)
	w.WriteI32(-12345)
	w.WriteI64(-123456789012345)
	w.WriteF32(3.5)
	w.WriteF64(-2.25)
	r := NewReader(w.Bytes())
	if v, _ := r.ReadU8(); v != 200 {
		t.Errorf("u8 got %v", v)
	}
	if v, _ := r.ReadI32(); v != -12345 {
		t.Errorf("i32 got %v", v)
	}
	if v, _ := r.ReadI64(); v != -123456789012345 {
		t.Errorf("i64 got %v", v)
	}
	if v, _ := r.ReadF32(); v != 3.5 {
		t.Errorf("f32 got %v", v)
	}
	if v, _ := r.ReadF64(); v != -2.25 {
		t.Errorf("f64 got %v", v)
	}
}
