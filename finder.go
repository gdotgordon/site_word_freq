// The finder drives the main processing of the crawler, accumulates
// results, and reports the finall word count tallies.
package main

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"
)

// Used to determine a channel buffer size.  This is a swag that each
// visited page may generate this number of new links to process.
const concurrencyMultiplier = 5

// The WordFinder is the struct that controls the overall processing.
// It collates the results to get the longest word at the end.
type WordFinder struct {
	records  []*SearchRecord
	visited  map[string]bool
	words    map[string]int
	target   string
	startURL *url.URL
	filter   chan ([]string)
	mu       sync.Mutex
}

// The following two structs are for sorting the frequency map.
type kvPair struct {
	key   string
	value int
}

type kvSorter []kvPair

// Ensure we've implemented all the sort.Interface methods.
var _ sort.Interface = (*kvSorter)(nil)

// Creates a new WordFinder with the given start URL.
func newWordFinder(startURL *url.URL) *WordFinder {

	// Restrict crawling to within initial site for a reasonable demo.
	// So a site that has our host in it (we don't need the www part
	// to comapre) is a link we'll follow/
	target := startURL.Hostname()
	if strings.HasPrefix(target, "www.") {
		target = target[4:]
	}

	return &WordFinder{
		visited:  make(map[string]bool),
		words:    make(map[string]int),
		startURL: startURL,
		target:   target,
		filter:   make(chan []string, concurrencyMultiplier*(*concurrency)),
	}
}

// This is the main run loop from the crawler.  It creates the
// worker goroutines, filters and submits new URL processing tasks,
// and waits for the entire process to complete before returning.
func (wf *WordFinder) run() {

	// Create and launch the goroutines.
	tasks := make(chan SearchRecord, concurrencyMultiplier*(*concurrency))
	var wg sync.WaitGroup
	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for rec := range tasks {
				rec.processLink(wf)
			}
		}()
	}

	// Prime the pump by feeding the start url into the work channel.
	tasks <- SearchRecord{url: wf.startURL.String()}

	// Loop until there is no more work.  By keeping a count, we know
	// when there is no more work left.  The loop decrements once each
	// time through to balance the result of adding a new search task.
	for cnt := 1; cnt > 0; cnt-- {

		// At the start of each loop iteration, we block on the "filter"
		// channel, which contains results from each page scan (all the
		// links found for a page are in a single slice).  The filter
		// also removes any links already visited.
		l := <-wf.filter

		for _, link := range l {
			if wf.visited[link] == false {
				wf.visited[link] = true
				// Every link sent into the "task" channel
				// adds one to the count.
				cnt++
				tasks <- SearchRecord{url: link}
			}
		}
	}

	// Don't leak goroutines (yeah, it's a demo, but still).
	close(tasks)
	close(wf.filter)
	wg.Wait()
}

// When a goroutine is finished processing a link, it transfers it's
// link and word count data to the finder.
func (wf *WordFinder) addLinkData(sr *SearchRecord, wds map[string]int,
	links []string) {
	wf.mu.Lock()
	wf.records = append(wf.records, sr)
	for k, v := range wds {
		wf.words[k] += v
	}
	wf.mu.Unlock()

	// Only create a new goroutine to send the link if the channel
	// would block.  One way or another, we want to keep the thread
	// available for processing.
	select {
	case wf.filter <- links:
	default:
		go func() {
			wf.filter <- links
		}()
	}
}

// Show any errors and the top word counts.
func (wf *WordFinder) printResults() {
	sorter := make(kvSorter, len(wf.words))
	i := 0
	for k, v := range wf.words {
		sorter[i] = kvPair{k, v}
		i++
	}
	sort.Sort(sorter)

	fmt.Printf("top word totals:\n")
	for i := 0; i < *totWords && i < len(sorter); i++ {
		fmt.Printf("[%d] %s: %d\n", i+1, sorter[i].key, sorter[i].value)
	}
}

// Returns the search records that contained errors or
// nil if no errors occurred.
func (wf *WordFinder) getErrors() []*SearchRecord {
	var sr []*SearchRecord
	for _, r := range wf.records {
		if r.err != nil {
			sr = append(sr, r)
		}
	}
	return sr
}

// The following methods are used to to sort the histogram by value.
// Len is part of sort.Interface.
func (kvs kvSorter) Len() int {
	return len(kvs)
}

// Swap is part of sort.Interface.
func (kvs kvSorter) Swap(i, j int) {
	kvs[i], kvs[j] = kvs[j], kvs[i]
}

// Less is part of sort.Interface.
func (kvs kvSorter) Less(i, j int) bool {
	return kvs[j].value < kvs[i].value
}
