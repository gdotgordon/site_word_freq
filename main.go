// This program finds the most frequently occurring words of a
// specified minimum length for a given site.  It is essentially a
// web crawler that makes its best effort to stay within the hostname
// of the original site.  On a given page, it both scans for text, for
// which it builds a frequncy histogram, plus it extracts the "href"
// links for further processing.
//
// At the end, the most frequent cumulateive word counts are displayed
// in sorted order.  It also reports some statistics related to channel
// usage, so in theory, we could performance tune the program.
//
// The program uses two channels, one for the goroutines to read URLs
// to process, and another for the results to be sent back to the main
// processing loop.  We use a looping and counting techique to determine
// when we're done processing.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"syscall"
)

// Used to determine a channel buffer size.  This is a swag that each
// visited page may generate this number of new links to process.
const concurrencyMultiplier = 5

var (
	concurrency = flag.Int("concurrency", 5,
		"number of active concurrent goroutines")
	minLen   = flag.Int("min_len", 10, "the minimum word length to track")
	totWords = flag.Int("tot_words", 10, "show the top 'this many' words")

	// The output of sites visited depends on whether the output is
	// sent to a terminal.
	isTTY bool
)

func main() {
	flag.Parse()
	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr,
			"usage: %s [-concurrency #] [-min_len #] <start url>\n", os.Args[0])
		os.Exit(1)
	}

	startURL := flag.Arg(0)
	surl, err := url.Parse(startURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "The url '%s' is not syntactically valid\n",
			startURL)
		os.Exit(1)
	}

	// We'll use escape sequences if stdout is not being redirected
	// to a file.  This check may not be perfect, but it is fin
	// for our purposes.
	fi, err := os.Stdout.Stat()
	if err == nil {
		if (fi.Mode() & (os.ModeDevice | os.ModeCharDevice)) ==
			(os.ModeDevice | os.ModeCharDevice) {
			isTTY = true
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	finder := newWordFinder(surl)

	go func() {
		// Shutdown cleanup on termination signal (SIGINT and SIGTERM
		// for now).
		ch := make(chan os.Signal)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		log.Printf("%-75.75s", <-ch)
		cancel()
	}()

	finder.run(ctx)
	showStatus(finder)
}

func showStatus(finder *WordFinder) {
	elist := finder.getErrors()
	if elist == nil {
		fmt.Printf("No errors occurred in run.\n")
	} else {
		for _, r := range elist {
			fmt.Printf("'%s': error occurred: %s\n", r.url, r.err.Error())
		}
	}

	rs := finder.getRunStats()
	fmt.Printf("job channel was blocked %.2f%% of the time (%d/%d)\n\n",
		float64(rs.chanBlocked)/float64(rs.chanBlocked+rs.chanFree)*100,
		rs.chanBlocked, rs.chanBlocked+rs.chanFree)

	res := finder.getResults()
	fmt.Printf("Top %d totals for words of length >= %d:\n",
		*totWords, *minLen)
	for i, kv := range res {
		fmt.Printf("[%d] %s: %d\n", i+1, kv.key, kv.value)
	}
}
