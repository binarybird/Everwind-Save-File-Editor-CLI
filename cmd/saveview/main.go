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
  saveview components <file>
  saveview meta <file> [-o out.meta]`)
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
		file, outPath, err := parseJSONArgs(os.Args[2:])
		if err != nil {
			fmt.Fprintln(os.Stderr, "usage: saveview json <file> [-o out.json]")
			os.Exit(2)
		}
		if outPath != "" {
			err = runJSONToFile(file, outPath)
		} else {
			err = runJSON(os.Stdout, file)
		}
		if err != nil {
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
		file, propPath, rawValue, outPath, err := parseSetArgs(os.Args[2:])
		if err != nil {
			fmt.Fprintln(os.Stderr, "usage: saveview set <file> <path> <value> -o <out>")
			os.Exit(2)
		}
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
	case "meta":
		positional, outPath, _, err := extractOutFlag(os.Args[2:])
		if err != nil || len(positional) != 1 {
			fmt.Fprintln(os.Stderr, "usage: saveview meta <file> [-o out.meta]")
			os.Exit(2)
		}
		if err := runMeta(os.Stdout, positional[0], outPath); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

// extractOutFlag scans args for a "-o <value>" pair, which may appear
// anywhere in the list (not just trailing the positional arguments) --
// the standard library's flag package doesn't support flags that trail
// positional arguments, which is exactly saveview's `set`/`json` syntax,
// so both commands parse their argv by hand. It returns the remaining
// positional arguments (with "-o" and its value removed, in their
// original relative order), the flag's value, and whether the flag was
// present at all. An "-o" with nothing following it is an error, not a
// panic.
func extractOutFlag(args []string) (positional []string, outPath string, found bool, err error) {
	for i := 0; i < len(args); i++ {
		if args[i] == "-o" {
			if i+1 >= len(args) {
				return nil, "", false, fmt.Errorf("-o requires a value")
			}
			positional = append(positional, args[:i]...)
			positional = append(positional, args[i+2:]...)
			return positional, args[i+1], true, nil
		}
	}
	return args, "", false, nil
}

// parseJSONArgs parses the arguments following "saveview json", i.e.
// os.Args[2:]: exactly one positional file argument, plus an optional
// "-o out.json" pair anywhere in the list. outPath is "" when -o was not
// given, meaning "write to stdout".
func parseJSONArgs(args []string) (file, outPath string, err error) {
	positional, out, _, err := extractOutFlag(args)
	if err != nil {
		return "", "", err
	}
	if len(positional) != 1 {
		return "", "", fmt.Errorf("expected exactly one file argument, got %d", len(positional))
	}
	return positional[0], out, nil
}

// parseSetArgs parses the arguments following "saveview set", i.e.
// os.Args[2:]: exactly three positional arguments (file, path, value)
// plus a mandatory "-o out" pair anywhere in the list.
func parseSetArgs(args []string) (file, path, value, outPath string, err error) {
	positional, out, found, err := extractOutFlag(args)
	if err != nil {
		return "", "", "", "", err
	}
	if !found || out == "" {
		return "", "", "", "", fmt.Errorf("an output path (-o) is required")
	}
	if len(positional) != 3 {
		return "", "", "", "", fmt.Errorf("expected exactly 3 positional arguments (file, path, value), got %d", len(positional))
	}
	return positional[0], positional[1], positional[2], out, nil
}
