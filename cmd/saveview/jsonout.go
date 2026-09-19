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
