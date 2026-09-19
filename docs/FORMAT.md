# Skyverse save file format analysis

Reverse-engineered from three save files copied into this directory:

- `Player_Local.sav` (1,061,290 bytes)
- `Player_Remote_021fa7902e50eeeb8c6ebdbe4fc922fe.sav` (896,541 bytes)
- `WorldInfo.sav` (416,007 bytes)

Each `.sav` file has a companion `.sav.meta` file containing a single ASCII
decimal number (e.g. `1611657685`) and no line terminator — almost
certainly a Unix timestamp or checksum for the save slot. Not investigated
further; irrelevant to the save data itself.

The game appears to be built in **Unreal Engine 5** (class paths reference
`/Script/Skyverse` and `/Script/CoreUObject`; `FVector` components are
serialized as `double`s, indicating Large World Coordinates / UE5). The save
files are **not** standard `UGameplayStatics::SaveGameToSlot` output — there
is no `GVAS` magic header, engine-version block, or `SaveGameClassName`.
Instead, the game has a custom save routine that serializes a raw Unreal
**tagged property list** directly to disk. The same low-level serializer is
reused recursively for embedded blobs (see "Nested blobs" below).

All three files use the exact same format and were parsed byte-for-byte
successfully (0 unexplained bytes) with the algorithm described here.

## Top-level structure

```
[1 byte header]  [Property List]  [4-byte trailing footer]
```

- **Header byte**: always observed as `0x00`. Purpose unknown (possibly a
  format/version byte, or an "is compressed" flag that's always false in
  these samples). Must be skipped unconditionally before parsing starts.
- **Property List**: see below.
- **Trailing footer**: always observed as `00 00 00 00` (4 zero bytes)
  immediately after the list's closing `None` tag. Purpose unknown —
  possibly a checksum/reserved field that's unused, or simple padding.

This exact same 3-part wrapper (1-byte header, property list, 4-byte zero
footer) also appears **inside** the file, embedded in opaque byte arrays —
see "Nested blobs".

## Primitive encodings

### FString

```
int32 Length          // signed, little-endian
if Length == 0:  ""    (no further bytes)
if Length  > 0:  Length bytes of ASCII/UTF-8, INCLUDING a trailing \0
if Length  < 0:  -Length UTF-16LE code units, INCLUDING a trailing \0
```

### "FName-like" field

Used for type/struct/package names inside property tag headers (never for
ordinary property values). Empirically:

```
int32 Number     // usually 1, sometimes higher (e.g. 2) — not always 0/1,
                  // meaning is not fully understood (possibly an FName
                  // instance/collision number), but it's always exactly
                  // 4 bytes and must simply be skipped.
FString Value
```

A plain `NameProperty`'s *value* (as opposed to a type-tag name) is just a
bare `FString` — no `Number` prefix.

## Property list

A sequence of property tags, terminated by a tag whose Name is `"None"`
(with no further fields for that terminating tag).

## Property tag

```
FString Name
FString Type                     // e.g. "StrProperty", "StructProperty", ...

// --- type-specific "extra" header (see table below) ---

int32   ArrayIndex                // usually 0
int32   Size                      // byte length of Value, see per-type notes

// --- BoolProperty is special: value is inline, no guid marker/guid ever ---
if Type == "BoolProperty":
    uint8 Value                   // 0 = false, any nonzero = true (seen: 0x01, 0x10)
else:
    uint8 GuidMarker
    if GuidMarker == 1:
        16 bytes PropertyGuid
    <Size> bytes of Value
```

`GuidMarker` was seen as `0` (StrProperty/plain values) and `8` (every
native-struct value we found: Vector, IntVector). Only a literal `1` should
be treated as "guid follows" — no `!= 0` case was ever hit that also
carried a real guid in these files, so this behaviour is inferred from
consistency across ~2,700 properties, not from a directly observed
guid. Flag as best-effort if a game update introduces real property guids.

### Type-specific extra header

| Type | Extra fields (in order) |
|---|---|
| `StructProperty` | `StructName` (FName-like), `PackagePath` (FName-like) |
| `ByteProperty` | `EnumName` (FName-like) — `"None"` if it's a plain byte |
| `EnumProperty` | `EnumTypeName` (FName-like), `EnumPackagePath` (FName-like), `UnderlyingTypeName` (FName-like, e.g. `"ByteProperty"`) |
| `ArrayProperty` / `SetProperty` | `InnerType` (FName-like); **if** `InnerType == "StructProperty"`: additionally `InnerStructName` (FName-like), `InnerStructPackagePath` (FName-like) |
| `MapProperty` | `KeyType` (FName-like), `ValueType` (FName-like) — untested, inferred by symmetry, no `MapProperty` occurred in these 3 files |
| everything else (`IntProperty`, `FloatProperty`, `DoubleProperty`, `StrProperty`, `NameProperty`, `ObjectProperty`, `Int64Property`, ...) | none |

`TextProperty` never appeared in any of the three files. Real Unreal
`FText` tags carry extra flag/namespace/key data before the value; if a
save ever contains one, the generic "no extra header" assumption will
desync the parser. The Go tool should catch this (Size-bounded resync,
see Implementation Notes) rather than crash.

### Value decoding by type

| Type | Value format |
|---|---|
| `BoolProperty` | inline in the tag (see above), `Size` is always `0` |
| `StrProperty`, `NameProperty` | `FString` |
| `IntProperty` | `int32` LE |
| `UInt32Property` | `uint32` LE (not observed, inferred) |
| `Int64Property` | `int64` LE (not observed, inferred) |
| `FloatProperty` | `float32` LE |
| `DoubleProperty` | `float64` LE |
| `ByteProperty` | single `uint8` when `Size == 1` (plain byte); when the tag's `EnumName` extra isn't `"None"` it may instead be serialized like `EnumProperty` — not observed populated in these files, only as an empty array inner type |
| `EnumProperty` | `FString`, formatted `"EnumTypeName::ValueName"`, e.g. `"EModificatorType::MT_Add"` |
| `ObjectProperty` | opaque `Size` bytes — observed sizes 4 (likely a null/none reference) and 85–123 (likely a soft object path `FString`-like reference); not fully decoded, treat as raw/hex in the viewer |
| `StructProperty` | nested Property List if `StructName` is **not** in the native-struct set below; otherwise `Size` raw bytes with a fixed native layout (see below) |
| `ArrayProperty` / `SetProperty` | `int32 Count`, then `Count` elements (see below) |

### Native (non-nested) struct types

These structs are NOT serialized as their own tagged property list — their
value is `Size` bytes of packed native fields, decoded directly:

| StructName | Size (bytes) | Layout |
|---|---|---|
| `Vector` | 24 | 3× `double` (x, y, z) — UE5 LWC |
| `IntVector` | 12 | 3× `int32` (x, y, z) |
| `Rotator` | 24 | 3× `double` (pitch, yaw, roll) |
| `DateTime` | 8 | `int64` .NET-style ticks (100ns units since 0001-01-01) |
| `Timespan` | 8 | `int64` ticks, same unit |
| `Color` | 4 (inferred) | 4× `uint8`, likely B,G,R,A order (untested — no populated sample) |
| `LinearColor` | 16 (inferred) | 4× `float32` R,G,B,A (untested — no populated sample) |

Any struct name **not** in this list is treated as a nested property list
(recurse), which is how `WorldLocation`, `UDSSaveData`, `ItemSlotSaveData`,
`ComponentSaveData`, `TopLevelAssetPath`, etc. are handled — they are
ordinary game-defined structs, not engine primitives.

Crucially, **the parser never needs to hardcode a struct's byte size** for
skipping purposes — `Size` in the tag always tells you exactly how many
bytes the value occupies, regardless of type. The native-struct table above
is only needed to decode those bytes into meaningful fields for display; if
a struct name is unknown, falling back to "show `Size` raw/hex bytes" is
always structurally safe.

### Array/Set value format

```
int32 Count
Count × element
```

Element format depends on `InnerType`:

- `StructProperty`: each element is a **full nested property list**
  (its own `None` terminator), using `InnerStructName` as the implied
  struct type for every element.
- `BoolProperty` / `ByteProperty`: 1 byte per element (packed, no tags)
- `IntProperty` / `UInt32Property`: 4 bytes per element
- `FloatProperty`: 4 bytes per element
- `DoubleProperty`: 8 bytes per element
- `Int64Property`: 8 bytes per element (inferred)
- `StrProperty` / `NameProperty`: one `FString` per element
- `ObjectProperty`: not observed populated (only empty, `Count == 0`) —
  unknown per-element layout, treat as unsupported/raw

This was validated exactly: summing `4 (count) + Count × elementSize` for
every array in all three files reproduces the tag's declared `Size` with
zero discrepancy (validated in aggregate across `Player_Local.sav`'s
10-element `Components` array down to the byte, and across `WorldInfo.sav`'s
several hundred `ArrayProperty` occurrences).

## Nested blobs (the interesting part)

Several `StructProperty` fields named `Data` of struct type
`UObjectSaveData` contain a field also named `Data`, typed
`ArrayProperty<ByteProperty>` — i.e., a raw byte array. This is **not**
opaque binary junk: every single one of these byte blobs, in every
component of every player save tested, is itself a complete instance of
the top-level wrapper format:

```
[1 byte header (0x00)] [Property List] [4-byte trailing footer (0x00000000)]
```

i.e. it's the game reusing its generic "serialize this UObject's tagged
properties to a byte buffer" routine both for the top-level save file and
for each individual placed/spawned game object's own property state. This
was verified on all 10 components in `Player_Local.sav`: every blob parses
cleanly to its declared length, leaving exactly the same 4 trailing zero
bytes each time.

**Practical implication for the viewer**: any `ArrayProperty<ByteProperty>`
value should be offered as "decode as nested property list" — attempt it,
and if it parses cleanly (consumes reasonable content and ends near the
buffer's length), show it as a tree; otherwise fall back to a hex dump.

## Known top-level schema

### `Player_Local.sav` / `Player_Remote_*.sav` (identical schema)

```
IslandID                  StrProperty   e.g. "-15846-3123-1845"
bIsOnIsland                BoolProperty
StartLocation               StructProperty<WorldLocation>       (absent on remote sample)
StartIslandID               StrProperty                          (absent on remote sample)
CurrentChunk                 StructProperty<IntVector>
LocationWithWorldOrigin      StructProperty<WorldLocation>
FlyingIslandCockpitData      StructProperty<FlyingIslandCockpitData>  (Local only)
PlayerMarkerSettings          StructProperty<PlayerMarkerSettings>
  bFavoritesFilterEnabled       BoolProperty
  Filters                       ArrayProperty<BoolProperty>  (8 flags)
Location                        StructProperty<WorldLocation>
Components                       ArrayProperty<StructProperty<ComponentSaveData>>
  ComponentSaveData:
    Data                            StructProperty<UObjectSaveData>
      ClassName                        StructProperty<TopLevelAssetPath>
        PackageName                       NameProperty  e.g. "/Script/Skyverse"
        AssetName                         NameProperty  e.g. component class name
      Data                              ArrayProperty<ByteProperty>  -- nested property list (see above)
    ComponentName                    StrProperty
```

`WorldLocation` (a custom struct, not native):
```
Location       StructProperty<Vector>
WorldOrigin    StructProperty<Vector>       (present in the 228-byte variant; the
                                              117-byte StartLocation variant only has Location)
```

### `WorldInfo.sav`

```
WorldName                 StrProperty
InitializeGameVersion       StrProperty   e.g. "0.4.715"
GameVersion                 StrProperty
CreatedTime                  StructProperty<DateTime>
LastPlayedTime                StructProperty<DateTime>
PlayedTime                     StructProperty<Timespan>
Seed                            StrProperty   (random world seed string)
UDSData                          StructProperty<UDSSaveData>   -- "Ultra Dynamic Sky" plugin data
  CurrentTime                       StructProperty<UDSTimeData>
    Month, Day                          IntProperty
    TimeOfDay                            FloatProperty
  CloudPosition                     StructProperty<Vector>
  IncludesWeatherState               BoolProperty
  Weather                             ObjectProperty
  TransitionDuration                  DoubleProperty
  WindDirection                        DoubleProperty
bNewGame                          BoolProperty
BoatsData                          ArrayProperty<StructProperty<BoatSaveData>>
  OriginalLocation    StructProperty<IntVector>
  Location             StructProperty<WorldLocation>
  Rotation              StructProperty<Rotator>
  Data                   ObjectProperty
ItemPickupStruct                    ArrayProperty<StructProperty<ItemPickupSaveData>>
  Location    StructProperty<WorldLocation>
  ItemData     StructProperty<ItemSlotSaveData>
    BaseData      ObjectProperty
    Level          IntProperty
    Durability      IntProperty
    bHasDurability   BoolProperty
    Energy            IntProperty
    FuelType           ObjectProperty
    Fuel                 FloatProperty
    SpecialEffect         ObjectProperty
    SpecialEffectLevel     IntProperty
    CustomModificators       ArrayProperty<StructProperty<EQModificatorSaveData>>
      Type    EnumProperty<EModificatorType>  e.g. "EModificatorType::MT_Add"
      (further fields not observed populated beyond Type)
    AdditionalAlchemyEffectsList  ArrayProperty<ObjectProperty>
    AdditionalAlchemyEffectsTiers ArrayProperty<ByteProperty>
    UniqueID                        StrProperty
  Amount        IntProperty
ExploredIslands                      ArrayProperty<StructProperty<ExploredIslandSaveData>>
  (fields include ExploredIslandData sub-struct — not fully enumerated)
ContainerMarkers                      ArrayProperty<StructProperty<FlagMarkerData>>
  (not fully enumerated)
```

Observed enum types: `EModificatorType` (values seen: `MT_Add`),
`ECharacterStat` (values not individually inspected).

## Implementation notes for the Go port

1. **Trust `Size` for skipping, always.** Every property, including ones
   of a type the parser doesn't specifically understand, can be safely
   skipped by reading exactly `Size` bytes once the tag header (name,
   type, extra, ArrayIndex, Size, guid marker) has been consumed. This
   makes the parser forward-compatible with property types not seen in
   these 3 sample files — worst case, an unrecognized type's *value* is
   shown as a hex blob instead of a decoded scalar.
2. **The one place `Size` doesn't save you** is the type-specific *extra
   header* (before `Size` is even known) — e.g. if a future save contains
   a `TextProperty` or `MapProperty` and the extra-header assumptions
   above are wrong, the parser will desync immediately and every
   subsequent read will look like garbage (huge lengths, etc). The CLI
   should detect this (a bogus huge FString length, or a read past EOF)
   and abort that subtree with a clear "unable to parse from offset X,
   type Y" error rather than crashing or silently producing garbage.
3. Struct-typed array elements (`ArrayProperty` with `InnerType ==
   "StructProperty"`) don't repeat the inner struct's tag header per
   element — only the property list content, terminated by `None`, per
   element, `Count` times.
4. `ArrayProperty<ByteProperty>` used for opaque data blobs should be
   tried recursively (see "Nested blobs").
