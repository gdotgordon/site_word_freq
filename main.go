// This program finds the most frequently occurring words of a
// specified minimum length for a given site.  It is essentailly a
// web crawler that makes its best effort to stay within the hostname
// of the original site.  On a given page, it both scans for text, for
// which it builds a frequncy histogram, plus it extracts the "href"
// links for further processing.  At the end, the accumulated word count
// results for all sites are sorted, with the most frequent ones displayed.
//
// Architecturally it uses the following elements:
// - A configurable fixed number of goroutines.  This is important
// to be able to scale a backend service without rebuilding it.
// - Rich error reporting per goroutine.  This is accomplished by
// sending a struct which contains an error field in addition to the
// input parameters into the task channel.  Using this technique, we
// can clearly sort out which errors are tied to which URLs.
//
// One of the challenges in implementing a recursive-style algorithm
// such as a crawler using a fixed thread pool is determining when the
// processing is complete.
//
// The program uses two channels, one for the goroutines to read URLs
// to process, and another for the results to be sent back to the main
// processing loop.  We use a looping and counting techique that is used
// to determine when we're done processing.
//
// The code loops, first waiting for new URLs to process, removing any sites
// already visited, and then sends the unique urls to the task channel
// to be processed.  It adds 1 to the count for each record sent to the
// task queue, and decrements by one before it is about to read the results
// channel.  This counting technique is demonstrated in Donovan and
// Kernighan's "The Go Programming Language" book.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"sync"
)

var (
	concurrency = flag.Int("concurrency", 5,
		"number of active concurrent goroutines")
	minLen = flag.Int("min_len", 10, "the minimum word length to track")
)

func main() {
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr,
			"usage: %s [-concurrency #] [-min_len #] <start url>\n", os.Args[0])
		os.Exit(1)
	}

	startUrl := flag.Arg(0)
	surl, err := url.Parse(startUrl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "The url '%s' is not syntactically valid\n",
			startUrl)
		os.Exit(1)
	}

	// Restrict crawling to within initial site for a reasonable demo.
	// So a site that has our host in it (we don't need the www part
	// to comapre) is a link we'll follow/
	target := surl.Hostname()
	if strings.HasPrefix(target, "www.") {
		target = target[4:]
	}

	finder := &WordFinder{
		visited:  make(map[string]bool),
		words:    make(map[string]int),
		startUrl: surl,
		target:   target,
		filter:   make(chan []string, 5*(*concurrency)),
	}

	tasks := make(chan SearchRecord, 5*(*concurrency))
	var wg sync.WaitGroup
	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for rec := range tasks {
				rec.processLink(finder)
			}
		}()
	}

	// Prime the pump by feeding the start url into the work channel.
	// Loop unitl there is no more work.  By keeping a count, we know
	// when there is no more work left.
	//
	// Every link sent into the "task" channel adds one to the count.
	// At the start of each loop iteration, we block on the "filter"
	// channel, which contains esults from each page scan (all the links
	// found for a page are in a single slice)  The loop decrements once
	// each time through to balance the result of adding a new search task.
	// Note, the filter is so named, as we skip any previously scanned pages.
	tasks <- SearchRecord{url: startUrl}
	for cnt := 1; cnt > 0; cnt-- {
		l := <-finder.filter
		for _, link := range l {
			if finder.visited[link] == false {
				finder.visited[link] = true
				cnt++
				tasks <- SearchRecord{url: link}
			}
		}
	}
	log.Printf("Exited processing loop!\n")

	// Don't leak goroutines (yeah, it's a demo, but still).
	close(tasks)
	wg.Wait()

	finder.printResults()
}
