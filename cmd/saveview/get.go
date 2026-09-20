package main

import (
	"fmt"
	"io"
	"os"

	"github.com/binarybird/Everwind-Save-File-Editor-CLI/gvas"
)

func runGet(w io.Writer, path, propPath string) error {
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
	fmt.Fprintln(w, formatValue(p))
	return nil
}
