# site_word_freq
Crawls a web site and returns the most commonly occurring words longer than a specified length.

External dependencies: The *"golang.org/x/net/html"* HTML parser package is required to build.

This program finds the most frequently occurring words of a
specified minimum length for a given site.  It is essentially a
web crawler that makes its best effort to stay within the hostname
of the original site.  On a given page, it both scans for text, for
which it builds a frequency histogram, plus it extracts the "href"
links for further processing.  At the end, the accumulated word count
results for all sites is sorted, with the most frequent ones displayed.

Usage: `crawl <web site> [-pprof_port <port num>] [more config options]`
 
The well-known commercial websites are generally too large to viably crawl
completely in reasonable time for a demo.  However, there are handlers
for SIGINT and SIGTERM that drain the existing work-in-progress, and the
results up to that point are displayed.  In fact, running with a large site
is the best way to profile the execution.  The program optionally starts a
`pprof` HTTP server using the configured port for this purpose.

If you find a smaller site, the traversal will only take a few seconds, and
proper completion of the algorithm can be verified (i.e. no deadlocked
goroutines or writes on closed channels, etc.).

The sequence of sites being visited is displayed on a single line, and
changes color to red if the crawl is interrupted, as the queue is drained.

Architecturally it uses the following elements:
- A configurable fixed number of HTML processing goroutines with
the performance enhancement of creating a new goroutine to actually
submit the child URLs from that page if the send channel would block.
It is useful to be able to scale a backend service without rebuilding it.
- Rich error reporting per goroutine.  This is accomplished by
sending a struct which contains an error field in addition to the
input parameters into the task channel.  Using this technique, we
can clearly sort out which errors are tied to which URLs.

A geometrically expanding algorithm such as a web crawler is a challenge
for a program with a fixed number of worker threads.  The technique
of offloading channel writes that would block to a goroutine that
waits for access is used here.  While buffered channels are also used,
they would still eventually fill up, and given that goroutines are very
lightweight, the design choice of dynamic scaling this offers seems to
work well.

In examining the heap profile, we note that the code that parses the
lines scanned from the various HTTP pages is a hotspot that consequently
has been optimized some.  In terms of CPU, unsurprisingly, the
synchronization mechanisms use significant time, as does the
regular expression parser that extracts words (space separators are
not sufficient to parse words).

Another challenge in implementing a recursive-style algorithm
such as a crawler using a fixed thread pool is determining when the
processing is complete.  To accomplish this, the program uses two
channels, one for the goroutines to read URLs to process, and another
for the results to be sent back to the main processing loop.  We use
a looping and counting technique that determines when we're done processing.
This technique is demonstrated in Donovan and Kernighan's "The Go Programming
Language" book.
