package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestColor(*testing.T) {
	fmt.Printf("%s%s%s",
		string([]byte{033, '[', '3', '1', ';', '1', 'm'}),
		"hello",
		string([]byte{033, '[', '0', 'm'}))
}

func TestServer(t *testing.T) {
	firstPage := `
	<!DOCTYPE html>

	<html lang="en>"
	<head>
	    <meta charset=utf-8">
			<title>First TestPage</title>
	</head>

	<body>
	The quick brown fox jumped over the lazy dog's parallelogram, er, tarantulas
	<a href="anotherpage">another page</a>
	</body>
	</html>
	`

	anotherPage := `
	<!DOCTYPE html>

	<html lang="en>"
	<head>
	    <meta charset=utf-8">
			<title>First TestPage</title>
	</head>

	<body>
	A parallelogram is a really cool shape!  No kidding, a parallelogram.
	But tarantulas are kind of cool too!
	\u0041\u0042\u0043\u0044\u0045\u0046\u0047\u0048\u0049\u004a
	</body>
	</html>
	`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,
		r *http.Request) {
		ru := r.URL.String()
		if ru == "/" {
			w.Write([]byte(firstPage))
		} else if ru == "/anotherpage" {
			w.Write([]byte(anotherPage))
		}
	}))
	defer ts.Close()
	defer ts.CloseClientConnections()

	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatalf("URL parse failed: %v\n", err)
	}
	ctx := context.Background()
	finder := newWordFinder(u)
	finder.run(ctx)
	errs := finder.getErrors()
	if len(errs) != 0 {
		t.Fatalf("got %d unexpected errors\n", len(errs))
	}

	results := finder.getResults()
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d\n", len(results))
	}
	if results[0].key != "parallelogram" || results[0].value != 3 {
		t.Fatalf("unexpected frequency counts observed\n")
	}
	if results[1].key != "tarantulas" || results[1].value != 2 {
		t.Fatalf("unexpected frequency counts observed\n")
	}
	if results[2].key != "ABCDEFGHIJ" || results[2].value != 1 {
		t.Fatalf("unexpected frequency counts observed\n")
	}
}

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
		t.Fatalf("unexpected result: %\n", res)
	}
}
