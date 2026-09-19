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
	case "set":
		// Parse manually to allow -o flag anywhere in the argument list
		var file, propPath, rawValue, outPath string
		var outIdx int
		var foundOut bool

		// Look for -o flag and its value
		for i := 2; i < len(os.Args)-1; i++ {
			if os.Args[i] == "-o" {
				outPath = os.Args[i+1]
				foundOut = true
				outIdx = i
				break
			}
		}

		if !foundOut || outPath == "" {
			fmt.Fprintln(os.Stderr, "usage: saveview set <file> <path> <value> -o <out>")
			os.Exit(2)
		}

		// Collect positional arguments (skip the -o and its value)
		var args []string
		for i := 2; i < len(os.Args); i++ {
			if i == outIdx || i == outIdx+1 {
				continue
			}
			args = append(args, os.Args[i])
		}

		if len(args) != 3 {
			fmt.Fprintln(os.Stderr, "usage: saveview set <file> <path> <value> -o <out>")
			os.Exit(2)
		}

		file = args[0]
		propPath = args[1]
		rawValue = args[2]

		if err := runSet(file, propPath, rawValue, outPath); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case "components":
		if len(os.Args) != 3 {
			fmt.Fprintln(os.Stderr, "usage: saveview components <file>")
			os.Exit(2)
		}
		if err := runComponents(os.Stdout, os.Args[2]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}
