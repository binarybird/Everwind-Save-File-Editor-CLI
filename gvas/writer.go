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
