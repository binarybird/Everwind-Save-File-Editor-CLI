package main

import (
	"fmt"
	"io"
	"os"

	"github.com/binarybird/Everwind-Save-File-Editor-CLI/gvas"
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
	if comps.Array == nil || comps.Array.InnerType.Value != "StructProperty" {
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
