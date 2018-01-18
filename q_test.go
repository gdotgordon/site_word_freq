package main

import (
	"testing"
)

func TestConvertUnicode(t *testing.T) {
	b := []byte{'A', '\\', 'u', '0', '0', '2', '2', 'H',
		'\\', 'u', '2', '3', '1', '8', 'Z'}
	dotestConvert(t, b, `A"H⌘Z`)

	b = []byte{'\\', 'u', '0', '0', '2', '2',
		'\\', 'u', '2', '3', '1', '8'}
	dotestConvert(t, b, `"⌘`)

	b = []byte{'\\', 'u', '0', '0', 'b', 'd'}
	dotestConvert(t, b, `½`)
}

func dotestConvert(t *testing.T, data []byte, expected string) {
	s := string(data)
	res := convertUnicodeEscapes(s)
	if convertUnicodeEscapes(s) != expected {
		t.Fatalf("unexpected result: %s", res)
	}
}
