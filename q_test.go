package main

import (
	"testing"
)

func TestConvertUnicode(t *testing.T) {
	b := []byte{'A', '\\', 'u', '0', '0', '2', '2', 'H',
		'\\', 'u', '2', '3', '1', '8', 'Z'}
	s := string(b)
	res := convertUnicodeEscapes(s)
	if res != `A"H⌘Z` {
		t.Fatalf("unexpected result: %s", res)
	}
	b = []byte{'\\', 'u', '0', '0', '2', '2',
		'\\', 'u', '2', '3', '1', '8'}
	s = string(b)
	res = convertUnicodeEscapes(s)
	if res != `"⌘` {
		t.Fatalf("unexpected result: %s", res)
	}
}
