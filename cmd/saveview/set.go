package main

import (
	"fmt"
	"os"
	"strconv"

	"skyversesave/gvas"
)

// runSet loads path, resolves propPath, parses rawValue against that
// property's existing scalar kind, and writes the edited file to outPath.
// The input file at path is never modified — see docs/superpowers/specs/2026-09-18-skyverse-save-tool-design.md,
// "`set` requires `-o`".
func runSet(path, propPath, rawValue, outPath string) error {
	if outPath == "" {
		return fmt.Errorf("an output path (-o) is required; the input file is never overwritten in place")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	f, err := gvas.Unmarshal(data)
	if err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}
	p, err := gvas.Lookup(f, propPath)
	if err != nil {
		return err
	}
	if err := applyScalarEdit(p, rawValue); err != nil {
		return fmt.Errorf("setting %s: %w", propPath, err)
	}
	out, err := gvas.Marshal(f)
	if err != nil {
		return fmt.Errorf("encoding edited save: %w", err)
	}
	if err := os.WriteFile(outPath, out, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", outPath, err)
	}
	return nil
}

func applyScalarEdit(p *gvas.Property, rawValue string) error {
	switch {
	case p.Str != nil:
		return p.SetString(rawValue)
	case p.Bool != nil:
		v, err := strconv.ParseBool(rawValue)
		if err != nil {
			return fmt.Errorf("parsing %q as bool: %w", rawValue, err)
		}
		return p.SetBool(v)
	case p.Int32 != nil:
		v, err := strconv.ParseInt(rawValue, 10, 32)
		if err != nil {
			return fmt.Errorf("parsing %q as int32: %w", rawValue, err)
		}
		return p.SetInt32(int32(v))
	case p.Int64 != nil:
		v, err := strconv.ParseInt(rawValue, 10, 64)
		if err != nil {
			return fmt.Errorf("parsing %q as int64: %w", rawValue, err)
		}
		return p.SetInt64(v)
	case p.Float32 != nil:
		v, err := strconv.ParseFloat(rawValue, 32)
		if err != nil {
			return fmt.Errorf("parsing %q as float32: %w", rawValue, err)
		}
		return p.SetFloat32(float32(v))
	case p.Float64 != nil:
		v, err := strconv.ParseFloat(rawValue, 64)
		if err != nil {
			return fmt.Errorf("parsing %q as float64: %w", rawValue, err)
		}
		return p.SetFloat64(v)
	default:
		return fmt.Errorf("property %q (type %s) is not an editable scalar in this version of saveview", p.Name, p.Type)
	}
}
