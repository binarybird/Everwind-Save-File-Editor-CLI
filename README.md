# skyverse-save-tool

A Go library (`gvas`) and CLI (`saveview`) for reading and editing
Skyverse `.sav` files. See `docs/FORMAT.md` for the reverse-engineered
binary format, and `docs/superpowers/specs/2026-09-18-skyverse-save-tool-design.md`
for this tool's design.

## Build

    go build ./cmd/saveview

(`go build ./...` also works, but only as a "does everything compile"
check — with more than one package in the module it doesn't leave a
`saveview` binary behind on its own.)

## CLI usage

    saveview dump <file>                      # indented tree to stdout
    saveview json <file>                      # JSON export
    saveview get <file> <path>                # read one property
    saveview set <file> <path> <value> -o out # write an edited copy
    saveview components <file>                # summary of placed components

Paths use dot notation for struct fields and `[N]` for array indices, e.g.
`Components[3].Data.ComponentName`. `set` never modifies the input file —
`-o` is required.

## Library usage

    f, err := gvas.Unmarshal(data)
    p, err := gvas.Lookup(f, "WorldName")
    var subset struct{ WorldName string }
    err = gvas.DecodeInto(f, &subset)
    out, err := gvas.Marshal(f) // byte-identical if nothing was edited

## Testing

    go test ./...

Round-trip fidelity (`gvas.Marshal(gvas.Unmarshal(data)) == data`) is
enforced for all three files in `testdata/` — see `gvas/encode_test.go`.

One known gap: strings are re-encoded as ASCII or UTF-16LE based on the
*decoded content* (pure-ASCII vs. not), not the encoding actually read from
the source file, so a string whose source encoding was UTF-16 but whose
content happens to be representable in pure ASCII will be re-encoded as
ASCII on `Marshal` — semantically identical, but byte-different from the
input. This has not been observed in any of the three files in `testdata/`.
