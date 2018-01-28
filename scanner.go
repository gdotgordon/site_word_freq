// The scanner is the task launched from the worker goroutines to
// parse a given link and extract and count embedded words, and also
// to find embedded links and send those to the work channel to be
// processed by the same goroutines.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

// The SearchRecord is passed to each gorutine to process a given
// link.  Each link builds a word count of words at least as long
// as requested length.  Note each search record will get accumulated
// by the WordFinder.  As each search record has its own error field,
// this gives us an organized way to catalog all the errors that
// occurred in the processing.
type SearchRecord struct {
	url string
	err error
}

var (
	// Match words with Unicode characters, "w" is just ASCII.
	//words = regexp.MustCompile(`\w+`)
	words = regexp.MustCompile(`[\p{L}\d_]+`)

	// Literal unicode values such as '\u0022' may be encountered on a page.
	// These need to be converted to a Unicode byte sequence because as read
	// they are not recognized by the compiler as a Unicode character, just
	// a plain sequence of characters.
	uliteral = regexp.MustCompile(
		`\\u[0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F]`)
)

func (sr *SearchRecord) processLink(ctx context.Context, wf *WordFinder) {
	// Read the url contents and parse the line to get embedded
	// text and extract links for future processing.
	wf.fmtr.showStatusLine(sr.url, wf.interrupt)

	// Don't redirect outside our site.
	sr.url = html.EscapeString(sr.url)
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if !strings.HasSuffix(req.URL.Hostname(), wf.target) {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
	resp, err := client.Get(sr.url)
	if err != nil {
		log.Printf("error opening '%s': %v\n", sr.url, err)
		sr.err = err
		wf.addLinkData(sr, nil, nil)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		// If the page is forbidden or not found, skip it with no error.
		if resp.StatusCode != 403 && resp.StatusCode != 404 {
			sr.err = fmt.Errorf("HTTP status %d received\n", resp.StatusCode)
		}
		wf.addLinkData(sr, nil, nil)
		return
	}
	ct := resp.Header.Get("Content-type")
	if ct == "" {
		wf.addLinkData(sr, nil, nil)
		return
	}
	m, _, err := mime.ParseMediaType(ct)
	if err != nil {
		log.Printf("error parsing content type '%s': %v\n", ct, err)
		sr.err = err
		wf.addLinkData(sr, nil, nil)
		return
	}
	if ct == "application/binary" {
		wf.addLinkData(sr, nil, nil)
		return
	}

	br := bufio.NewReader(resp.Body)
	if m == "text/html" {
		sr.processHTML(br, wf, sr.url)
	} else {
		// If it's not HTML and not bianry, take a swag at parsing
		// it as line-oriented text.
		wds := make(map[string]int)
		for {
			b, err := br.ReadBytes('\n')
			if err != nil && err != io.EOF {
				sr.err = err
				break
			}
			if b != nil && len(b) > 0 {
				processText(string(b), wds)
			}
			if err == io.EOF {
				break
			}
		}
		wf.addLinkData(sr, wds, nil)
	}
}

func (sr *SearchRecord) processHTML(r io.Reader, wf *WordFinder,
	baseURL string) {

	var base *url.URL

	links := make([]string, 0)
	wds := make(map[string]int)
	z := html.NewTokenizer(r)
	inAnchor := false
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			// Reading EOF is the normal end of processsing for the page.
			// Regardless of the error, we'll send what we have to the
			// channel.
			e := z.Err()
			if e != io.EOF {
				sr.err = z.Err()
				log.Printf("error parsing '%s': %v\n", sr.url, e)
			}
			wf.addLinkData(sr, wds, links)
			return
		case html.TextToken:
			if !inAnchor {
				processText(string(z.Text()), wds)
			}
			inAnchor = false
		case html.StartTagToken:
			// If the next tag is an anchor, extract the 'href'.
			tn, hasAttr := z.TagName()
			if (len(tn) != 1 || tn[0] != 'a') || !hasAttr {
				continue
			}
			inAnchor = true
			more := true
			for {
				if !more {
					break
				}
				k, v, m := z.TagAttr()
				more = m

				// Skip if it's not an 'href'.
				attr := string(k)
				if attr != "href" {
					continue
				}

				// Skip fragment links to the same page
				// (i.e. the entire link is a fragment).
				av := string(v)
				if strings.HasPrefix(av, "#") {
					continue
				}

				// Make sure the url is valid format.
				u, err := url.Parse(av)
				if err != nil {
					log.Printf("Warning: parse error on '%s': %v\n",
						av, err)
					continue
				}

				// Remove any fragment, as it is just a location within
				// a page, and we don't want to scan two pages that are
				// otherwsie identical twice.
				if u.Fragment != "" {
					u.Fragment = ""
					av = u.String()
					u, err = url.Parse(av)
					if err != nil {
						log.Printf("Warning: (re)-parse error: %v\n", err)
						continue
					}
				}

				// Ensure we have a full url.
				if !u.IsAbs() {
					if base == nil {
						base, err = url.Parse(baseURL)
						if err != nil {
							log.Printf("Warning: URL parse error: %v\n", err)
							continue
						}
					}
					u = base.ResolveReference(u)
					av = u.String()
				}

				// To keep things from ballooning out of
				// control, only crawl withiin the current site,
				// or a reasonable stab at such an equivalency.
				if strings.HasSuffix(u.Hostname(), wf.target) {
					links = append(links, av)
				}
			}
		case html.EndTagToken:
			inAnchor = false
		}
	}
}

// Extract words from text.  If they are long enough, record
// them in the map.
func processText(text string, wds map[string]int) {
	text = convertUnicodeEscapes(text)
	res := words.FindAllString(text, -1)
	if len(res) > 0 {
		for _, v := range res {
			if len(v) >= *minLen && strings.IndexByte(v, '_') == -1 {
				wds[v]++
			}
		}
	}
}

// Replace any literal Unicode sequences, such as \u2318 with the
// actual Unicode bytes.  Some web pages have the Unicode character
// sequences as a literal sequence of characters.
func convertUnicodeEscapes(text string) string {

	// See if there are any literal Unicode sequences in the string.
	if !strings.HasPrefix(text, "\\u") {
		return text
	}
	si := uliteral.FindAllStringIndex(text, -1)
	if si == nil {
		return text
	}

	// We will use the JSON decoder to convert the literal byte sequence
	// to an actual Unicode character.  It does the right thing if the
	// bytes are surrounded by double-quotes.
	b := []byte(text)
	var res []byte
	var svloc int
	for _, d1 := range si {
		// Process next Unicode sequence.
		// Surround the sequence in double quotes and decode to bytes.
		// The surrounding quotes will automatically be removed.
		qb := append(append([]byte{'"'}, b[d1[0]:d1[1]]...), '"')
		var js string
		err := json.Unmarshal(qb, &js)
		if err != nil {
			log.Printf("warning: unmarshal error: %v\n", err)
			return text
		}

		// Construct the part of the converted string up to and
		// including the current converted Unicode bytes.  Then
		// remember where to resume next time through the loop.
		res = append(append(res, b[svloc:d1[0]]...), []byte(js)...)
		svloc = d1[1]
	}

	// Add the final piece of the original string, which is after the last
	// Unicode sequence.
	res = append(res, b[svloc:]...)
	return string(res)
}
