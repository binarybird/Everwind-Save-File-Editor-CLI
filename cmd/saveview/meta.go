package main

import (
	"fmt"
	"io"
	"os"

	"skyversesave/gvas"
)

// runMeta computes path's ".meta" sidecar content (see
// gvas.MetaChecksum) and writes it to outPath, defaulting to
// "<path>.meta" when outPath is "". It always prints the computed value
// to w too, so the value is visible even when writing to a file.
//
// Unlike `set`, path's raw bytes are checksummed directly -- there's no
// need to Unmarshal/Marshal through gvas first, since the sidecar must
// match whatever bytes are actually on disk (or about to be written
// there), not a re-encoded copy of them.
func runMeta(w io.Writer, path, outPath string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	checksum := gvas.MetaChecksum(data)
	if outPath == "" {
		outPath = path + ".meta"
	}
	if err := os.WriteFile(outPath, []byte(checksum), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", outPath, err)
	}
	fmt.Fprintf(w, "%s\nwrote %s\n", checksum, outPath)
	return nil
}
