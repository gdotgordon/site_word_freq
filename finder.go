package main

import (
	"fmt"
	"net/url"
	"sort"
	"sync"
)

// The WordFinder is the struct that controls the overall processing.
// It collates the results to get the longest word at the end.
type WordFinder struct {
	records  []*SearchRecord
	visited  map[string]bool
	words    map[string]int
	target   string
	startUrl *url.URL
	filter   chan ([]string)
	mu       sync.Mutex
}

// The following two structs are for sorting the frequency map.
type kvPair struct {
	key   string
	value int
}

type kvSorter []kvPair

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

func (wf *WordFinder) printResults() {
	for _, r := range wf.records {
		if r.err != nil {
			fmt.Printf("'%s': error occurred: %v ", r.url, r.err)
		}
	}

	sorter := make(kvSorter, len(wf.words))
	i := 0
	for k, v := range wf.words {
		sorter[i] = kvPair{k, v}
		i++
	}
	sort.Sort(sorter)

	fmt.Printf("top ten word totals:\n")
	for i := 0; i < 10; i++ {
		fmt.Printf("%s: %d\n", sorter[i].key, sorter[i].value)
	}
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
