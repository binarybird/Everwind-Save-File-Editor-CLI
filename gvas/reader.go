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
