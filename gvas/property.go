package gvas

// ExtraHeader holds the type-specific fields that appear in a property tag
// header between Type and ArrayIndex. See docs/FORMAT.md, "Type-specific
// extra header". Only the fields relevant to a given Property.Type are
// populated; the rest are zero values.
type ExtraHeader struct {
	StructName             FName // StructProperty
	PackagePath            FName // StructProperty
	EnumName               FName // ByteProperty
	EnumTypeName           FName // EnumProperty
	EnumPackagePath        FName // EnumProperty
	UnderlyingType         FName // EnumProperty
	InnerType              FName // ArrayProperty / SetProperty
	InnerStructName        FName // ArrayProperty / SetProperty, when InnerType=="StructProperty"
	InnerStructPackagePath FName // ArrayProperty / SetProperty, when InnerType=="StructProperty"
	KeyType                FName // MapProperty
	ValueType              FName // MapProperty
}

// ArrayValue holds the decoded elements of an ArrayProperty/SetProperty.
// Exactly one of Structs/Bools/Ints/Int64s/Floats/Doubles/Strings/Bytes is
// populated for a recognized InnerType; RawCount+RawElements is the
// fallback for anything else (e.g. ObjectProperty elements), preserving
// the exact bytes for round-tripping. See docs/FORMAT.md, "Array/Set value
// format".
type ArrayValue struct {
	InnerType FName

	Structs [][]*Property // one property list per element, when InnerType.Value == "StructProperty"
	Bools   []bool
	Ints    []int32
	Int64s  []int64
	Floats  []float32
	Doubles []float64
	Strings []string
	Bytes   []byte // one byte per element, when InnerType.Value == "ByteProperty"

	RawCount    int32
	RawElements []byte
}

// Property is one entry in a property list: a tagged name/type/value
// triple. Exactly one of the typed value fields is populated, chosen by
// Type. See docs/FORMAT.md, "Property tag".
type Property struct {
	Name       string
	Type       string
	ArrayIndex int32
	Extra      ExtraHeader

	// GuidMarker and Guid are meaningless for BoolProperty (which has
	// neither — see docs/FORMAT.md). For every other type, GuidMarker is
	// the raw marker byte as read; Guid is set only when GuidMarker == 1.
	GuidMarker uint8
	Guid       []byte

	Bool *bool
	// BoolRaw is the exact inline byte a BoolProperty was read from (0 =
	// false, any nonzero = true; both 0x01 and 0x10 have been observed —
	// see docs/FORMAT.md). Marshal writes BoolRaw back verbatim when it
	// still agrees with *Bool, and only falls back to canonical 0/1 when
	// SetBool has actually changed the semantic value, so an unedited
	// file round-trips byte-for-byte.
	BoolRaw uint8
	// Str holds StrProperty/NameProperty/EnumProperty's value (EnumProperty
	// formatted "Type::Value"), and also ObjectProperty's value when decode
	// confirmed its bytes are a clean FString — an Unreal object-path
	// string "<PackagePath>.<AssetName>", or "" for a null reference. An
	// ObjectProperty whose bytes don't decode cleanly falls back to Raw
	// instead (see decodeValue in decode.go).
	Str     *string
	Int32   *int32
	Int64   *int64
	Float32 *float32
	Float64 *float64
	Byte    *uint8 // plain (non-enum) ByteProperty value when Size == 1

	Struct []*Property  // StructProperty, when StructName is not a native struct
	Native *NativeValue // StructProperty, when StructName is a native struct
	Array  *ArrayValue  // ArrayProperty / SetProperty

	Raw []byte // fallback: an ObjectProperty whose bytes weren't a clean FString, or any type not specifically decoded

	// NestedFile is set when Type=="ArrayProperty", Extra.InnerType.Value
	// =="ByteProperty", and Array.Bytes successfully parsed as another
	// embedded [header][property list][footer] blob. See docs/FORMAT.md,
	// "Nested blobs". When set, this is authoritative over Array.Bytes for
	// re-encoding (see encode.go).
	NestedFile *File
}

// File is the top-level (or nested-blob) wrapper: a single reserved header
// byte, a property list, and a trailing footer. See docs/FORMAT.md,
// "Top-level structure".
type File struct {
	HeaderByte byte
	Root       []*Property
	Footer     []byte
}
