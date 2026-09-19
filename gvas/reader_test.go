package gvas

import "testing"

func TestReadFString(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"empty", []byte{0, 0, 0, 0}, ""},
		{"ascii", append([]byte{9, 0, 0, 0}, []byte("IslandID\x00")...), "IslandID"},
		{
			"utf16",
			append([]byte{0xFE, 0xFF, 0xFF, 0xFF}, // -2 -> 2 UTF-16 code units incl. null terminator
				0x41, 0x00, 0x00, 0x00), // "A\0" as UTF-16LE
			"A",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := NewReader(c.data)
			got, err := r.ReadFString()
			if err != nil {
				t.Fatalf("ReadFString: %v", err)
			}
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
			if r.Pos() != len(c.data) {
				t.Errorf("pos = %d, want %d (didn't consume exactly the buffer)", r.Pos(), len(c.data))
			}
		})
	}
}

func TestReadFName(t *testing.T) {
	// Number=2 (int32 LE), then FString "Foo"
	data := append([]byte{2, 0, 0, 0}, append([]byte{4, 0, 0, 0}, []byte("Foo\x00")...)...)
	r := NewReader(data)
	got, err := r.ReadFName()
	if err != nil {
		t.Fatalf("ReadFName: %v", err)
	}
	if got.Number != 2 || got.Value != "Foo" {
		t.Errorf("got %+v, want {Number:2 Value:Foo}", got)
	}
}

func TestReadPrimitives(t *testing.T) {
	r := NewReader([]byte{
		0x2A,                   // u8 = 42
		0x01, 0x00, 0x00, 0x00, // i32 = 1
		0xFF, 0xFF, 0xFF, 0xFF, // i32 = -1 (read separately below)
	})
	if v, err := r.ReadU8(); err != nil || v != 42 {
		t.Fatalf("ReadU8 = %v, %v", v, err)
	}
	if v, err := r.ReadI32(); err != nil || v != 1 {
		t.Fatalf("ReadI32 = %v, %v", v, err)
	}
	if v, err := r.ReadI32(); err != nil || v != -1 {
		t.Fatalf("ReadI32 = %v, %v", v, err)
	}
}

func TestReadTruncated(t *testing.T) {
	r := NewReader([]byte{1, 2})
	if _, err := r.ReadI32(); err == nil {
		t.Fatal("expected error reading past end of buffer, got nil")
	}
}
