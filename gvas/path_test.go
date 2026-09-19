package gvas

import "testing"

func TestLookupTopLevel(t *testing.T) {
	f, err := Unmarshal(readTestdata(t, "WorldInfo.sav"))
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	p, err := Lookup(f, "WorldName")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if p.Str == nil || *p.Str != "jff" {
		t.Errorf("got %+v", p)
	}
}

func TestLookupNestedStruct(t *testing.T) {
	f, err := Unmarshal(readTestdata(t, "WorldInfo.sav"))
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	p, err := Lookup(f, "UDSData.CurrentTime.Month")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if p.Int32 == nil {
		t.Errorf("got %+v", p)
	}
}

func TestLookupArrayIndex(t *testing.T) {
	f, err := Unmarshal(readTestdata(t, "Player_Local.sav"))
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	p, err := Lookup(f, "Components[0].ComponentName")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if p.Str == nil {
		t.Errorf("got %+v", p)
	}
}

func TestLookupThroughNestedFile(t *testing.T) {
	f, err := Unmarshal(readTestdata(t, "Player_Local.sav"))
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	// Data.Data is an ArrayProperty<ByteProperty> that decoded as a
	// NestedFile; Lookup must transparently descend into it.
	p, err := Lookup(f, "Components[0].Data.Data.ComponentSaveDataDoesNotExist")
	if err == nil {
		t.Fatalf("expected an error for a nonexistent property, got %+v", p)
	}
}

func TestLookupErrors(t *testing.T) {
	f, err := Unmarshal(readTestdata(t, "WorldInfo.sav"))
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if _, err := Lookup(f, "DoesNotExist"); err == nil {
		t.Error("expected error for missing top-level property")
	}
	if _, err := Lookup(f, "BoatsData[999]"); err == nil {
		t.Error("expected error for out-of-range array index")
	}
	if _, err := Lookup(f, "WorldName.Nested"); err == nil {
		t.Error("expected error descending into a scalar")
	}
}
