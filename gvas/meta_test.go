package gvas

import "testing"

func TestMetaChecksum(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want string
	}{
		// crc32.ChecksumIEEE("") == 0
		{"empty", []byte{}, "0"},
		// crc32.ChecksumIEEE("123456789") == 0xCBF43926 == 3421780262,
		// the standard CRC32/IEEE check value.
		{"standard check value", []byte("123456789"), "3421780262"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := MetaChecksum(c.data)
			if got != c.want {
				t.Errorf("MetaChecksum(%q) = %q, want %q", c.data, got, c.want)
			}
		})
	}
}

// TestMetaChecksumMatchesRealSaveFiles confirms MetaChecksum's output
// against the real .sav testdata files' own recorded checksums are not
// available as testdata (the sidecar .meta files live only in the
// game's own save directory, not this repo's testdata/), so this test
// instead confirms MetaChecksum is deterministic and round-trips
// through re-Marshal for an unedited file, matching the invariant the
// real game's checksum comparison depends on: Marshal(Unmarshal(data))
// producing the same bytes (and therefore the same checksum) as data
// itself when nothing was edited.
func TestMetaChecksumMatchesRealSaveFiles(t *testing.T) {
	data := readTestdata(t, "Player_Local.sav")
	f, err := Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	out, err := Marshal(f)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if MetaChecksum(data) != MetaChecksum(out) {
		t.Errorf("MetaChecksum(original) = %s, MetaChecksum(round-tripped) = %s, want equal", MetaChecksum(data), MetaChecksum(out))
	}
}
