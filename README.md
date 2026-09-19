# skyverse-save-tool

A Go library (`gvas`) and CLI (`saveview`) for reading and editing
Skyverse `.sav` files. See `docs/FORMAT.md` for the reverse-engineered
binary format, and `docs/superpowers/specs/2026-09-18-skyverse-save-tool-design.md`
for this tool's design.

## Build

    go build ./...

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
