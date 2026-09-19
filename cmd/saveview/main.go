package main

import (
	"fmt"
	"os"
)

func usage() {
	fmt.Fprintln(os.Stderr, `saveview - view and edit Skyverse .sav files

Usage:
  saveview dump <file>
  saveview json <file> [-o out.json]
  saveview get <file> <path>
  saveview set <file> <path> <value> -o <out>
  saveview components <file>`)
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}
