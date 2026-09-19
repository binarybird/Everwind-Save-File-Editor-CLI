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
		// Preserve the exact original byte (e.g. 0x10, not just 0x01 --
		// see docs/FORMAT.md) whenever it still agrees with the current
		// semantic value; only fall back to canonical 0/1 once SetBool
		// has actually flipped the value.
		b := p.BoolRaw
		if (b != 0) != *p.Bool {
			b = 0
			if *p.Bool {
				b = 1
			}
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
