// The finder drives the main processing of the crawler, accumulates
// error results and stats, and reports the final word count tallies.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

// The WordFinder controls the overall processing.  It collates the
// results to get the longest word at the end.
type WordFinder struct {
	words     map[string]int
	errRecs   []*SearchRecord
	target    string
	startURL  *url.URL
	filter    chan ([]string)
	interrupt bool
	mu        sync.Mutex
	client    *http.Client
	fmtr      *formatter
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
func newWordFinder(startURL *url.URL, f *formatter) *WordFinder {

	// Restrict crawling to within the initial site.  Thus a
	// site that has our host in it is a link we'll follow
	// (we don't need the www part to compare).
	target := startURL.Hostname()
	if strings.HasPrefix(target, "www.") {
		target = target[4:]
	}

	// The one client is thread safe for use by the scanners.
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if !strings.HasSuffix(req.URL.Hostname(), target) {
				return http.ErrUseLastResponse
			}
			return nil
		},
		Timeout: time.Duration(*connTimeout) * time.Second,
	}

	return &WordFinder{
		words:    make(map[string]int, *dictSize),
		startURL: startURL,
		target:   target,
		filter:   make(chan []string, *chanBufLen),
		client:   client,
		fmtr:     f,
	}
}

// This is the main run loop from the crawler.  It creates the
// worker goroutines, filters and submits new URL processing tasks,
// and waits for the entire process to complete before returning.
func (wf *WordFinder) run(ctx context.Context) {

	log.Printf("Beginning run, type Ctrl-C to interrupt.\n\n")

	// Create and launch the goroutines that crawl and
	// gather word counts.
	visited := make(map[string]bool)
	search := make(chan string, *chanBufLen)
	var wg sync.WaitGroup
	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go func(tasks <-chan string) {
			defer wg.Done()

			for rec := range tasks {
				sr := SearchRecord{url: rec}
				sr.processLink(ctx, wf)
			}
		}(search)
	}

	// The function definition for the main processing loop.
	loopFunc := func(tasks chan<- string, filter <-chan []string) {

		// Prime the pump by feeding start url into the work channel.
		tasks <- wf.startURL.String()

		// Loop until there is no more work.  By keeping a count, we
		// know when there is no more work left.  The loop decrements
		// once each time through to balance the result of adding a new
		// search task.
		for cnt := 1; cnt > 0; cnt-- {
			// At the start of each loop iteration, we block on the
			// "filter" channel, which contains results from each
			// page scan (all the links found for a page are in a
			// single slice).  Note since we are inside the loop,
			// we are guaranteed to get more reads,  and the
			// interrupt-handling preserves this invariant.
			l := <-filter

			// If the user cancelled, swallow the new urls.
			select {
			case <-ctx.Done():
				wf.interrupt = true
				line := fmt.Sprintf("draining queue... (%d) ",
					cnt)
				wf.fmtr.showStatusLine(line, wf.interrupt)
				continue
			default:
				break
			}

			// Process the links seen in the page scan read from
			// the channel.
			for _, link := range l {
				// Don't visit the same link twice.
				if visited[link] {
					continue
				}
				visited[link] = true

				// Every link sent into the "task"
				// channel adds one to the counter.
				//  The loop decremnts the count by one
				// at the end of each iteration.
				cnt++
				select {
				case tasks <- link:
				default:
					link := link
					go func() {
						tasks <- link
					}()
				}
			}
		}

		// Note: due to the counting in the loop above, we know
		// that all sending and receiving of data is done, so
		// it is safe to close the write channel here.
		close(tasks)
	}

	// Block, waiting for the loop to finish, as there is nothing
	// else we need to do here.  We could trivially transform this into
	// a goroutine invocation if needed.
	loopFunc(search, wf.filter)

	// As above, all processing is done, so close the other channel.
	close(wf.filter)
	wg.Wait()
}

// When a goroutine is finished processing a link, it transfers its
// link and word count data to the finder.  We could eliminate the
// mutex here and have the dictionary merge happen in the channel
// read loop, but then the unmerged dictionaries would pile up
// in the channel buffers or waiting goroutines, so this is a
// time/sapce tradeoff, as merging the data here is fast.
func (wf *WordFinder) addLinkData(ctx context.Context,
	sr *SearchRecord, wds map[string]int, links []string) {
	if (wds != nil && len(wds) > 0) || links != nil {
		wf.mu.Lock()

		// Only append records with errors.
		if sr.err != nil {
			wf.errRecs = append(wf.errRecs, sr)
		}
		for k, v := range wds {
			wf.words[k] += v
		}
		wf.mu.Unlock()
	}

	sendData := func(filter chan<- []string) {
		// Only create a new goroutine to send the link if the channel
		// would block.  One way or another, we want to keep the thread
		// available for processing.
		select {
		case <-ctx.Done():
			wf.interrupt = true
			filter <- nil
		case filter <- links:
		default:
			go func() { filter <- links }()
		}
	}
	sendData(wf.filter)
}

// Show any errors and the top word counts.
func (wf *WordFinder) getResults() []kvPair {
	sorter := make(kvSorter, len(wf.words))
	i := 0
	for k, v := range wf.words {
		sorter[i] = kvPair{k, v}
		i++
	}
	sort.Sort(sorter)
	cnt := int(*totWords)
	if len(sorter) < cnt {
		cnt = len(sorter)
	}
	return sorter[:cnt]
}

// Returns the search records that contained errors or
// nil if no errors occurred.
func (wf *WordFinder) getErrors() []*SearchRecord {
	return wf.errRecs
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
