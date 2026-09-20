# skyverse-save-tool

A Go library (`gvas`) and command-line tool (`saveview`) for reading and
editing **Skyverse**'s `.sav` save files — the same binary format used
for player saves, world state, and flying-island saves. Built by
reverse-engineering the format from scratch (documented in full in
`docs/FORMAT.md`); there's no official spec or SDK for it.

The core promise: `gvas.Marshal(gvas.Unmarshal(data))` reproduces `data`
byte-for-byte when nothing was edited. Every property type the format
uses — structs, arrays, native engine types (vectors, colors,
timestamps), even a "nested blob" encoding pattern the game uses for
some component data — round-trips exactly, so editing one field never
silently corrupts or reformats anything else in the file.

## What it can do

- **Read anything.** Dump a save's entire property tree as indented
  text or JSON, or look up a single value by path.
- **Edit scalars.** Strings, bools, integers (32/64-bit), and floats
  (32/64-bit) can be set by path, including item references
  (`ObjectProperty` fields like a slot's equipped item).
- **Insert/remove struct-array elements.** Used by the companion
  [`Everwind-Save-File-Editor-Web-UI`](https://github.com/binarybird/Everwind-Save-File-Editor-Web-UI/)
  project to add or remove items from inventory slots —
  `AppendStructElement`/`RemoveStructElement` on any
  `StructProperty`-inner array.
- **Generate the `.meta` checksum sidecar** a save needs to actually be
  accepted by the game (see "Why the `.meta` file matters" below).

## Build

    go build ./cmd/saveview

(`go build ./...` also works, but only as a "does everything compile"
check — with more than one package in the module it doesn't leave a
`saveview` binary behind on its own.)

## CLI usage

    saveview dump <file>                      # indented property tree to stdout
    saveview json <file> [-o out.json]        # JSON export
    saveview get <file> <path>                # read one property
    saveview set <file> <path> <value> -o out # write an edited copy
    saveview components <file>                # summary of placed save components
    saveview meta <file> [-o out.meta]        # write the .meta checksum sidecar

Paths use dot notation for struct fields and `[N]` for array indices,
e.g. `Components[3].Data.ComponentName` or
`Components[1].Data.Data.Slots[0].Items[0].BaseData`. `set` never
modifies the input file in place — `-o` is required.

Example: change a world's name in a copy, then generate that copy's
checksum sidecar so the game will actually accept it:

    saveview set WorldInfo.sav WorldName "New World Name" -o WorldInfo-edited.sav
    saveview meta WorldInfo-edited.sav
    # -> writes WorldInfo-edited.sav.meta next to it

Then copy both `WorldInfo-edited.sav` and `WorldInfo-edited.sav.meta`
into the game's save directory as `WorldInfo.sav` /
`WorldInfo.sav.meta` (your original `WorldInfo.sav` here in the working
directory is untouched throughout).

## Why the `.meta` file matters

Every `<name>.sav` in the game's save directory has a sibling
`<name>.sav.meta` — a tiny text file containing nothing but the `.sav`
file's CRC32 checksum, as a plain decimal number. **The game recomputes
that checksum every time it loads a save and compares it against the
`.meta` file.** If they don't match, it silently discards the file and
falls back to its own `<name>.sav.backup` instead — no error, no
prompt, just your edit quietly reverted the next time you play.

Since nothing outside the game itself ever had reason to write a
`.sav.meta` file before, **any `.sav` you edit with this tool (or any
other tool) needs its `.meta` sidecar regenerated to match**, or the
edit will never actually take effect in-game. `saveview meta` does
exactly that:

    saveview set Player_Local.sav Components[1].Data.Data.Slots[0].SlotsWithItems 99 -o Player_Local-edited.sav
    saveview meta Player_Local-edited.sav
    # -> writes Player_Local-edited.sav.meta next to it, with the matching checksum

Then copy both `Player_Local-edited.sav` and
`Player_Local-edited.sav.meta` into the game's save directory, renamed
to `Player_Local.sav` / `Player_Local.sav.meta` (overwriting the
existing pair), before launching.

## Library usage

```go
f, err := gvas.Unmarshal(data)
p, err := gvas.Lookup(f, "WorldName")
err = p.SetString("New World Name")

var subset struct{ WorldName string }
err = gvas.DecodeInto(f, &subset) // read several fields into a struct at once

out, err := gvas.Marshal(f)          // byte-identical to data if nothing was edited
meta := gvas.MetaChecksum(out)       // the string to write as out's ".meta" sidecar
```

## Testing

    go test ./...

Round-trip fidelity (`gvas.Marshal(gvas.Unmarshal(data)) == data`) is
enforced for all three files in `testdata/` — see `gvas/encode_test.go`.

One known gap: strings are re-encoded as ASCII or UTF-16LE based on the
*decoded content* (pure-ASCII vs. not), not the encoding actually read
from the source file, so a string whose source encoding was UTF-16 but
whose content happens to be representable in pure ASCII will be
re-encoded as ASCII on `Marshal` — semantically identical, but
byte-different from the input. This has not been observed in any of the
three files in `testdata/`.
