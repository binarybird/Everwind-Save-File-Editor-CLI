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
	case "dump":
		if len(os.Args) != 3 {
			fmt.Fprintln(os.Stderr, "usage: saveview dump <file>")
			os.Exit(2)
		}
		if err := runDump(os.Stdout, os.Args[2]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case "json":
		if len(os.Args) != 3 {
			fmt.Fprintln(os.Stderr, "usage: saveview json <file>")
			os.Exit(2)
		}
		if err := runJSON(os.Stdout, os.Args[2]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case "get":
		if len(os.Args) != 4 {
			fmt.Fprintln(os.Stderr, "usage: saveview get <file> <path>")
			os.Exit(2)
		}
		if err := runGet(os.Stdout, os.Args[2], os.Args[3]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}
