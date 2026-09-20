package gvas

import "fmt"

// AppendStructElement appends elem as a new last element of p's
// StructProperty-inner array (an ArrayProperty or SetProperty whose
// Extra.InnerType.Value == "StructProperty"). Returns an error if p is
// not such an array. The encoded element count is derived from
// len(p.Array.Structs) at Marshal time, so no other bookkeeping is
// needed.
func (p *Property) AppendStructElement(elem []*Property) error {
	if p.Array == nil || p.Array.InnerType.Value != "StructProperty" {
		return fmt.Errorf("gvas: %s is not a struct-array property", p.Name)
	}
	p.Array.Structs = append(p.Array.Structs, elem)
	return nil
}

// RemoveStructElement removes the element at index from p's
// StructProperty-inner array. Returns an error if p is not such an
// array, or if index is out of range.
func (p *Property) RemoveStructElement(index int) error {
	if p.Array == nil || p.Array.InnerType.Value != "StructProperty" {
		return fmt.Errorf("gvas: %s is not a struct-array property", p.Name)
	}
	if index < 0 || index >= len(p.Array.Structs) {
		return fmt.Errorf("gvas: %s: index %d out of range (%d elements)", p.Name, index, len(p.Array.Structs))
	}
	p.Array.Structs = append(p.Array.Structs[:index], p.Array.Structs[index+1:]...)
	return nil
}
