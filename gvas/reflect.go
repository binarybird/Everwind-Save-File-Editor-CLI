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
