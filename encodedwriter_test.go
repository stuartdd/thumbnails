package main

import (
	"testing"
)

func TestEncodedWriter(t *testing.T) {
	ew := NewEncodedWriter(3)
	assertEW(t, "001", ew, "0", "0", 3)
	assertEW(t, "002", ew, "12345678", "012345678", 9)
	assertEW(t, "003", ew, "9", "0123456789", 12)
}

func assertEW(t *testing.T, id string, ew *EncodedWriter, val, expected string, expLen int) {
	b := []byte(val)
	c, err := ew.Write(b)
	if err != nil {
		t.Fatalf("%s Should never return err", id)
	}
	if c != len(b) {
		t.Fatalf("%s Should have written %d. Actual %d", id, len(b), c)
	}
	s := string(ew.Bytes())
	if s != expected {
		t.Fatalf("%s Contains '%s'. Expected '%s'", id, s, expected)
	}
	l := len(ew.bytes)
	if l != expLen {
		t.Fatalf("%s Length is '%d'. Expected '%d'", id, l, expLen)
	}
}
