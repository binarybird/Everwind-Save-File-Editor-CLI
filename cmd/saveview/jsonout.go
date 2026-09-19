package main

import (
	"bytes"
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
	Name   string     `json:"name"`
	Type   string     `json:"type"`
	Value  any        `json:"value,omitempty"`
	Fields []jsonProp `json:"fields,omitempty"`
	Elems  []jsonElem `json:"elements,omitempty"`
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
			jp.Fields = toJSONProps(p.Struct)
		case p.NestedFile != nil:
			jp.Fields = toJSONProps(p.NestedFile.Root)
		case p.Array != nil && p.Array.Structs != nil:
			for _, elem := range p.Array.Structs {
				jp.Elems = append(jp.Elems, jsonElem{Fields: toJSONProps(elem)})
			}
		default:
			jp.Value = jsonLeafValue(p)
		}
		out = append(out, jp)
	}
	return out
}

// jsonLeafValue projects a scalar (or scalar-array) Property into a real
// machine-readable JSON value, rather than dump.go's human-display
// formatValue: numbers become JSON numbers, bools become JSON booleans,
// and scalar arrays (int/float/double/int64/bool/string) become real JSON
// arrays of the matching native type. This is intentionally a separate
// contract from formatValue/dump.go -- dump is a human tree display, json
// is a machine export -- so they are not shared even where the logic looks
// similar. Anything left over (ObjectProperty/Raw, native structs, a plain
// ByteProperty, a raw/byte-inner array) keeps formatValue's existing
// byte-count/placeholder string; that's an intentionally opaque summary,
// not lossy machine data.
func jsonLeafValue(p *gvas.Property) any {
	switch {
	case p.Int32 != nil:
		return *p.Int32
	case p.Int64 != nil:
		return *p.Int64
	case p.Float32 != nil:
		return *p.Float32
	case p.Float64 != nil:
		return *p.Float64
	case p.Bool != nil:
		return *p.Bool
	case p.Str != nil:
		return *p.Str
	case p.Array != nil:
		return jsonArrayValue(p.Array)
	default:
		return formatValue(p)
	}
}

// jsonArrayValue expands a scalar ArrayValue into a real JSON array of the
// matching native type. ArrayValue.Bytes (raw byte arrays, e.g. a
// ByteProperty-inner array that didn't decode as a NestedFile) and any
// other unrecognized inner type stay as an opaque placeholder string --
// those aren't meaningful per-element scalar data the way skill levels,
// recipe flags, or name lists are.
func jsonArrayValue(a *gvas.ArrayValue) any {
	switch {
	case a.Ints != nil:
		return a.Ints
	case a.Int64s != nil:
		return a.Int64s
	case a.Floats != nil:
		return a.Floats
	case a.Doubles != nil:
		return a.Doubles
	case a.Bools != nil:
		return a.Bools
	case a.Strings != nil:
		return a.Strings
	default:
		return fmt.Sprintf("<%d elements>", arrayLen(a))
	}
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

// runJSONToFile is runJSON's -o counterpart: it writes the JSON encoding of
// path to outPath instead of an io.Writer, matching saveview's advertised
// `saveview json <file> [-o out.json]` syntax.
func runJSONToFile(path, outPath string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	f, err := gvas.Unmarshal(data)
	if err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(toJSONProps(f.Root)); err != nil {
		return err
	}
	if err := os.WriteFile(outPath, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", outPath, err)
	}
	return nil
}
