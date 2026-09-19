# Skyverse Save Tool Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A Go library (`gvas`) that decodes/encodes the Skyverse `.sav` binary format losslessly, plus a `saveview` CLI to view and make scalar edits to save files.

**Architecture:** A generic `Property` tree (not hardcoded per-game-struct types) decoded by `gvas.Unmarshal` and re-encoded by `gvas.Marshal`, with a reflection-based typed convenience layer on top. The CLI walks the generic tree directly so it works on any save regardless of schema. `Marshal` always recomputes sizes bottom-up from the tree's actual current content, so editing a scalar's value and re-marshaling correctly propagates through every ancestor struct/array's `Size` field — including through embedded "nested blob" property lists (see `docs/FORMAT.md`, "Nested blobs").

**Tech Stack:** Go (standard library only — `encoding/binary` conventions implemented by hand since the format isn't quite standard `binary.Read`, `encoding/json` for CLI JSON output, `testing` + table-driven tests, `unicode/utf16` for the UTF-16 FString path).

**Spec:** `docs/superpowers/specs/2026-09-18-skyverse-save-tool-design.md` (and the binary format itself: `docs/FORMAT.md`)

## Global Constraints

- No third-party Go dependencies (spec: "No third-party dependencies").
- `gvas` must never hardcode a specific save's schema — only the wire-format rules from `docs/FORMAT.md` (spec: "generic — not tied to any one save's schema").
- `saveview set` must never overwrite the input file; it always requires an explicit `-o` output path (spec: "`set` requires `-o`... never overwrites the input save").
- Editing is scoped to scalar leaf values only for this plan — no adding/removing properties or array elements (spec: "Out of scope for this pass").
- Correctness bar: `Marshal(Unmarshal(data))` must equal `data` byte-for-byte for all three files in `testdata/` (spec: "Correctness bar").
- All three sample files already live in `testdata/` (`Player_Local.sav`, `Player_Remote_021fa7902e50eeeb8c6ebdbe4fc922fe.sav`, `WorldInfo.sav`) — committed in a prior session.

---

## Task 1: Project scaffolding

**Files:**
- Create: `go.mod`
- Create: `cmd/saveview/main.go`

**Interfaces:**
- Produces: module path `skyversesave`, a `main()` that prints usage and exits with status 2 when run with no arguments — later tasks add real subcommands here.

- [ ] **Step 1: Initialize the Go module**

Run: `cd /home/binarybird/Desktop/analysis/skyverse-save-tool && go mod init skyversesave`

Expected: creates `go.mod` containing `module skyversesave` and a `go` directive.

- [ ] **Step 2: Write a minimal main.go**

```go
package main

import (
	"fmt"
	"os"
)

func usage() {
	fmt.Fprintln(os.Stderr, `saveview - view and edit Skyverse .sav files

Usage:
  saveview dump <file>
  saveview json <file> [-o out.json]
  saveview get <file> <path>
  saveview set <file> <path> <value> -o <out>
  saveview components <file>`)
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}
```

- [ ] **Step 3: Verify it builds and runs**

Run: `go build ./... && ./saveview`

Expected: prints the usage text to stderr and exits with status 2 (check with `echo $?`).

- [ ] **Step 4: Commit**

```bash
git add go.mod cmd/saveview/main.go
git commit -m "chore: scaffold Go module and CLI entrypoint"
```

---

## Task 2: Binary primitives (reader.go + writer.go)

**Files:**
- Create: `gvas/reader.go`
- Create: `gvas/writer.go`
- Test: `gvas/reader_test.go`
- Test: `gvas/writer_test.go`

**Interfaces:**
- Produces:
  - `type FName struct { Number int32; Value string }`
  - `type Reader struct { ... }`, `func NewReader(data []byte) *Reader`
  - `func (r *Reader) Pos() int`, `func (r *Reader) Len() int`
  - `func (r *Reader) ReadBytes(n int) ([]byte, error)`
  - `func (r *Reader) ReadU8() (uint8, error)`
  - `func (r *Reader) ReadI32() (int32, error)`
  - `func (r *Reader) ReadI64() (int64, error)`
  - `func (r *Reader) ReadF32() (float32, error)`
  - `func (r *Reader) ReadF64() (float64, error)`
  - `func (r *Reader) ReadFString() (string, error)`
  - `func (r *Reader) ReadFName() (FName, error)`
  - `type Writer struct { ... }`, `func NewWriter() *Writer`, `func (w *Writer) Bytes() []byte`
  - `func (w *Writer) WriteBytes(b []byte)`, `WriteU8`, `WriteI32`, `WriteI64`, `WriteF32`, `WriteF64`, `WriteFString(string)`, `WriteFName(FName)`

- [ ] **Step 1: Write failing reader tests**

Create `gvas/reader_test.go`:

```go
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
```

- [ ] **Step 2: Run the reader tests to confirm they fail to compile**

Run: `go test ./gvas/... 2>&1 | head -20`

Expected: FAIL — `gvas` package doesn't exist yet / `NewReader` undefined.

- [ ] **Step 3: Implement reader.go**

```go
package gvas

import (
	"encoding/binary"
	"fmt"
	"math"
	"unicode/utf16"
)

// FName mirrors Unreal's flag+string encoding used for type/struct/package
// names inside property tag headers (never for ordinary property values).
// Number's exact meaning isn't fully understood (see docs/FORMAT.md) but it
// must always be preserved verbatim for byte-exact round-tripping.
type FName struct {
	Number int32
	Value  string
}

// Reader is a cursor over a save file's bytes.
type Reader struct {
	data []byte
	pos  int
}

func NewReader(data []byte) *Reader {
	return &Reader{data: data}
}

func (r *Reader) Pos() int { return r.pos }
func (r *Reader) Len() int { return len(r.data) }

func (r *Reader) ReadBytes(n int) ([]byte, error) {
	if n < 0 {
		return nil, fmt.Errorf("gvas: negative read length %d at offset %d", n, r.pos)
	}
	if r.pos+n > len(r.data) {
		return nil, fmt.Errorf("gvas: wanted %d bytes at offset %d, only %d remain", n, r.pos, len(r.data)-r.pos)
	}
	b := r.data[r.pos : r.pos+n]
	r.pos += n
	return b, nil
}

func (r *Reader) ReadU8() (uint8, error) {
	b, err := r.ReadBytes(1)
	if err != nil {
		return 0, err
	}
	return b[0], nil
}

func (r *Reader) ReadI32() (int32, error) {
	b, err := r.ReadBytes(4)
	if err != nil {
		return 0, err
	}
	return int32(binary.LittleEndian.Uint32(b)), nil
}

func (r *Reader) ReadI64() (int64, error) {
	b, err := r.ReadBytes(8)
	if err != nil {
		return 0, err
	}
	return int64(binary.LittleEndian.Uint64(b)), nil
}

func (r *Reader) ReadF32() (float32, error) {
	b, err := r.ReadBytes(4)
	if err != nil {
		return 0, err
	}
	return math.Float32frombits(binary.LittleEndian.Uint32(b)), nil
}

func (r *Reader) ReadF64() (float64, error) {
	b, err := r.ReadBytes(8)
	if err != nil {
		return 0, err
	}
	return math.Float64frombits(binary.LittleEndian.Uint64(b)), nil
}

// ReadFString reads Unreal's length-prefixed string encoding:
// 0 -> "", >0 -> that many ASCII/UTF-8 bytes including a trailing NUL,
// <0 -> -n UTF-16LE code units including a trailing NUL. See docs/FORMAT.md.
func (r *Reader) ReadFString() (string, error) {
	n, err := r.ReadI32()
	if err != nil {
		return "", err
	}
	if n == 0 {
		return "", nil
	}
	if n > 0 {
		b, err := r.ReadBytes(int(n))
		if err != nil {
			return "", err
		}
		if len(b) == 0 || b[len(b)-1] != 0 {
			return "", fmt.Errorf("gvas: ASCII FString at offset %d missing NUL terminator", r.pos-int(n))
		}
		return string(b[:len(b)-1]), nil
	}
	count := int(-n)
	b, err := r.ReadBytes(count * 2)
	if err != nil {
		return "", err
	}
	units := make([]uint16, count)
	for i := 0; i < count; i++ {
		units[i] = binary.LittleEndian.Uint16(b[i*2 : i*2+2])
	}
	if units[len(units)-1] != 0 {
		return "", fmt.Errorf("gvas: UTF-16 FString at offset %d missing NUL terminator", r.pos-count*2)
	}
	return string(utf16.Decode(units[:len(units)-1])), nil
}

// ReadFName reads the "FName-like" flag+string idiom used for type/struct
// names inside property tag headers. See docs/FORMAT.md.
func (r *Reader) ReadFName() (FName, error) {
	num, err := r.ReadI32()
	if err != nil {
		return FName{}, err
	}
	val, err := r.ReadFString()
	if err != nil {
		return FName{}, err
	}
	return FName{Number: num, Value: val}, nil
}
```

- [ ] **Step 4: Run reader tests, confirm pass**

Run: `go test ./gvas/... -run 'TestReadFString|TestReadFName|TestReadPrimitives|TestReadTruncated' -v`

Expected: all PASS.

- [ ] **Step 5: Write failing writer tests (round-trip against the reader)**

Create `gvas/writer_test.go`:

```go
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
```

- [ ] **Step 6: Run writer tests to confirm they fail to compile**

Run: `go test ./gvas/... 2>&1 | head -20`

Expected: FAIL — `NewWriter` undefined.

- [ ] **Step 7: Implement writer.go**

```go
package gvas

import (
	"encoding/binary"
	"math"
	"unicode/utf16"
)

// Writer accumulates bytes for a save file being re-encoded. It is the
// mirror image of Reader: every ReadX has a matching WriteX that produces
// bytes ReadX can parse back losslessly.
type Writer struct {
	buf []byte
}

func NewWriter() *Writer { return &Writer{} }

func (w *Writer) Bytes() []byte { return w.buf }

func (w *Writer) WriteBytes(b []byte) { w.buf = append(w.buf, b...) }

func (w *Writer) WriteU8(v uint8) { w.buf = append(w.buf, v) }

func (w *Writer) WriteI32(v int32) {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], uint32(v))
	w.buf = append(w.buf, b[:]...)
}

func (w *Writer) WriteI64(v int64) {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], uint64(v))
	w.buf = append(w.buf, b[:]...)
}

func (w *Writer) WriteF32(v float32) {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], math.Float32bits(v))
	w.buf = append(w.buf, b[:]...)
}

func (w *Writer) WriteF64(v float64) {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], math.Float64bits(v))
	w.buf = append(w.buf, b[:]...)
}

// WriteFString mirrors Reader.ReadFString. It writes the ASCII form when the
// string is pure ASCII (Unreal's own fast path), otherwise the UTF-16LE
// form, in both cases including a trailing NUL. See docs/FORMAT.md.
func (w *Writer) WriteFString(s string) {
	ascii := true
	for _, r := range s {
		if r == 0 || r > 127 {
			ascii = false
			break
		}
	}
	if s == "" {
		w.WriteI32(0)
		return
	}
	if ascii {
		w.WriteI32(int32(len(s) + 1))
		w.WriteBytes([]byte(s))
		w.WriteU8(0)
		return
	}
	units := utf16.Encode([]rune(s))
	units = append(units, 0)
	w.WriteI32(-int32(len(units)))
	for _, u := range units {
		var b [2]byte
		binary.LittleEndian.PutUint16(b[:], u)
		w.WriteBytes(b[:])
	}
}

func (w *Writer) WriteFName(f FName) {
	w.WriteI32(f.Number)
	w.WriteFString(f.Value)
}
```

- [ ] **Step 8: Run all gvas tests, confirm pass**

Run: `go test ./gvas/... -v`

Expected: every test in both files PASSes.

- [ ] **Step 9: Commit**

```bash
git add gvas/reader.go gvas/writer.go gvas/reader_test.go gvas/writer_test.go
git commit -m "feat(gvas): binary reader/writer primitives for FString and FName"
```

---

## Task 3: Native struct codecs (nativestruct.go)

**Files:**
- Create: `gvas/nativestruct.go`
- Test: `gvas/nativestruct_test.go`

**Interfaces:**
- Consumes: `Reader`/`Writer` from Task 2.
- Produces:
  - `type Vector3 struct { X, Y, Z float64 }`
  - `type IntVector3 struct { X, Y, Z int32 }`
  - `type Rotator3 struct { Pitch, Yaw, Roll float64 }`
  - `type ColorBytes struct { B, G, R, A byte }`
  - `type LinearColorFloats struct { R, G, B, A float32 }`
  - `type NativeValue struct { StructName string; Vector *Vector3; IntVector *IntVector3; Rotator *Rotator3; DateTimeTicks *int64; TimespanTicks *int64; Color *ColorBytes; LinearColor *LinearColorFloats; Raw []byte }`
  - `func isNativeStructName(name string) bool`
  - `func decodeNativeStruct(name string, raw []byte) *NativeValue`
  - `func encodeNativeValue(n *NativeValue) []byte`

- [ ] **Step 1: Write failing tests**

Create `gvas/nativestruct_test.go`:

```go
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
```

- [ ] **Step 2: Run tests, confirm compile failure**

Run: `go test ./gvas/... 2>&1 | head -20`

Expected: FAIL — `decodeNativeStruct` undefined.

- [ ] **Step 3: Implement nativestruct.go**

```go
package gvas

// Vector3, IntVector3, Rotator3, ColorBytes and LinearColorFloats mirror
// the fixed native layouts documented in docs/FORMAT.md ("Native
// (non-nested) struct types"). Color/LinearColor sizes there are marked
// "inferred" (no populated sample existed during reverse engineering) but
// are included for forward compatibility; if a real save disagrees on
// byte length, decodeNativeStruct falls back to Raw rather than misdecode.
type Vector3 struct{ X, Y, Z float64 }
type IntVector3 struct{ X, Y, Z int32 }
type Rotator3 struct{ Pitch, Yaw, Roll float64 }
type ColorBytes struct{ B, G, R, A byte }
type LinearColorFloats struct{ R, G, B, A float32 }

// NativeValue holds the decoded value of a StructProperty whose StructName
// is a recognized engine-native type — i.e. one serialized as packed bytes
// rather than a nested property list. Exactly one of the typed pointer
// fields is set on success; Raw is the fallback for an unrecognized name
// or an unexpected byte length, and is always what gets re-encoded when
// none of the typed fields apply.
type NativeValue struct {
	StructName  string
	Vector      *Vector3
	IntVector   *IntVector3
	Rotator     *Rotator3
	DateTimeTicks *int64
	TimespanTicks *int64
	Color       *ColorBytes
	LinearColor *LinearColorFloats
	Raw         []byte
}

var nativeStructNames = map[string]bool{
	"Vector": true, "Vector2D": true, "Vector4": true, "Quat": true,
	"Rotator": true, "Guid": true, "DateTime": true, "Timespan": true,
	"Color": true, "LinearColor": true, "IntPoint": true, "IntVector": true,
	"Box": true, "Box2D": true, "Transform": true, "Plane": true, "Matrix": true,
}

func isNativeStructName(name string) bool { return nativeStructNames[name] }

// decodeNativeStruct never fails: an unrecognized name or a byte length
// that doesn't match the expected layout always yields a Raw fallback, per
// docs/FORMAT.md's point that Size alone is always enough to skip safely.
func decodeNativeStruct(name string, raw []byte) *NativeValue {
	nv := &NativeValue{StructName: name}
	switch name {
	case "Vector":
		if len(raw) == 24 {
			r := NewReader(raw)
			x, _ := r.ReadF64()
			y, _ := r.ReadF64()
			z, _ := r.ReadF64()
			nv.Vector = &Vector3{x, y, z}
			return nv
		}
	case "IntVector":
		if len(raw) == 12 {
			r := NewReader(raw)
			x, _ := r.ReadI32()
			y, _ := r.ReadI32()
			z, _ := r.ReadI32()
			nv.IntVector = &IntVector3{x, y, z}
			return nv
		}
	case "Rotator":
		if len(raw) == 24 {
			r := NewReader(raw)
			p, _ := r.ReadF64()
			yaw, _ := r.ReadF64()
			roll, _ := r.ReadF64()
			nv.Rotator = &Rotator3{p, yaw, roll}
			return nv
		}
	case "DateTime":
		if len(raw) == 8 {
			r := NewReader(raw)
			v, _ := r.ReadI64()
			nv.DateTimeTicks = &v
			return nv
		}
	case "Timespan":
		if len(raw) == 8 {
			r := NewReader(raw)
			v, _ := r.ReadI64()
			nv.TimespanTicks = &v
			return nv
		}
	case "Color":
		if len(raw) == 4 {
			nv.Color = &ColorBytes{B: raw[0], G: raw[1], R: raw[2], A: raw[3]}
			return nv
		}
	case "LinearColor":
		if len(raw) == 16 {
			r := NewReader(raw)
			cr, _ := r.ReadF32()
			cg, _ := r.ReadF32()
			cb, _ := r.ReadF32()
			ca, _ := r.ReadF32()
			nv.LinearColor = &LinearColorFloats{cr, cg, cb, ca}
			return nv
		}
	}
	nv.Raw = raw
	return nv
}

func encodeNativeValue(n *NativeValue) []byte {
	w := NewWriter()
	switch {
	case n.Vector != nil:
		w.WriteF64(n.Vector.X)
		w.WriteF64(n.Vector.Y)
		w.WriteF64(n.Vector.Z)
	case n.IntVector != nil:
		w.WriteI32(n.IntVector.X)
		w.WriteI32(n.IntVector.Y)
		w.WriteI32(n.IntVector.Z)
	case n.Rotator != nil:
		w.WriteF64(n.Rotator.Pitch)
		w.WriteF64(n.Rotator.Yaw)
		w.WriteF64(n.Rotator.Roll)
	case n.DateTimeTicks != nil:
		w.WriteI64(*n.DateTimeTicks)
	case n.TimespanTicks != nil:
		w.WriteI64(*n.TimespanTicks)
	case n.Color != nil:
		w.WriteBytes([]byte{n.Color.B, n.Color.G, n.Color.R, n.Color.A})
	case n.LinearColor != nil:
		w.WriteF32(n.LinearColor.R)
		w.WriteF32(n.LinearColor.G)
		w.WriteF32(n.LinearColor.B)
		w.WriteF32(n.LinearColor.A)
	default:
		return n.Raw
	}
	return w.Bytes()
}
```

- [ ] **Step 4: Run tests, confirm pass**

Run: `go test ./gvas/... -run Native -v`

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add gvas/nativestruct.go gvas/nativestruct_test.go
git commit -m "feat(gvas): native struct codecs (Vector, IntVector, DateTime, ...)"
```

---

## Task 4: Property tree types + Unmarshal (property.go + decode.go)

This is the core parser: the full algorithm from `docs/FORMAT.md`.

**Files:**
- Create: `gvas/property.go`
- Create: `gvas/decode.go`
- Test: `gvas/decode_test.go`

**Interfaces:**
- Consumes: `Reader`, `FName` (Task 2); `NativeValue`, `isNativeStructName`, `decodeNativeStruct` (Task 3).
- Produces:
  - `type ExtraHeader struct { StructName, PackagePath, EnumName, EnumTypeName, EnumPackagePath, UnderlyingType, InnerType, InnerStructName, InnerStructPackagePath, KeyType, ValueType FName }`
  - `type ArrayValue struct { InnerType FName; Structs [][]*Property; Bools []bool; Ints []int32; Int64s []int64; Floats []float32; Doubles []float64; Strings []string; Bytes []byte; RawCount int32; RawElements []byte }`
  - `type Property struct { Name, Type string; ArrayIndex int32; Extra ExtraHeader; GuidMarker uint8; Guid []byte; Bool *bool; Str *string; Int32 *int32; Int64 *int64; Float32 *float32; Float64 *float64; Byte *uint8; Struct []*Property; Native *NativeValue; Array *ArrayValue; Raw []byte; NestedFile *File }`
  - `type File struct { HeaderByte byte; Root []*Property; Footer []byte }`
  - `func Unmarshal(data []byte) (*File, error)`

- [ ] **Step 1: Define the tree types in property.go**

```go
package gvas

// ExtraHeader holds the type-specific fields that appear in a property tag
// header between Type and ArrayIndex. See docs/FORMAT.md, "Type-specific
// extra header". Only the fields relevant to a given Property.Type are
// populated; the rest are zero values.
type ExtraHeader struct {
	StructName             FName // StructProperty
	PackagePath            FName // StructProperty
	EnumName               FName // ByteProperty
	EnumTypeName            FName // EnumProperty
	EnumPackagePath          FName // EnumProperty
	UnderlyingType            FName // EnumProperty
	InnerType                  FName // ArrayProperty / SetProperty
	InnerStructName              FName // ArrayProperty / SetProperty, when InnerType=="StructProperty"
	InnerStructPackagePath        FName // ArrayProperty / SetProperty, when InnerType=="StructProperty"
	KeyType                         FName // MapProperty
	ValueType                        FName // MapProperty
}

// ArrayValue holds the decoded elements of an ArrayProperty/SetProperty.
// Exactly one of Structs/Bools/Ints/Int64s/Floats/Doubles/Strings/Bytes is
// populated for a recognized InnerType; RawCount+RawElements is the
// fallback for anything else (e.g. ObjectProperty elements), preserving
// the exact bytes for round-tripping. See docs/FORMAT.md, "Array/Set value
// format".
type ArrayValue struct {
	InnerType FName

	Structs [][]*Property // one property list per element, when InnerType.Value == "StructProperty"
	Bools   []bool
	Ints    []int32
	Int64s  []int64
	Floats  []float32
	Doubles []float64
	Strings []string
	Bytes   []byte // one byte per element, when InnerType.Value == "ByteProperty"

	RawCount    int32
	RawElements []byte
}

// Property is one entry in a property list: a tagged name/type/value
// triple. Exactly one of the typed value fields is populated, chosen by
// Type. See docs/FORMAT.md, "Property tag".
type Property struct {
	Name       string
	Type       string
	ArrayIndex int32
	Extra      ExtraHeader

	// GuidMarker and Guid are meaningless for BoolProperty (which has
	// neither — see docs/FORMAT.md). For every other type, GuidMarker is
	// the raw marker byte as read; Guid is set only when GuidMarker == 1.
	GuidMarker uint8
	Guid       []byte

	Bool    *bool
	Str     *string // StrProperty, NameProperty, and EnumProperty (formatted "Type::Value")
	Int32   *int32
	Int64   *int64
	Float32 *float32
	Float64 *float64
	Byte    *uint8 // plain (non-enum) ByteProperty value when Size == 1

	Struct []*Property  // StructProperty, when StructName is not a native struct
	Native *NativeValue // StructProperty, when StructName is a native struct
	Array  *ArrayValue  // ArrayProperty / SetProperty

	Raw []byte // fallback: ObjectProperty, or any type not specifically decoded

	// NestedFile is set when Type=="ArrayProperty", Extra.InnerType.Value
	// =="ByteProperty", and Array.Bytes successfully parsed as another
	// embedded [header][property list][footer] blob. See docs/FORMAT.md,
	// "Nested blobs". When set, this is authoritative over Array.Bytes for
	// re-encoding (see encode.go).
	NestedFile *File
}

// File is the top-level (or nested-blob) wrapper: a single reserved header
// byte, a property list, and a trailing footer. See docs/FORMAT.md,
// "Top-level structure".
type File struct {
	HeaderByte byte
	Root       []*Property
	Footer     []byte
}
```

- [ ] **Step 2: Write failing decode tests using testdata/**

Create `gvas/decode_test.go`:

```go
package gvas

import (
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

func findProp(props []*Property, name string) *Property {
	for _, p := range props {
		if p.Name == name {
			return p
		}
	}
	return nil
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
```

- [ ] **Step 3: Run tests, confirm compile failure**

Run: `go test ./gvas/... 2>&1 | head -30`

Expected: FAIL — `Unmarshal` undefined.

- [ ] **Step 4: Implement decode.go**

```go
package gvas

import "fmt"

// Unmarshal decodes a Skyverse .sav buffer (or an embedded nested blob of
// the same shape) into a File. See docs/FORMAT.md for the full format
// this implements.
func Unmarshal(data []byte) (*File, error) {
	r := NewReader(data)
	header, err := r.ReadU8()
	if err != nil {
		return nil, fmt.Errorf("gvas: reading header byte: %w", err)
	}
	root, err := readPropertyList(r)
	if err != nil {
		return nil, err
	}
	footer, err := r.ReadBytes(4)
	if err != nil {
		return nil, fmt.Errorf("gvas: reading trailing footer: %w", err)
	}
	if r.Pos() != r.Len() {
		return nil, fmt.Errorf("gvas: %d unexpected trailing bytes after footer at offset %d", r.Len()-r.Pos(), r.Pos())
	}
	return &File{HeaderByte: header, Root: root, Footer: footer}, nil
}

// readPropertyList reads properties until a "None" terminator (or empty
// name, which the format also treats as a terminator) is hit.
func readPropertyList(r *Reader) ([]*Property, error) {
	var props []*Property
	for {
		startPos := r.Pos()
		name, err := r.ReadFString()
		if err != nil {
			return nil, fmt.Errorf("gvas: reading property name at offset %d: %w", startPos, err)
		}
		if name == "None" || name == "" {
			return props, nil
		}
		p, err := readProperty(r, name)
		if err != nil {
			return nil, fmt.Errorf("gvas: reading property %q at offset %d: %w", name, startPos, err)
		}
		props = append(props, p)
	}
}

func readProperty(r *Reader, name string) (*Property, error) {
	typ, err := r.ReadFString()
	if err != nil {
		return nil, fmt.Errorf("reading type: %w", err)
	}
	p := &Property{Name: name, Type: typ}

	switch typ {
	case "StructProperty":
		if p.Extra.StructName, err = r.ReadFName(); err != nil {
			return nil, fmt.Errorf("reading struct name: %w", err)
		}
		if p.Extra.PackagePath, err = r.ReadFName(); err != nil {
			return nil, fmt.Errorf("reading struct package path: %w", err)
		}
	case "ByteProperty":
		if p.Extra.EnumName, err = r.ReadFName(); err != nil {
			return nil, fmt.Errorf("reading enum name: %w", err)
		}
	case "EnumProperty":
		if p.Extra.EnumTypeName, err = r.ReadFName(); err != nil {
			return nil, fmt.Errorf("reading enum type name: %w", err)
		}
		if p.Extra.EnumPackagePath, err = r.ReadFName(); err != nil {
			return nil, fmt.Errorf("reading enum package path: %w", err)
		}
		if p.Extra.UnderlyingType, err = r.ReadFName(); err != nil {
			return nil, fmt.Errorf("reading enum underlying type: %w", err)
		}
	case "ArrayProperty", "SetProperty":
		if p.Extra.InnerType, err = r.ReadFName(); err != nil {
			return nil, fmt.Errorf("reading inner type: %w", err)
		}
		if p.Extra.InnerType.Value == "StructProperty" {
			if p.Extra.InnerStructName, err = r.ReadFName(); err != nil {
				return nil, fmt.Errorf("reading inner struct name: %w", err)
			}
			if p.Extra.InnerStructPackagePath, err = r.ReadFName(); err != nil {
				return nil, fmt.Errorf("reading inner struct package path: %w", err)
			}
		}
	case "MapProperty":
		if p.Extra.KeyType, err = r.ReadFName(); err != nil {
			return nil, fmt.Errorf("reading map key type: %w", err)
		}
		if p.Extra.ValueType, err = r.ReadFName(); err != nil {
			return nil, fmt.Errorf("reading map value type: %w", err)
		}
	}

	if p.ArrayIndex, err = r.ReadI32(); err != nil {
		return nil, fmt.Errorf("reading array index: %w", err)
	}
	size, err := r.ReadI32()
	if err != nil {
		return nil, fmt.Errorf("reading size: %w", err)
	}
	if size < 0 {
		return nil, fmt.Errorf("negative size %d", size)
	}

	if typ == "BoolProperty" {
		b, err := r.ReadU8()
		if err != nil {
			return nil, fmt.Errorf("reading inline bool value: %w", err)
		}
		v := b != 0
		p.Bool = &v
		return p, nil
	}

	if p.GuidMarker, err = r.ReadU8(); err != nil {
		return nil, fmt.Errorf("reading guid marker: %w", err)
	}
	if p.GuidMarker == 1 {
		if p.Guid, err = r.ReadBytes(16); err != nil {
			return nil, fmt.Errorf("reading property guid: %w", err)
		}
	}

	valEnd := r.Pos() + int(size)
	if err := decodeValue(r, p, int(size)); err != nil {
		return nil, fmt.Errorf("reading value: %w", err)
	}
	if r.Pos() != valEnd {
		return nil, fmt.Errorf("value decode consumed %d bytes, expected %d", r.Pos()-(valEnd-int(size)), size)
	}
	return p, nil
}

func decodeValue(r *Reader, p *Property, size int) error {
	switch p.Type {
	case "StrProperty", "NameProperty":
		s, err := r.ReadFString()
		if err != nil {
			return err
		}
		p.Str = &s
	case "EnumProperty":
		s, err := r.ReadFString()
		if err != nil {
			return err
		}
		p.Str = &s
	case "IntProperty":
		v, err := r.ReadI32()
		if err != nil {
			return err
		}
		p.Int32 = &v
	case "Int64Property":
		v, err := r.ReadI64()
		if err != nil {
			return err
		}
		p.Int64 = &v
	case "FloatProperty":
		v, err := r.ReadF32()
		if err != nil {
			return err
		}
		p.Float32 = &v
	case "DoubleProperty":
		v, err := r.ReadF64()
		if err != nil {
			return err
		}
		p.Float64 = &v
	case "ByteProperty":
		if size == 1 {
			v, err := r.ReadU8()
			if err != nil {
				return err
			}
			p.Byte = &v
			return nil
		}
		raw, err := r.ReadBytes(size)
		if err != nil {
			return err
		}
		p.Raw = raw
	case "StructProperty":
		if isNativeStructName(p.Extra.StructName.Value) {
			raw, err := r.ReadBytes(size)
			if err != nil {
				return err
			}
			p.Native = decodeNativeStruct(p.Extra.StructName.Value, raw)
			return nil
		}
		children, err := readPropertyList(r)
		if err != nil {
			return err
		}
		p.Struct = children
	case "ArrayProperty", "SetProperty":
		av, err := decodeArrayValue(r, p.Extra, size)
		if err != nil {
			return err
		}
		p.Array = av
		if p.Extra.InnerType.Value == "ByteProperty" {
			if nested, err := Unmarshal(av.Bytes); err == nil {
				p.NestedFile = nested
			}
		}
	default:
		raw, err := r.ReadBytes(size)
		if err != nil {
			return err
		}
		p.Raw = raw
	}
	return nil
}

func decodeArrayValue(r *Reader, extra ExtraHeader, size int) (*ArrayValue, error) {
	valEnd := r.Pos() + size
	count, err := r.ReadI32()
	if err != nil {
		return nil, fmt.Errorf("reading array count: %w", err)
	}
	if count < 0 {
		return nil, fmt.Errorf("negative array count %d", count)
	}
	av := &ArrayValue{InnerType: extra.InnerType}

	switch extra.InnerType.Value {
	case "StructProperty":
		for i := int32(0); i < count; i++ {
			children, err := readPropertyList(r)
			if err != nil {
				return nil, fmt.Errorf("array element %d: %w", i, err)
			}
			av.Structs = append(av.Structs, children)
		}
	case "BoolProperty":
		for i := int32(0); i < count; i++ {
			b, err := r.ReadU8()
			if err != nil {
				return nil, err
			}
			av.Bools = append(av.Bools, b != 0)
		}
	case "ByteProperty":
		raw, err := r.ReadBytes(int(count))
		if err != nil {
			return nil, err
		}
		av.Bytes = raw
	case "IntProperty", "UInt32Property":
		for i := int32(0); i < count; i++ {
			v, err := r.ReadI32()
			if err != nil {
				return nil, err
			}
			av.Ints = append(av.Ints, v)
		}
	case "Int64Property":
		for i := int32(0); i < count; i++ {
			v, err := r.ReadI64()
			if err != nil {
				return nil, err
			}
			av.Int64s = append(av.Int64s, v)
		}
	case "FloatProperty":
		for i := int32(0); i < count; i++ {
			v, err := r.ReadF32()
			if err != nil {
				return nil, err
			}
			av.Floats = append(av.Floats, v)
		}
	case "DoubleProperty":
		for i := int32(0); i < count; i++ {
			v, err := r.ReadF64()
			if err != nil {
				return nil, err
			}
			av.Doubles = append(av.Doubles, v)
		}
	case "StrProperty", "NameProperty":
		for i := int32(0); i < count; i++ {
			s, err := r.ReadFString()
			if err != nil {
				return nil, err
			}
			av.Strings = append(av.Strings, s)
		}
	default:
		av.RawCount = count
		remaining := valEnd - r.Pos()
		raw, err := r.ReadBytes(remaining)
		if err != nil {
			return nil, err
		}
		av.RawElements = raw
	}

	if r.Pos() != valEnd {
		return nil, fmt.Errorf("array decode consumed to %d, expected %d", r.Pos(), valEnd)
	}
	return av, nil
}
```

- [ ] **Step 5: Run tests, confirm pass**

Run: `go test ./gvas/... -run 'Unmarshal' -v`

Expected: all PASS. If `TestUnmarshalPlayerLocal`'s nested-file assertion fails, re-check that `decodeValue`'s `ArrayProperty`/`SetProperty` case is reached for the `Data` field specifically (name is not type-discriminating — it's `Extra.InnerType.Value == "ByteProperty"` that matters) and that `Unmarshal` on the sub-slice doesn't error (add a `t.Log` of the error temporarily to inspect).

- [ ] **Step 6: Run full gvas suite**

Run: `go test ./gvas/... -v`

Expected: all PASS (Tasks 2, 3, 4 tests together).

- [ ] **Step 7: Commit**

```bash
git add gvas/property.go gvas/decode.go gvas/decode_test.go
git commit -m "feat(gvas): Property tree types and Unmarshal"
```

---

## Task 5: Marshal + edit round-trip (encode.go)

**Files:**
- Create: `gvas/encode.go`
- Test: `gvas/encode_test.go`

**Interfaces:**
- Consumes: everything from Tasks 2-4.
- Produces:
  - `func Marshal(f *File) ([]byte, error)`
  - `func (p *Property) SetString(v string) error`
  - `func (p *Property) SetBool(v bool) error`
  - `func (p *Property) SetInt32(v int32) error`
  - `func (p *Property) SetInt64(v int64) error`
  - `func (p *Property) SetFloat32(v float32) error`
  - `func (p *Property) SetFloat64(v float64) error`

- [ ] **Step 1: Write failing tests**

Create `gvas/encode_test.go`:

```go
package gvas

import (
	"bytes"
	"os"
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

func TestSetInsideNestedBlobPropagates(t *testing.T) {
	original := readTestdata(t, "Player_Local.sav")
	f, err := Unmarshal(original)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	comps := findProp(f.Root, "Components")
	firstComponent := comps.Array.Structs[0]
	data := findProp(firstComponent, "Data")
	compName := findProp(data.Struct, "ComponentName")
	if compName == nil || compName.Str == nil {
		t.Fatal("first component missing ComponentName")
	}
	if err := compName.SetString("EditedComponentName"); err != nil {
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
	got := findProp(findProp(f2.Root, "Components").Array.Structs[0], "Data")
	gotName := findProp(got.Struct, "ComponentName")
	if gotName == nil || *gotName.Str != "EditedComponentName" {
		t.Fatalf("ComponentName after edit: %+v", gotName)
	}
}
```

- [ ] **Step 2: Run tests, confirm compile failure**

Run: `go test ./gvas/... 2>&1 | head -20`

Expected: FAIL — `Marshal` undefined.

- [ ] **Step 3: Implement encode.go**

```go
package gvas

import "fmt"

// Marshal re-encodes a File to bytes. It always recomputes each
// property's Size from the property's *current* content (bottom-up), so
// editing a scalar value and re-marshaling correctly changes every
// ancestor struct/array's Size — including through an embedded
// NestedFile, which is marshaled recursively before its containing
// ArrayProperty's bytes are computed. When nothing was edited, this is
// byte-for-byte identical to the original input (see gvas.Unmarshal).
func Marshal(f *File) ([]byte, error) {
	w := NewWriter()
	w.WriteU8(f.HeaderByte)
	if err := writePropertyList(w, f.Root); err != nil {
		return nil, err
	}
	w.WriteBytes(f.Footer)
	return w.Bytes(), nil
}

func writePropertyList(w *Writer, props []*Property) error {
	for _, p := range props {
		if err := writeProperty(w, p); err != nil {
			return fmt.Errorf("writing property %q: %w", p.Name, err)
		}
	}
	w.WriteFString("None")
	return nil
}

func writeProperty(w *Writer, p *Property) error {
	w.WriteFString(p.Name)
	w.WriteFString(p.Type)

	switch p.Type {
	case "StructProperty":
		w.WriteFName(p.Extra.StructName)
		w.WriteFName(p.Extra.PackagePath)
	case "ByteProperty":
		w.WriteFName(p.Extra.EnumName)
	case "EnumProperty":
		w.WriteFName(p.Extra.EnumTypeName)
		w.WriteFName(p.Extra.EnumPackagePath)
		w.WriteFName(p.Extra.UnderlyingType)
	case "ArrayProperty", "SetProperty":
		w.WriteFName(p.Extra.InnerType)
		if p.Extra.InnerType.Value == "StructProperty" {
			w.WriteFName(p.Extra.InnerStructName)
			w.WriteFName(p.Extra.InnerStructPackagePath)
		}
	case "MapProperty":
		w.WriteFName(p.Extra.KeyType)
		w.WriteFName(p.Extra.ValueType)
	}

	w.WriteI32(p.ArrayIndex)

	if p.Type == "BoolProperty" {
		w.WriteI32(0) // Size is always 0 for BoolProperty
		if p.Bool == nil {
			return fmt.Errorf("BoolProperty has no value")
		}
		var b uint8
		if *p.Bool {
			b = 1
		}
		w.WriteU8(b)
		return nil
	}

	value, err := encodeValue(p)
	if err != nil {
		return err
	}
	w.WriteI32(int32(len(value)))
	w.WriteU8(p.GuidMarker)
	if p.GuidMarker == 1 {
		w.WriteBytes(p.Guid)
	}
	w.WriteBytes(value)
	return nil
}

func encodeValue(p *Property) ([]byte, error) {
	switch p.Type {
	case "StrProperty", "NameProperty", "EnumProperty":
		if p.Str == nil {
			return nil, fmt.Errorf("%s has no string value", p.Type)
		}
		w := NewWriter()
		w.WriteFString(*p.Str)
		return w.Bytes(), nil
	case "IntProperty":
		if p.Int32 == nil {
			return nil, fmt.Errorf("IntProperty has no value")
		}
		w := NewWriter()
		w.WriteI32(*p.Int32)
		return w.Bytes(), nil
	case "Int64Property":
		if p.Int64 == nil {
			return nil, fmt.Errorf("Int64Property has no value")
		}
		w := NewWriter()
		w.WriteI64(*p.Int64)
		return w.Bytes(), nil
	case "FloatProperty":
		if p.Float32 == nil {
			return nil, fmt.Errorf("FloatProperty has no value")
		}
		w := NewWriter()
		w.WriteF32(*p.Float32)
		return w.Bytes(), nil
	case "DoubleProperty":
		if p.Float64 == nil {
			return nil, fmt.Errorf("DoubleProperty has no value")
		}
		w := NewWriter()
		w.WriteF64(*p.Float64)
		return w.Bytes(), nil
	case "ByteProperty":
		if p.Byte != nil {
			return []byte{*p.Byte}, nil
		}
		return p.Raw, nil
	case "StructProperty":
		if p.Native != nil {
			return encodeNativeValue(p.Native), nil
		}
		w := NewWriter()
		if err := writePropertyList(w, p.Struct); err != nil {
			return nil, err
		}
		return w.Bytes(), nil
	case "ArrayProperty", "SetProperty":
		return encodeArrayProperty(p)
	default:
		return p.Raw, nil
	}
}

func encodeArrayProperty(p *Property) ([]byte, error) {
	// A ByteProperty-inner array that decoded as a NestedFile is
	// authoritative: re-marshal it so edits made inside the nested blob
	// propagate outward. See docs/FORMAT.md, "Nested blobs".
	if p.Extra.InnerType.Value == "ByteProperty" && p.NestedFile != nil {
		blob, err := Marshal(p.NestedFile)
		if err != nil {
			return nil, fmt.Errorf("marshaling nested file: %w", err)
		}
		w := NewWriter()
		w.WriteI32(int32(len(blob)))
		w.WriteBytes(blob)
		return w.Bytes(), nil
	}
	return encodeArrayValue(p.Array)
}

func encodeArrayValue(a *ArrayValue) ([]byte, error) {
	w := NewWriter()
	switch a.InnerType.Value {
	case "StructProperty":
		w.WriteI32(int32(len(a.Structs)))
		for _, elem := range a.Structs {
			if err := writePropertyList(w, elem); err != nil {
				return nil, err
			}
		}
	case "BoolProperty":
		w.WriteI32(int32(len(a.Bools)))
		for _, b := range a.Bools {
			var v uint8
			if b {
				v = 1
			}
			w.WriteU8(v)
		}
	case "ByteProperty":
		w.WriteI32(int32(len(a.Bytes)))
		w.WriteBytes(a.Bytes)
	case "IntProperty", "UInt32Property":
		w.WriteI32(int32(len(a.Ints)))
		for _, v := range a.Ints {
			w.WriteI32(v)
		}
	case "Int64Property":
		w.WriteI32(int32(len(a.Int64s)))
		for _, v := range a.Int64s {
			w.WriteI64(v)
		}
	case "FloatProperty":
		w.WriteI32(int32(len(a.Floats)))
		for _, v := range a.Floats {
			w.WriteF32(v)
		}
	case "DoubleProperty":
		w.WriteI32(int32(len(a.Doubles)))
		for _, v := range a.Doubles {
			w.WriteF64(v)
		}
	case "StrProperty", "NameProperty":
		w.WriteI32(int32(len(a.Strings)))
		for _, s := range a.Strings {
			w.WriteFString(s)
		}
	default:
		w.WriteI32(a.RawCount)
		w.WriteBytes(a.RawElements)
	}
	return w.Bytes(), nil
}

// --- scalar setters (see docs/superpowers/specs/2026-09-18-skyverse-save-tool-design.md, "Encode") ---

func (p *Property) SetString(v string) error {
	if p.Str == nil {
		return fmt.Errorf("gvas: %s is a %s, not a string-valued property", p.Name, p.Type)
	}
	p.Str = &v
	return nil
}

func (p *Property) SetBool(v bool) error {
	if p.Bool == nil {
		return fmt.Errorf("gvas: %s is a %s, not a bool-valued property", p.Name, p.Type)
	}
	p.Bool = &v
	return nil
}

func (p *Property) SetInt32(v int32) error {
	if p.Int32 == nil {
		return fmt.Errorf("gvas: %s is a %s, not an int32-valued property", p.Name, p.Type)
	}
	p.Int32 = &v
	return nil
}

func (p *Property) SetInt64(v int64) error {
	if p.Int64 == nil {
		return fmt.Errorf("gvas: %s is a %s, not an int64-valued property", p.Name, p.Type)
	}
	p.Int64 = &v
	return nil
}

func (p *Property) SetFloat32(v float32) error {
	if p.Float32 == nil {
		return fmt.Errorf("gvas: %s is a %s, not a float32-valued property", p.Name, p.Type)
	}
	p.Float32 = &v
	return nil
}

func (p *Property) SetFloat64(v float64) error {
	if p.Float64 == nil {
		return fmt.Errorf("gvas: %s is a %s, not a float64-valued property", p.Name, p.Type)
	}
	p.Float64 = &v
	return nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./gvas/... -v`

Expected: all PASS, including the byte-exact `TestMarshalUnmarshalRoundTrip` for all three testdata files. If a round-trip test fails, the failure message names the first differing byte offset — cross-reference that offset against `docs/FORMAT.md` to find which assumption (extra-header field, FString encoding choice, etc.) needs correcting; do not weaken the test.

- [ ] **Step 5: Commit**

```bash
git add gvas/encode.go gvas/encode_test.go
git commit -m "feat(gvas): Marshal with byte-exact round-trip and scalar edit propagation"
```

---

## Task 6: Typed reflection layer (reflect.go)

**Files:**
- Create: `gvas/reflect.go`
- Test: `gvas/reflect_test.go`

**Interfaces:**
- Consumes: `File`, `Property`, `Property.SetString/SetBool/SetInt32/SetInt64/SetFloat32/SetFloat64` (Tasks 4-5).
- Produces: `func DecodeInto(f *File, v any) error`, `func EncodeInto(f *File, v any) error`

- [ ] **Step 1: Write failing tests**

Create `gvas/reflect_test.go`:

```go
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
```

- [ ] **Step 2: Run tests, confirm compile failure**

Run: `go test ./gvas/... 2>&1 | head -20`

Expected: FAIL — `DecodeInto` undefined.

- [ ] **Step 3: Implement reflect.go**

```go
package gvas

import (
	"fmt"
	"reflect"
)

// DecodeInto populates a struct pointed to by v from f's top-level
// property list, mirroring encoding/json's Unmarshal. A field matches the
// property whose Name equals the field's `save:"..."` tag, or the field's
// own name if untagged. Supported field kinds: string, bool, int32, int64,
// float32, float64, a nested struct (matched against a non-native
// StructProperty), and slices of the above (matched against an
// ArrayProperty). Unmatched struct fields are left at their zero value;
// unmatched save properties are ignored. This is a convenience layered on
// the generic Property tree (see docs/FORMAT.md and property.go) — it is
// not required for the tree to be inspectable, only for ergonomic typed
// access to properties a caller already knows the shape of.
func DecodeInto(f *File, v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return fmt.Errorf("gvas: DecodeInto requires a non-nil pointer, got %T", v)
	}
	elem := rv.Elem()
	if elem.Kind() != reflect.Struct {
		return fmt.Errorf("gvas: DecodeInto requires a pointer to a struct, got %T", v)
	}
	return decodeStructFields(f.Root, elem)
}

func decodeStructFields(props []*Property, structVal reflect.Value) error {
	byName := make(map[string]*Property, len(props))
	for _, p := range props {
		byName[p.Name] = p
	}
	t := structVal.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		name := field.Tag.Get("save")
		if name == "" {
			name = field.Name
		}
		p, ok := byName[name]
		if !ok {
			continue
		}
		if err := decodeFieldValue(p, structVal.Field(i)); err != nil {
			return fmt.Errorf("field %s (property %q): %w", field.Name, name, err)
		}
	}
	return nil
}

func decodeFieldValue(p *Property, fv reflect.Value) error {
	switch fv.Kind() {
	case reflect.String:
		if p.Str == nil {
			return fmt.Errorf("property is a %s, not string-valued", p.Type)
		}
		fv.SetString(*p.Str)
	case reflect.Bool:
		if p.Bool == nil {
			return fmt.Errorf("property is a %s, not bool-valued", p.Type)
		}
		fv.SetBool(*p.Bool)
	case reflect.Int32:
		if p.Int32 == nil {
			return fmt.Errorf("property is a %s, not int32-valued", p.Type)
		}
		fv.SetInt(int64(*p.Int32))
	case reflect.Int64:
		if p.Int64 == nil {
			return fmt.Errorf("property is a %s, not int64-valued", p.Type)
		}
		fv.SetInt(*p.Int64)
	case reflect.Float32:
		if p.Float32 == nil {
			return fmt.Errorf("property is a %s, not float32-valued", p.Type)
		}
		fv.SetFloat(float64(*p.Float32))
	case reflect.Float64:
		if p.Float64 == nil {
			return fmt.Errorf("property is a %s, not float64-valued", p.Type)
		}
		fv.SetFloat(*p.Float64)
	case reflect.Struct:
		if p.Struct == nil {
			return fmt.Errorf("property is a %s, not a nested struct", p.Type)
		}
		return decodeStructFields(p.Struct, fv)
	case reflect.Slice:
		if p.Array == nil {
			return fmt.Errorf("property is a %s, not an array", p.Type)
		}
		return decodeSliceValue(p.Array, fv)
	default:
		return fmt.Errorf("unsupported Go field kind %s", fv.Kind())
	}
	return nil
}

func decodeSliceValue(a *ArrayValue, fv reflect.Value) error {
	elemType := fv.Type().Elem()
	switch elemType.Kind() {
	case reflect.String:
		out := reflect.MakeSlice(fv.Type(), len(a.Strings), len(a.Strings))
		for i, s := range a.Strings {
			out.Index(i).SetString(s)
		}
		fv.Set(out)
	case reflect.Bool:
		out := reflect.MakeSlice(fv.Type(), len(a.Bools), len(a.Bools))
		for i, b := range a.Bools {
			out.Index(i).SetBool(b)
		}
		fv.Set(out)
	case reflect.Int32:
		out := reflect.MakeSlice(fv.Type(), len(a.Ints), len(a.Ints))
		for i, v := range a.Ints {
			out.Index(i).SetInt(int64(v))
		}
		fv.Set(out)
	case reflect.Float32:
		out := reflect.MakeSlice(fv.Type(), len(a.Floats), len(a.Floats))
		for i, v := range a.Floats {
			out.Index(i).SetFloat(float64(v))
		}
		fv.Set(out)
	case reflect.Struct:
		out := reflect.MakeSlice(fv.Type(), len(a.Structs), len(a.Structs))
		for i, elem := range a.Structs {
			if err := decodeStructFields(elem, out.Index(i)); err != nil {
				return fmt.Errorf("element %d: %w", i, err)
			}
		}
		fv.Set(out)
	default:
		return fmt.Errorf("unsupported slice element kind %s", elemType.Kind())
	}
	return nil
}

// EncodeInto is the write-back counterpart to DecodeInto: it copies scalar
// field values from the struct pointed to by v into the matching
// Properties already present in f's tree (via the same `save:"..."` tag
// matching rules), so a subsequent gvas.Marshal(f) reflects them. Like
// DecodeInto, field matching walks nested structs and slices of structs;
// unlike DecodeInto, every exported field must match an existing property
// of a compatible scalar kind, or EncodeInto returns an error — there is
// no property-creation path (see docs/superpowers/specs/2026-09-18-skyverse-save-tool-design.md,
// "Out of scope for this pass").
func EncodeInto(f *File, v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return fmt.Errorf("gvas: EncodeInto requires a non-nil pointer, got %T", v)
	}
	elem := rv.Elem()
	if elem.Kind() != reflect.Struct {
		return fmt.Errorf("gvas: EncodeInto requires a pointer to a struct, got %T", v)
	}
	return encodeStructFields(f.Root, elem)
}

func encodeStructFields(props []*Property, structVal reflect.Value) error {
	byName := make(map[string]*Property, len(props))
	for _, p := range props {
		byName[p.Name] = p
	}
	t := structVal.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		name := field.Tag.Get("save")
		if name == "" {
			name = field.Name
		}
		p, ok := byName[name]
		if !ok {
			return fmt.Errorf("field %s: no property named %q", field.Name, name)
		}
		if err := encodeFieldValue(p, structVal.Field(i)); err != nil {
			return fmt.Errorf("field %s (property %q): %w", field.Name, name, err)
		}
	}
	return nil
}

func encodeFieldValue(p *Property, fv reflect.Value) error {
	switch fv.Kind() {
	case reflect.String:
		return p.SetString(fv.String())
	case reflect.Bool:
		return p.SetBool(fv.Bool())
	case reflect.Int32:
		return p.SetInt32(int32(fv.Int()))
	case reflect.Int64:
		return p.SetInt64(fv.Int())
	case reflect.Float32:
		return p.SetFloat32(float32(fv.Float()))
	case reflect.Float64:
		return p.SetFloat64(fv.Float())
	case reflect.Struct:
		if p.Struct == nil {
			return fmt.Errorf("property is a %s, not a nested struct", p.Type)
		}
		return encodeStructFields(p.Struct, fv)
	case reflect.Slice:
		return fmt.Errorf("EncodeInto does not support writing back slice fields in this version (property %q, type %s)", p.Name, p.Type)
	default:
		return fmt.Errorf("unsupported Go field kind %s", fv.Kind())
	}
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./gvas/... -run 'Decode|Encode' -v`

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add gvas/reflect.go gvas/reflect_test.go
git commit -m "feat(gvas): reflection-based DecodeInto/EncodeInto for typed struct access"
```

---

## Task 7: Property paths (path.go)

**Files:**
- Create: `gvas/path.go`
- Test: `gvas/path_test.go`

**Interfaces:**
- Consumes: `File`, `Property` (Task 4).
- Produces: `func Lookup(f *File, path string) (*Property, error)`

- [ ] **Step 1: Write failing tests**

Create `gvas/path_test.go`:

```go
package gvas

import "testing"

func TestLookupTopLevel(t *testing.T) {
	f, err := Unmarshal(readTestdata(t, "WorldInfo.sav"))
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	p, err := Lookup(f, "WorldName")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if p.Str == nil || *p.Str != "jff" {
		t.Errorf("got %+v", p)
	}
}

func TestLookupNestedStruct(t *testing.T) {
	f, err := Unmarshal(readTestdata(t, "WorldInfo.sav"))
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	p, err := Lookup(f, "UDSData.CurrentTime.Month")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if p.Int32 == nil {
		t.Errorf("got %+v", p)
	}
}

func TestLookupArrayIndex(t *testing.T) {
	f, err := Unmarshal(readTestdata(t, "Player_Local.sav"))
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	p, err := Lookup(f, "Components[0].Data.ComponentName")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if p.Str == nil {
		t.Errorf("got %+v", p)
	}
}

func TestLookupThroughNestedFile(t *testing.T) {
	f, err := Unmarshal(readTestdata(t, "Player_Local.sav"))
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	// Data.Data is an ArrayProperty<ByteProperty> that decoded as a
	// NestedFile; Lookup must transparently descend into it.
	p, err := Lookup(f, "Components[0].Data.Data.ComponentSaveDataDoesNotExist")
	if err == nil {
		t.Fatalf("expected an error for a nonexistent property, got %+v", p)
	}
}

func TestLookupErrors(t *testing.T) {
	f, err := Unmarshal(readTestdata(t, "WorldInfo.sav"))
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if _, err := Lookup(f, "DoesNotExist"); err == nil {
		t.Error("expected error for missing top-level property")
	}
	if _, err := Lookup(f, "BoatsData[999]"); err == nil {
		t.Error("expected error for out-of-range array index")
	}
	if _, err := Lookup(f, "WorldName.Nested"); err == nil {
		t.Error("expected error descending into a scalar")
	}
}
```

- [ ] **Step 2: Run tests, confirm compile failure**

Run: `go test ./gvas/... 2>&1 | head -20`

Expected: FAIL — `Lookup` undefined.

- [ ] **Step 3: Implement path.go**

```go
package gvas

import (
	"fmt"
	"strconv"
	"strings"
)

// pathSegment is either a field name (Name!="" ) or an array index
// (HasIndex==true), matching a single "Foo" or "[N]" token in a path like
// "Components[3].ComponentName".
type pathSegment struct {
	Name     string
	HasIndex bool
	Index    int
}

// parsePath splits "Components[3].Data.ComponentName" into
// [{Name:Components} {HasIndex:true Index:3} {Name:Data} {Name:ComponentName}].
func parsePath(path string) ([]pathSegment, error) {
	var segs []pathSegment
	for _, dotPart := range strings.Split(path, ".") {
		if dotPart == "" {
			return nil, fmt.Errorf("gvas: empty path segment in %q", path)
		}
		name := dotPart
		var indices []int
		for {
			open := strings.IndexByte(name, '[')
			if open < 0 {
				break
			}
			close := strings.IndexByte(name[open:], ']')
			if close < 0 {
				return nil, fmt.Errorf("gvas: unclosed '[' in path segment %q", dotPart)
			}
			close += open
			idx, err := strconv.Atoi(name[open+1 : close])
			if err != nil {
				return nil, fmt.Errorf("gvas: invalid array index in %q: %w", dotPart, err)
			}
			indices = append(indices, idx)
			name = name[:open] + name[close+1:]
		}
		segs = append(segs, pathSegment{Name: name})
		for _, idx := range indices {
			segs = append(segs, pathSegment{HasIndex: true, Index: idx})
		}
	}
	return segs, nil
}

// Lookup resolves a dotted/bracketed property path (see docs/superpowers/specs/2026-09-18-skyverse-save-tool-design.md,
// "Property paths") against f, starting from the top-level property list.
// It transparently descends through an ArrayProperty<ByteProperty> that
// decoded as a NestedFile, as if it were an ordinary nested struct.
func Lookup(f *File, path string) (*Property, error) {
	segs, err := parsePath(path)
	if err != nil {
		return nil, err
	}
	props := f.Root
	var current *Property
	for i, seg := range segs {
		if seg.HasIndex {
			if current == nil || current.Array == nil {
				return nil, fmt.Errorf("gvas: path %q: [%d] used on a non-array at segment %d", path, seg.Index, i)
			}
			elem, err := arrayElementProps(current.Array, seg.Index)
			if err != nil {
				return nil, fmt.Errorf("gvas: path %q: %w", path, err)
			}
			props = elem
			current = nil
			continue
		}
		found := findProp(props, seg.Name)
		if found == nil {
			return nil, fmt.Errorf("gvas: path %q: no property named %q at segment %d", path, seg.Name, i)
		}
		current = found
		switch {
		case found.Struct != nil:
			props = found.Struct
		case found.NestedFile != nil:
			props = found.NestedFile.Root
		default:
			props = nil
		}
	}
	if current == nil {
		return nil, fmt.Errorf("gvas: path %q did not resolve to a property", path)
	}
	return current, nil
}

func arrayElementProps(a *ArrayValue, idx int) ([]*Property, error) {
	if a.Structs == nil {
		return nil, fmt.Errorf("array inner type %q has no struct elements to index into", a.InnerType.Value)
	}
	if idx < 0 || idx >= len(a.Structs) {
		return nil, fmt.Errorf("index %d out of range (array has %d elements)", idx, len(a.Structs))
	}
	return a.Structs[idx], nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./gvas/... -run Lookup -v`

Expected: all PASS.

- [ ] **Step 5: Run the full gvas package test suite before moving to the CLI**

Run: `go test ./gvas/... -v`

Expected: every test across Tasks 2-7 PASSes.

- [ ] **Step 6: Commit**

```bash
git add gvas/path.go gvas/path_test.go
git commit -m "feat(gvas): property path parsing and Lookup"
```

---

## Task 8: CLI scaffolding + dump command

**Files:**
- Modify: `cmd/saveview/main.go`
- Create: `cmd/saveview/dump.go`
- Test: `cmd/saveview/dump_test.go`

**Interfaces:**
- Consumes: `gvas.Unmarshal`, `gvas.File`, `gvas.Property` (gvas package).
- Produces: `func runDump(w io.Writer, path string) error`, wired into `main()` as `saveview dump <file>`.

- [ ] **Step 1: Write a failing test**

Create `cmd/saveview/dump_test.go`:

```go
package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunDumpWorldInfo(t *testing.T) {
	var buf bytes.Buffer
	if err := runDump(&buf, "../../testdata/WorldInfo.sav"); err != nil {
		t.Fatalf("runDump: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"WorldName", "GameVersion", "UDSData", "ItemPickupStruct"} {
		if !strings.Contains(out, want) {
			t.Errorf("dump output missing %q", want)
		}
	}
}

func TestRunDumpMissingFile(t *testing.T) {
	var buf bytes.Buffer
	if err := runDump(&buf, "../../testdata/does-not-exist.sav"); err == nil {
		t.Fatal("expected an error for a missing file")
	}
}
```

- [ ] **Step 2: Run test, confirm compile failure**

Run: `go test ./cmd/saveview/... 2>&1 | head -20`

Expected: FAIL — `runDump` undefined.

- [ ] **Step 3: Implement dump.go**

```go
package main

import (
	"fmt"
	"io"
	"os"

	"skyversesave/gvas"
)

func runDump(w io.Writer, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	f, err := gvas.Unmarshal(data)
	if err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}
	dumpProps(w, f.Root, 0)
	return nil
}

func dumpProps(w io.Writer, props []*gvas.Property, depth int) {
	indent := ""
	for i := 0; i < depth; i++ {
		indent += "  "
	}
	for _, p := range props {
		fmt.Fprintf(w, "%s%s: %s = %s\n", indent, p.Name, p.Type, formatValue(p))
		switch {
		case p.Struct != nil:
			dumpProps(w, p.Struct, depth+1)
		case p.NestedFile != nil:
			fmt.Fprintf(w, "%s  [nested property list]\n", indent)
			dumpProps(w, p.NestedFile.Root, depth+1)
		case p.Array != nil && p.Array.Structs != nil:
			for i, elem := range p.Array.Structs {
				fmt.Fprintf(w, "%s  [%d]\n", indent, i)
				dumpProps(w, elem, depth+2)
			}
		}
	}
}

func formatValue(p *gvas.Property) string {
	switch {
	case p.Bool != nil:
		return fmt.Sprintf("%v", *p.Bool)
	case p.Str != nil:
		return *p.Str
	case p.Int32 != nil:
		return fmt.Sprintf("%d", *p.Int32)
	case p.Int64 != nil:
		return fmt.Sprintf("%d", *p.Int64)
	case p.Float32 != nil:
		return fmt.Sprintf("%g", *p.Float32)
	case p.Float64 != nil:
		return fmt.Sprintf("%g", *p.Float64)
	case p.Byte != nil:
		return fmt.Sprintf("%d", *p.Byte)
	case p.Native != nil:
		return formatNative(p.Native)
	case p.Array != nil:
		return fmt.Sprintf("<%d elements>", arrayLen(p.Array))
	case p.Struct != nil:
		return fmt.Sprintf("<%d fields>", len(p.Struct))
	default:
		return fmt.Sprintf("<%d raw bytes>", len(p.Raw))
	}
}

func formatNative(n *gvas.NativeValue) string {
	switch {
	case n.Vector != nil:
		return fmt.Sprintf("{%g, %g, %g}", n.Vector.X, n.Vector.Y, n.Vector.Z)
	case n.IntVector != nil:
		return fmt.Sprintf("{%d, %d, %d}", n.IntVector.X, n.IntVector.Y, n.IntVector.Z)
	case n.Rotator != nil:
		return fmt.Sprintf("{pitch:%g yaw:%g roll:%g}", n.Rotator.Pitch, n.Rotator.Yaw, n.Rotator.Roll)
	case n.DateTimeTicks != nil:
		return fmt.Sprintf("%d ticks", *n.DateTimeTicks)
	case n.TimespanTicks != nil:
		return fmt.Sprintf("%d ticks", *n.TimespanTicks)
	default:
		return fmt.Sprintf("<%d raw bytes>", len(n.Raw))
	}
}

func arrayLen(a *gvas.ArrayValue) int {
	switch {
	case a.Structs != nil:
		return len(a.Structs)
	case a.Bools != nil:
		return len(a.Bools)
	case a.Ints != nil:
		return len(a.Ints)
	case a.Floats != nil:
		return len(a.Floats)
	case a.Doubles != nil:
		return len(a.Doubles)
	case a.Strings != nil:
		return len(a.Strings)
	case a.Bytes != nil:
		return len(a.Bytes)
	default:
		return int(a.RawCount)
	}
}
```

- [ ] **Step 4: Wire `dump` into main.go**

In `cmd/saveview/main.go`, replace the `switch` body:

```go
	switch os.Args[1] {
	case "dump":
		if len(os.Args) != 3 {
			fmt.Fprintln(os.Stderr, "usage: saveview dump <file>")
			os.Exit(2)
		}
		if err := runDump(os.Stdout, os.Args[2]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
```

- [ ] **Step 5: Run tests, confirm pass**

Run: `go test ./cmd/saveview/... -v`

Expected: all PASS.

- [ ] **Step 6: Manual smoke test**

Run: `go run ./cmd/saveview dump testdata/WorldInfo.sav | head -30`

Expected: readable indented tree starting with `WorldName: StrProperty = jff`.

- [ ] **Step 7: Commit**

```bash
git add cmd/saveview/main.go cmd/saveview/dump.go cmd/saveview/dump_test.go
git commit -m "feat(cli): dump command"
```

---

## Task 9: json command

**Files:**
- Modify: `cmd/saveview/main.go`
- Create: `cmd/saveview/jsonout.go`
- Test: `cmd/saveview/jsonout_test.go`

**Interfaces:**
- Consumes: `gvas.File`, `gvas.Property` (gvas package), `formatValue`/`formatNative`/`arrayLen` (Task 8, same package).
- Produces: `func runJSON(w io.Writer, path string) error`, wired into `main()` as `saveview json <file>`.

- [ ] **Step 1: Write a failing test**

Create `cmd/saveview/jsonout_test.go`:

```go
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
```

- [ ] **Step 2: Run test, confirm compile failure**

Run: `go test ./cmd/saveview/... 2>&1 | head -20`

Expected: FAIL — `runJSON` undefined.

- [ ] **Step 3: Implement jsonout.go**

```go
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"skyversesave/gvas"
)

// jsonProp is the JSON projection of a gvas.Property: enough to be useful
// standalone, and to expand nested content (structs, nested blobs, struct
// array elements) inline rather than as opaque blobs.
type jsonProp struct {
	Name  string      `json:"name"`
	Type  string      `json:"type"`
	Value any         `json:"value,omitempty"`
	Field []jsonProp  `json:"fields,omitempty"`
	Elems []jsonElem  `json:"elements,omitempty"`
}

type jsonElem struct {
	Fields []jsonProp `json:"fields"`
}

func toJSONProps(props []*gvas.Property) []jsonProp {
	out := make([]jsonProp, 0, len(props))
	for _, p := range props {
		jp := jsonProp{Name: p.Name, Type: p.Type}
		switch {
		case p.Struct != nil:
			jp.Field = toJSONProps(p.Struct)
		case p.NestedFile != nil:
			jp.Field = toJSONProps(p.NestedFile.Root)
		case p.Array != nil && p.Array.Structs != nil:
			for _, elem := range p.Array.Structs {
				jp.Elems = append(jp.Elems, jsonElem{Fields: toJSONProps(elem)})
			}
		default:
			jp.Value = formatValue(p)
		}
		out = append(out, jp)
	}
	return out
}

func runJSON(w io.Writer, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	f, err := gvas.Unmarshal(data)
	if err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(toJSONProps(f.Root))
}
```

- [ ] **Step 4: Wire `json` into main.go**

In `cmd/saveview/main.go`, add a case alongside `dump`:

```go
	case "json":
		if len(os.Args) != 3 {
			fmt.Fprintln(os.Stderr, "usage: saveview json <file>")
			os.Exit(2)
		}
		if err := runJSON(os.Stdout, os.Args[2]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
```

- [ ] **Step 5: Run tests, confirm pass**

Run: `go test ./cmd/saveview/... -v`

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd/saveview/main.go cmd/saveview/jsonout.go cmd/saveview/jsonout_test.go
git commit -m "feat(cli): json command with nested blobs expanded inline"
```

---

## Task 10: get command

**Files:**
- Modify: `cmd/saveview/main.go`
- Create: `cmd/saveview/get.go`
- Test: `cmd/saveview/get_test.go`

**Interfaces:**
- Consumes: `gvas.Unmarshal`, `gvas.Lookup` (gvas package), `formatValue` (Task 8).
- Produces: `func runGet(w io.Writer, path, propPath string) error`, wired into `main()` as `saveview get <file> <path>`.

- [ ] **Step 1: Write a failing test**

Create `cmd/saveview/get_test.go`:

```go
package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunGetScalar(t *testing.T) {
	var buf bytes.Buffer
	if err := runGet(&buf, "../../testdata/WorldInfo.sav", "WorldName"); err != nil {
		t.Fatalf("runGet: %v", err)
	}
	if strings.TrimSpace(buf.String()) != "jff" {
		t.Errorf("got %q", buf.String())
	}
}

func TestRunGetNestedPath(t *testing.T) {
	var buf bytes.Buffer
	if err := runGet(&buf, "../../testdata/Player_Local.sav", "Components[0].Data.ComponentName"); err != nil {
		t.Fatalf("runGet: %v", err)
	}
	if strings.TrimSpace(buf.String()) == "" {
		t.Error("expected a non-empty component name")
	}
}

func TestRunGetMissingPath(t *testing.T) {
	var buf bytes.Buffer
	if err := runGet(&buf, "../../testdata/WorldInfo.sav", "DoesNotExist"); err == nil {
		t.Fatal("expected an error for a missing property path")
	}
}
```

- [ ] **Step 2: Run test, confirm compile failure**

Run: `go test ./cmd/saveview/... 2>&1 | head -20`

Expected: FAIL — `runGet` undefined.

- [ ] **Step 3: Implement get.go**

```go
package main

import (
	"fmt"
	"io"
	"os"

	"skyversesave/gvas"
)

func runGet(w io.Writer, path, propPath string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	f, err := gvas.Unmarshal(data)
	if err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}
	p, err := gvas.Lookup(f, propPath)
	if err != nil {
		return err
	}
	fmt.Fprintln(w, formatValue(p))
	return nil
}
```

- [ ] **Step 4: Wire `get` into main.go**

```go
	case "get":
		if len(os.Args) != 4 {
			fmt.Fprintln(os.Stderr, "usage: saveview get <file> <path>")
			os.Exit(2)
		}
		if err := runGet(os.Stdout, os.Args[2], os.Args[3]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
```

- [ ] **Step 5: Run tests, confirm pass**

Run: `go test ./cmd/saveview/... -v`

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd/saveview/main.go cmd/saveview/get.go cmd/saveview/get_test.go
git commit -m "feat(cli): get command"
```

---

## Task 11: set command

**Files:**
- Modify: `cmd/saveview/main.go`
- Create: `cmd/saveview/set.go`
- Test: `cmd/saveview/set_test.go`

**Interfaces:**
- Consumes: `gvas.Unmarshal`, `gvas.Marshal`, `gvas.Lookup`, `Property.SetString/SetBool/SetInt32/SetInt64/SetFloat32/SetFloat64` (gvas package).
- Produces: `func runSet(path, propPath, rawValue, outPath string) error`, wired into `main()` as `saveview set <file> <path> <value> -o <out>`.

- [ ] **Step 1: Write a failing test**

Create `cmd/saveview/set_test.go`:

```go
package main

import (
	"os"
	"path/filepath"
	"testing"

	"skyversesave/gvas"
)

func TestRunSetString(t *testing.T) {
	out := filepath.Join(t.TempDir(), "out.sav")
	if err := runSet("../../testdata/WorldInfo.sav", "WorldName", "NewName", out); err != nil {
		t.Fatalf("runSet: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}
	f, err := gvas.Unmarshal(data)
	if err != nil {
		t.Fatalf("parsing written file: %v", err)
	}
	p, err := gvas.Lookup(f, "WorldName")
	if err != nil || p.Str == nil || *p.Str != "NewName" {
		t.Fatalf("WorldName after set: %+v, err=%v", p, err)
	}
}

func TestRunSetBool(t *testing.T) {
	out := filepath.Join(t.TempDir(), "out.sav")
	if err := runSet("../../testdata/WorldInfo.sav", "bNewGame", "true", out); err != nil {
		t.Fatalf("runSet: %v", err)
	}
	data, _ := os.ReadFile(out)
	f, err := gvas.Unmarshal(data)
	if err != nil {
		t.Fatalf("parsing written file: %v", err)
	}
	p, err := gvas.Lookup(f, "bNewGame")
	if err != nil || p.Bool == nil || *p.Bool != true {
		t.Fatalf("bNewGame after set: %+v, err=%v", p, err)
	}
}

func TestRunSetInt(t *testing.T) {
	out := filepath.Join(t.TempDir(), "out.sav")
	if err := runSet("../../testdata/WorldInfo.sav", "UDSData.CurrentTime.Month", "12", out); err != nil {
		t.Fatalf("runSet: %v", err)
	}
	data, _ := os.ReadFile(out)
	f, err := gvas.Unmarshal(data)
	if err != nil {
		t.Fatalf("parsing written file: %v", err)
	}
	p, err := gvas.Lookup(f, "UDSData.CurrentTime.Month")
	if err != nil || p.Int32 == nil || *p.Int32 != 12 {
		t.Fatalf("Month after set: %+v, err=%v", p, err)
	}
}

func TestRunSetInvalidValueErrors(t *testing.T) {
	out := filepath.Join(t.TempDir(), "out.sav")
	if err := runSet("../../testdata/WorldInfo.sav", "UDSData.CurrentTime.Month", "not-a-number", out); err == nil {
		t.Fatal("expected an error parsing a non-numeric value for an int property")
	}
}

func TestRunSetNeverTouchesInput(t *testing.T) {
	inputCopy := filepath.Join(t.TempDir(), "input.sav")
	original, err := os.ReadFile("../../testdata/WorldInfo.sav")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inputCopy, original, 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "out.sav")
	if err := runSet(inputCopy, "WorldName", "Whatever", out); err != nil {
		t.Fatalf("runSet: %v", err)
	}
	after, err := os.ReadFile(inputCopy)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(original) {
		t.Fatal("runSet modified the input file")
	}
}
```

- [ ] **Step 2: Run test, confirm compile failure**

Run: `go test ./cmd/saveview/... 2>&1 | head -20`

Expected: FAIL — `runSet` undefined.

- [ ] **Step 3: Implement set.go**

```go
package main

import (
	"fmt"
	"os"
	"strconv"

	"skyversesave/gvas"
)

// runSet loads path, resolves propPath, parses rawValue against that
// property's existing scalar kind, and writes the edited file to outPath.
// The input file at path is never modified — see docs/superpowers/specs/2026-09-18-skyverse-save-tool-design.md,
// "`set` requires `-o`".
func runSet(path, propPath, rawValue, outPath string) error {
	if outPath == "" {
		return fmt.Errorf("an output path (-o) is required; the input file is never overwritten in place")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	f, err := gvas.Unmarshal(data)
	if err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}
	p, err := gvas.Lookup(f, propPath)
	if err != nil {
		return err
	}
	if err := applyScalarEdit(p, rawValue); err != nil {
		return fmt.Errorf("setting %s: %w", propPath, err)
	}
	out, err := gvas.Marshal(f)
	if err != nil {
		return fmt.Errorf("encoding edited save: %w", err)
	}
	if err := os.WriteFile(outPath, out, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", outPath, err)
	}
	return nil
}

func applyScalarEdit(p *gvas.Property, rawValue string) error {
	switch {
	case p.Str != nil:
		return p.SetString(rawValue)
	case p.Bool != nil:
		v, err := strconv.ParseBool(rawValue)
		if err != nil {
			return fmt.Errorf("parsing %q as bool: %w", rawValue, err)
		}
		return p.SetBool(v)
	case p.Int32 != nil:
		v, err := strconv.ParseInt(rawValue, 10, 32)
		if err != nil {
			return fmt.Errorf("parsing %q as int32: %w", rawValue, err)
		}
		return p.SetInt32(int32(v))
	case p.Int64 != nil:
		v, err := strconv.ParseInt(rawValue, 10, 64)
		if err != nil {
			return fmt.Errorf("parsing %q as int64: %w", rawValue, err)
		}
		return p.SetInt64(v)
	case p.Float32 != nil:
		v, err := strconv.ParseFloat(rawValue, 32)
		if err != nil {
			return fmt.Errorf("parsing %q as float32: %w", rawValue, err)
		}
		return p.SetFloat32(float32(v))
	case p.Float64 != nil:
		v, err := strconv.ParseFloat(rawValue, 64)
		if err != nil {
			return fmt.Errorf("parsing %q as float64: %w", rawValue, err)
		}
		return p.SetFloat64(v)
	default:
		return fmt.Errorf("property %q (type %s) is not an editable scalar in this version of saveview", p.Name, p.Type)
	}
}
```

- [ ] **Step 4: Wire `set` into main.go**

```go
	case "set":
		fs := flag.NewFlagSet("set", flag.ExitOnError)
		out := fs.String("o", "", "output file path (required)")
		fs.Parse(os.Args[2:])
		if fs.NArg() != 3 || *out == "" {
			fmt.Fprintln(os.Stderr, "usage: saveview set <file> <path> <value> -o <out>")
			os.Exit(2)
		}
		if err := runSet(fs.Arg(0), fs.Arg(1), fs.Arg(2), *out); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
```

Add `"flag"` to the import block in `main.go`.

- [ ] **Step 5: Run tests, confirm pass**

Run: `go test ./cmd/saveview/... -v`

Expected: all PASS.

- [ ] **Step 6: Manual smoke test**

Run:
```bash
go build ./...
./saveview set testdata/WorldInfo.sav WorldName "Renamed" -o /tmp/edited.sav
./saveview get /tmp/edited.sav WorldName
```
Expected: prints `Renamed`.

- [ ] **Step 7: Commit**

```bash
git add cmd/saveview/main.go cmd/saveview/set.go cmd/saveview/set_test.go
git commit -m "feat(cli): set command for scalar edits with mandatory -o output"
```

---

## Task 12: components command

**Files:**
- Modify: `cmd/saveview/main.go`
- Create: `cmd/saveview/components.go`
- Test: `cmd/saveview/components_test.go`

**Interfaces:**
- Consumes: `gvas.Unmarshal`, `gvas.Property`, `gvas.Lookup` (gvas package).
- Produces: `func runComponents(w io.Writer, path string) error`, wired into `main()` as `saveview components <file>`.

- [ ] **Step 1: Write a failing test**

Create `cmd/saveview/components_test.go`:

```go
package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunComponentsPlayerLocal(t *testing.T) {
	var buf bytes.Buffer
	if err := runComponents(&buf, "../../testdata/Player_Local.sav"); err != nil {
		t.Fatalf("runComponents: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	// header + 10 components
	if len(lines) != 11 {
		t.Fatalf("got %d lines, want 11 (1 header + 10 components):\n%s", len(lines), buf.String())
	}
}

func TestRunComponentsNoArrayField(t *testing.T) {
	var buf bytes.Buffer
	err := runComponents(&buf, "../../testdata/WorldInfo.sav")
	if err == nil {
		t.Fatal("expected an error: WorldInfo.sav has no top-level Components array")
	}
}
```

- [ ] **Step 2: Run test, confirm compile failure**

Run: `go test ./cmd/saveview/... 2>&1 | head -20`

Expected: FAIL — `runComponents` undefined.

- [ ] **Step 3: Implement components.go**

```go
package main

import (
	"fmt"
	"io"
	"os"

	"skyversesave/gvas"
)

// runComponents prints a one-line-per-component summary of a player save's
// top-level Components array (see docs/FORMAT.md's schema for
// Player_Local.sav/Player_Remote_*.sav) — the field that makes up the bulk
// of a player save's bytes.
func runComponents(w io.Writer, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	f, err := gvas.Unmarshal(data)
	if err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}
	comps, err := gvas.Lookup(f, "Components")
	if err != nil {
		return fmt.Errorf("%s has no top-level Components array: %w", path, err)
	}
	if comps.Array == nil || comps.Array.Structs == nil {
		return fmt.Errorf("%s's Components property is not a struct array", path)
	}
	fmt.Fprintln(w, "index\tname\tclass\tbytes")
	for i, elem := range comps.Array.Structs {
		name := findFieldString(elem, "ComponentName")
		class := componentClassName(elem)
		size := componentByteSize(elem)
		fmt.Fprintf(w, "%d\t%s\t%s\t%d\n", i, name, class, size)
	}
	return nil
}

func findFieldString(props []*gvas.Property, name string) string {
	for _, p := range props {
		if p.Name == name && p.Str != nil {
			return *p.Str
		}
	}
	return "<unknown>"
}

func componentClassName(props []*gvas.Property) string {
	for _, p := range props {
		if p.Name != "Data" || p.Struct == nil {
			continue
		}
		for _, dp := range p.Struct {
			if dp.Name != "ClassName" || dp.Struct == nil {
				continue
			}
			if asset := findFieldString(dp.Struct, "AssetName"); asset != "<unknown>" {
				return asset
			}
		}
	}
	return "<unknown>"
}

func componentByteSize(props []*gvas.Property) int {
	for _, p := range props {
		if p.Name != "Data" || p.Struct == nil {
			continue
		}
		for _, dp := range p.Struct {
			if dp.Name == "Data" && dp.Array != nil {
				return len(dp.Array.Bytes)
			}
		}
	}
	return 0
}
```

- [ ] **Step 4: Wire `components` into main.go**

```go
	case "components":
		if len(os.Args) != 3 {
			fmt.Fprintln(os.Stderr, "usage: saveview components <file>")
			os.Exit(2)
		}
		if err := runComponents(os.Stdout, os.Args[2]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
```

- [ ] **Step 5: Run tests, confirm pass**

Run: `go test ./cmd/saveview/... -v`

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd/saveview/main.go cmd/saveview/components.go cmd/saveview/components_test.go
git commit -m "feat(cli): components summary command"
```

---

## Task 13: README and final verification pass

**Files:**
- Create: `README.md`

**Interfaces:** none (documentation + verification only).

- [ ] **Step 1: Write README.md**

```markdown
# skyverse-save-tool

A Go library (`gvas`) and CLI (`saveview`) for reading and editing
Skyverse `.sav` files. See `docs/FORMAT.md` for the reverse-engineered
binary format, and `docs/superpowers/specs/2026-09-18-skyverse-save-tool-design.md`
for this tool's design.

## Build

    go build ./...

## CLI usage

    saveview dump <file>                      # indented tree to stdout
    saveview json <file>                      # JSON export
    saveview get <file> <path>                # read one property
    saveview set <file> <path> <value> -o out # write an edited copy
    saveview components <file>                # summary of placed components

Paths use dot notation for struct fields and `[N]` for array indices, e.g.
`Components[3].Data.ComponentName`. `set` never modifies the input file —
`-o` is required.

## Library usage

    f, err := gvas.Unmarshal(data)
    p, err := gvas.Lookup(f, "WorldName")
    var subset struct{ WorldName string }
    err = gvas.DecodeInto(f, &subset)
    out, err := gvas.Marshal(f) // byte-identical if nothing was edited

## Testing

    go test ./...

Round-trip fidelity (`gvas.Marshal(gvas.Unmarshal(data)) == data`) is
enforced for all three files in `testdata/` — see `gvas/encode_test.go`.
```

- [ ] **Step 2: Run the full test suite**

Run: `go test ./... -v 2>&1 | tail -60`

Expected: every test across `gvas` and `cmd/saveview` PASSes.

- [ ] **Step 3: Run go vet**

Run: `go vet ./...`

Expected: no output (no issues).

- [ ] **Step 4: Full manual smoke test of every command**

```bash
go build ./...
./saveview dump testdata/Player_Local.sav | head -5
./saveview json testdata/WorldInfo.sav | head -5
./saveview get testdata/WorldInfo.sav WorldName
./saveview components testdata/Player_Local.sav
./saveview set testdata/WorldInfo.sav WorldName "Test" -o /tmp/edited.sav
./saveview get /tmp/edited.sav WorldName
```

Expected: each command runs without error and produces the expected output; the final `get` prints `Test`.

- [ ] **Step 5: Commit**

```bash
git add README.md
git commit -m "docs: add README"
```
