package main

import (
	"fmt"
	"io"
	"os"

	"github.com/binarybird/Everwind-Save-File-Editor-CLI/gvas"
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
