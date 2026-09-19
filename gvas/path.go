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

func findProp(props []*Property, name string) *Property {
	for _, p := range props {
		if p.Name == name {
			return p
		}
	}
	return nil
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
