package gvas

// Vector3, IntVector3, Rotator3, ColorBytes and LinearColorFloats mirror
// the fixed native layouts documented in docs/FORMAT.md ("Native
// (non-nested) struct types"). Color/LinearColor sizes there are marked
// "inferred" (no populated sample existed during reverse engineering) but
// are included for forward compatibility; if a real save disagrees on
// byte length, decodeNativeStruct falls back to Raw rather than misdecode.
type Vector3 struct{ X, Y, Z float64 }
type IntVector3 struct{ X, Y, Z int32 }
type Rotator3 struct{ Pitch, Yaw, Roll float64 }
type ColorBytes struct{ B, G, R, A byte }
type LinearColorFloats struct{ R, G, B, A float32 }

// NativeValue holds the decoded value of a StructProperty whose StructName
// is a recognized engine-native type — i.e. one serialized as packed bytes
// rather than a nested property list. Exactly one of the typed pointer
// fields is set on success; Raw is the fallback for an unrecognized name
// or an unexpected byte length, and is always what gets re-encoded when
// none of the typed fields apply.
type NativeValue struct {
	StructName    string
	Vector        *Vector3
	IntVector     *IntVector3
	Rotator       *Rotator3
	DateTimeTicks *int64
	TimespanTicks *int64
	Color         *ColorBytes
	LinearColor   *LinearColorFloats
	Raw           []byte
}

var nativeStructNames = map[string]bool{
	"Vector": true, "Vector2D": true, "Vector4": true, "Quat": true,
	"Rotator": true, "Guid": true, "DateTime": true, "Timespan": true,
	"Color": true, "LinearColor": true, "IntPoint": true, "IntVector": true,
	"Box": true, "Box2D": true, "Transform": true, "Plane": true, "Matrix": true,
}

func isNativeStructName(name string) bool { return nativeStructNames[name] }

// decodeNativeStruct never fails: an unrecognized name or a byte length
// that doesn't match the expected layout always yields a Raw fallback, per
// docs/FORMAT.md's point that Size alone is always enough to skip safely.
func decodeNativeStruct(name string, raw []byte) *NativeValue {
	nv := &NativeValue{StructName: name}
	switch name {
	case "Vector":
		if len(raw) == 24 {
			r := NewReader(raw)
			x, _ := r.ReadF64()
			y, _ := r.ReadF64()
			z, _ := r.ReadF64()
			nv.Vector = &Vector3{x, y, z}
			return nv
		}
	case "IntVector":
		if len(raw) == 12 {
			r := NewReader(raw)
			x, _ := r.ReadI32()
			y, _ := r.ReadI32()
			z, _ := r.ReadI32()
			nv.IntVector = &IntVector3{x, y, z}
			return nv
		}
	case "Rotator":
		if len(raw) == 24 {
			r := NewReader(raw)
			p, _ := r.ReadF64()
			yaw, _ := r.ReadF64()
			roll, _ := r.ReadF64()
			nv.Rotator = &Rotator3{p, yaw, roll}
			return nv
		}
	case "DateTime":
		if len(raw) == 8 {
			r := NewReader(raw)
			v, _ := r.ReadI64()
			nv.DateTimeTicks = &v
			return nv
		}
	case "Timespan":
		if len(raw) == 8 {
			r := NewReader(raw)
			v, _ := r.ReadI64()
			nv.TimespanTicks = &v
			return nv
		}
	case "Color":
		if len(raw) == 4 {
			nv.Color = &ColorBytes{B: raw[0], G: raw[1], R: raw[2], A: raw[3]}
			return nv
		}
	case "LinearColor":
		if len(raw) == 16 {
			r := NewReader(raw)
			cr, _ := r.ReadF32()
			cg, _ := r.ReadF32()
			cb, _ := r.ReadF32()
			ca, _ := r.ReadF32()
			nv.LinearColor = &LinearColorFloats{cr, cg, cb, ca}
			return nv
		}
	}
	nv.Raw = raw
	return nv
}

func encodeNativeValue(n *NativeValue) []byte {
	w := NewWriter()
	switch {
	case n.Vector != nil:
		w.WriteF64(n.Vector.X)
		w.WriteF64(n.Vector.Y)
		w.WriteF64(n.Vector.Z)
	case n.IntVector != nil:
		w.WriteI32(n.IntVector.X)
		w.WriteI32(n.IntVector.Y)
		w.WriteI32(n.IntVector.Z)
	case n.Rotator != nil:
		w.WriteF64(n.Rotator.Pitch)
		w.WriteF64(n.Rotator.Yaw)
		w.WriteF64(n.Rotator.Roll)
	case n.DateTimeTicks != nil:
		w.WriteI64(*n.DateTimeTicks)
	case n.TimespanTicks != nil:
		w.WriteI64(*n.TimespanTicks)
	case n.Color != nil:
		w.WriteBytes([]byte{n.Color.B, n.Color.G, n.Color.R, n.Color.A})
	case n.LinearColor != nil:
		w.WriteF32(n.LinearColor.R)
		w.WriteF32(n.LinearColor.G)
		w.WriteF32(n.LinearColor.B)
		w.WriteF32(n.LinearColor.A)
	default:
		return n.Raw
	}
	return w.Bytes()
}
