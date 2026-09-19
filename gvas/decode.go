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
