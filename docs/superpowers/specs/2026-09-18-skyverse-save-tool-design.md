# Skyverse save tool — design

Status: approved by user 2026-09-18 (round-trip read/write + CLI edit
commands included in scope, per user's answers during brainstorming).

## Background

`docs/FORMAT.md` documents the reverse-engineered binary format used by
this game's `.sav` files (a custom, non-`GVAS` Unreal Engine tagged
property list, reused recursively for embedded object blobs). That
document is the authority on the wire format; this document covers what
we're building on top of it and the decisions specific to the tool.

Goal: a Go library that can decode any of these save files into an
inspectable/editable form and re-encode it byte-identically when nothing
changed, plus a CLI for viewing and making simple edits.

## Module layout

```
skyverse-save-tool/            (module: skyversesave)
  gvas/                          generic marshaler package
    reader.go                      low-level binary reader (FString, "FName-like", primitives)
    writer.go                      low-level binary writer (mirror of reader.go)
    property.go                    Property tree types
    decode.go                      Unmarshal(data []byte) (*File, error)
    encode.go                      Marshal(f *File) ([]byte, error)
    nativestruct.go                 Vector/IntVector/Rotator/DateTime/Timespan/Color/LinearColor codecs
    reflect.go                       DecodeInto(f *File, v any) error / EncodeInto(f *File, v any) error
    path.go                           property-path parsing/lookup shared by reflect.go and the CLI
  cmd/saveview/
    main.go                        CLI entrypoint + subcommand dispatch
    dump.go                          `dump` command
    jsonout.go                       `json` command
    get.go                            `get` command
    set.go                             `set` command
    components.go                      `components` command
  testdata/
    Player_Local.sav
    Player_Remote_021fa7902e50eeeb8c6ebdbe4fc922fe.sav
    WorldInfo.sav
  go.mod
```

No third-party dependencies — everything (binary I/O, JSON output, CLI
arg parsing) is doable with the standard library at this size, and it
keeps the tool a single `go build` away from running anywhere.

## `gvas` package

### Property tree

```go
type File struct {
    HeaderByte byte       // observed always 0x00; preserved verbatim
    Root       []*Property
    Footer     []byte      // observed always 4 zero bytes; preserved verbatim
}

type Property struct {
    Name       string
    Type       string        // e.g. "StrProperty", "StructProperty"
    ArrayIndex int32

    // Type-specific extra header, kept as parsed. Only StructProperty,
    // ByteProperty, EnumProperty, ArrayProperty/SetProperty, and
    // MapProperty carry any of this; everything else leaves it nil.
    Extra ExtraHeader

    RawTagGuid []byte    // 16 bytes if a property guid was present, else nil
    GuidMarker uint8     // preserved verbatim for round-trip; only byte value 1 means RawTagGuid is set

    // Exactly one of these is populated, chosen by Type:
    Bool    *bool
    Str     *string          // StrProperty, NameProperty, EnumProperty (formatted "Type::Value")
    Int32   *int32
    Int64   *int64
    Float32 *float32
    Float64 *float64
    Struct  []*Property      // non-native StructProperty, or the decoded fields of a native one is instead:
    Native  *NativeValue     // Vector/IntVector/Rotator/DateTime/Timespan/Color/LinearColor, when StructName is recognized
    Array   *ArrayValue      // ArrayProperty/SetProperty
    Raw     []byte           // fallback: ObjectProperty, unrecognized types, or a struct/array we chose not to interpret

    // NestedFile is populated only when Type=="ArrayProperty", Extra.InnerType=="ByteProperty",
    // and the bytes were successfully parsed as another embedded [header][property list][footer]
    // per the "Nested blobs" section of docs/FORMAT.md. When set, Raw is unused for this node.
    NestedFile *File
}
```

`ArrayValue` holds `InnerType` plus one of: `[]*Property` per element (struct
inner), or a typed Go slice (`[]bool`, `[]int32`, `[]float32`, `[]float64`,
`[]string`), or `[]byte` (byte inner), or `Raw []byte` for any inner type
we don't specifically decode (kept for round-trip).

This is a *generic* tree — it works for any save without hardcoding game
schemas, per `docs/FORMAT.md`'s note that several struct types
(`ExploredIslandSaveData`, `FlagMarkerData`, etc.) were never fully
enumerated.

### Decode (`Unmarshal`)

Implements the algorithm in `docs/FORMAT.md` exactly: skip the header
byte, read the property list recursively, always advancing by the tag's
declared `Size` for anything not specifically decoded, and reading the
trailing 4-byte footer. Every property that's read into a typed field also
keeps enough information to be re-encoded losslessly (see Encode).

On the extra-header assumptions that aren't fully proven (documented in
`docs/FORMAT.md`'s Implementation Notes — unknown flag-int semantics,
`TextProperty`/`MapProperty` layout untested), `Unmarshal` returns a
wrapped error identifying the byte offset and the property path where
parsing went off the rails, rather than panicking or silently returning
garbage. A malformed/unsupported save should fail loudly and early.

### Encode (`Marshal`)

Walks the tree bottom-up. For any node whose typed value came straight
from `Unmarshal` and was never mutated, this is a byte-for-byte replay
(tag header bytes are cached at parse time in an unexported field on
`Property` specifically for this purpose). For a mutated scalar
(`Bool`/`Str`/`Int32`/`Int64`/`Float32`/`Float64`), `Marshal` re-encodes
just that value and recomputes `Size` for that tag; recomputed sizes
propagate upward (a changed string inside a struct changes that struct's
`Size`, which changes its parent's `Size`, etc.).

Editing is intentionally scoped to scalar leaf values for v1 — no support
for adding/removing array elements or properties, since that would require
generating tag headers (extra fields, guid markers) we can't always fully
justify from first principles. `EncodeFrom`/`set` return a clear error if
asked to do something out of scope.

**Correctness bar**: a test asserts `Marshal(Unmarshal(data)) == data`
byte-for-byte for all three files in `testdata/`, both at the top level
and recursively for every successfully-decoded nested blob.

### Typed reflection layer (`DecodeInto` / `EncodeInto`)

Mirrors `encoding/json`: a struct field tagged `` `save:"FieldName"` ``
(defaulting to the Go field name if untagged) is matched against a
`Property.Name` in the tree at that level. Supported field kinds: the
scalar types above, `string`, nested struct (matches a non-native
`StructProperty`), and slices (matches `ArrayProperty`, element type must
correspond to the array's `InnerType`). This is a convenience layered on
top of the tree, not a replacement for it — the CLI itself uses the
generic tree directly, since it must handle any save without knowing
its schema up front.

### Property paths

A single path syntax used by both `get`/`set` and internally:
`Components[3].Data.ComponentName`, `PlayerMarkerSettings.Filters[0]`. Dot
for struct-field descent, `[N]` for array indexing (0-based). Descending
into a `NestedFile` (an embedded blob) uses the same dot syntax as if it
were an ordinary nested struct, e.g.
`Components[0].Data.Data.BaseStats[0].CurrentValue` — the CLI resolves
across the `NestedFile` boundary transparently since `docs/FORMAT.md`
established that's just another property list.

## CLI (`cmd/saveview`)

```
saveview dump <file>                      indented tree to stdout
saveview json <file> [-o out.json]        JSON export, nested blobs expanded inline
saveview get <file> <path>                print one property's value
saveview set <file> <path> <value> -o <out>
                                           parse <value> against the target
                                           property's existing type, write a
                                           new file to <out> (original is
                                           never modified in place)
saveview components <file>                summary table: name, class, byte size,
                                           per placed component (the obviously-interesting
                                           bulk of player saves per docs/FORMAT.md)
```

`set` requires `-o`/an explicit output path — never overwrites the input
save. Booleans accept `true`/`false`; numeric types are parsed with
`strconv`; strings are taken verbatim. Setting a value whose `Property`
doesn't resolve to a supported scalar kind (a native struct field,
inside a `Raw` fallback, etc.) is a clear error naming what's not
editable yet, not a partial/corrupt write.

## Testing plan

- `gvas` package: table-driven decode tests per primitive (FString
  positive/negative length, FName-like, each native struct) using
  synthetic byte buffers — not just the real save files — so specific
  edge ccases (empty string, negative/UTF-16 string) are covered
  independently of what happens to appear in the 3 samples.
- Golden round-trip test: `Marshal(Unmarshal(testdata/X.sav)) == read(testdata/X.sav)`
  for all three files.
- `set` integration test: change a known scalar (e.g.
  `WorldName` in `WorldInfo.sav`), assert the written file still parses,
  the new value reads back correctly, and every *other* top-level
  property is byte-identical to the original.
- CLI smoke tests using `testdata/` via `os/exec` or by calling the
  command functions directly.

## Out of scope for this pass

- Adding/removing/reordering properties or array elements.
- `MapProperty` and `TextProperty` support (never observed populated;
  will error clearly rather than guess).
- An interactive/TUI browser — `dump`/`json`/`get` cover viewing for now.
