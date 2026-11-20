//go:build windows

package security

import (
	"encoding/binary"
	"testing"
	"unicode/utf16"
)

func TestRelativeWindowsReparseTargetClassification(t *testing.T) {
	fixture := func(target string) []byte {
		units := utf16.Encode([]rune(target))
		data := make([]byte, 20+2*len(units))
		binary.LittleEndian.PutUint32(data[:4], 0xa000000c)
		binary.LittleEndian.PutUint16(data[4:6], uint16(len(data)-8))
		binary.LittleEndian.PutUint16(data[10:12], uint16(2*len(units)))
		binary.LittleEndian.PutUint32(data[16:20], 1)
		for i, u := range units {
			binary.LittleEndian.PutUint16(data[20+2*i:], u)
		}
		return data
	}
	for _, target := range []string{"file.txt", `..\file.txt`, "unicode-🦊.txt"} {
		got, err := relativeWindowsReparseTarget(fixture(target))
		if err != nil || got != target {
			t.Fatal(got, err)
		}
	}
	for _, target := range []string{"", `C:\private\file.txt`, `\private\file.txt`, "/private/file.txt", `\\server\share`, "a\x00b"} {
		if _, err := relativeWindowsReparseTarget(fixture(target)); err == nil {
			t.Fatal("unsafe raw target admitted", target)
		}
	}
	for _, mutate := range []func([]byte){
		func(b []byte) { b[0] = 0 }, func(b []byte) { binary.LittleEndian.PutUint32(b[16:20], 0) },
		func(b []byte) { binary.LittleEndian.PutUint32(b[16:20], 3) }, func(b []byte) { binary.LittleEndian.PutUint16(b[4:6], 65535) },
		func(b []byte) { binary.LittleEndian.PutUint16(b[4:6], 0) }, func(b []byte) { binary.LittleEndian.PutUint16(b[8:10], 1) },
		func(b []byte) { binary.LittleEndian.PutUint16(b[10:12], 1) }, func(b []byte) { binary.LittleEndian.PutUint16(b[10:12], 65000) },
		func(b []byte) { binary.LittleEndian.PutUint16(b[20:22], 0xd800) }, func(b []byte) { binary.LittleEndian.PutUint16(b[20:22], 0xdc00) },
	} {
		data := fixture("safe.txt")
		mutate(data)
		if _, err := relativeWindowsReparseTarget(data); err == nil {
			t.Fatal("malformed raw reparse admitted")
		}
	}
	if _, err := relativeWindowsReparseTarget(nil); err == nil {
		t.Fatal("empty reparse admitted")
	}
}
