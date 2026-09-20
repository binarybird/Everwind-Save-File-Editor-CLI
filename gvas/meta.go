package gvas

import (
	"hash/crc32"
	"strconv"
)

// MetaChecksum computes the exact content of a Skyverse save file's
// ".meta" sidecar (e.g. "Player_Local.sav.meta" alongside
// "Player_Local.sav"): the IEEE CRC32 checksum of data, formatted as a
// plain decimal string with no padding, sign, or trailing newline.
//
// The game recomputes this checksum when loading a save and rejects the
// file (falling back to its own ".backup") if it doesn't match the
// sidecar's contents -- so a save file written by a tool other than the
// game itself (e.g. Marshal's output after an edit) needs its ".meta"
// sidecar rewritten to match, or the game will treat the edit as
// corrupt. Confirmed by computing this checksum for several real save
// files and matching it byte-for-byte against their real ".meta" files
// on disk.
func MetaChecksum(data []byte) string {
	return strconv.FormatUint(uint64(crc32.ChecksumIEEE(data)), 10)
}
