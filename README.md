# site_word_freq
Crawls a web site and returns the most commonly occurring words longer than a specified length.

External dependencies: The scanner uses the *"golang.org/x/net/html"* HTML parser package,
so you'll need that to build.

This program finds the most frequently occurring words of a
specified minimum length for a given site.  It is essentially a
web crawler that makes its best effort to stay within the hostname
of the original site.  On a given page, it both scans for text, for
which it builds a frequency histogram, plus it extracts the "href"
links for further processing.  At the end, the accumulated word count
results for all sites are sorted, with the most frequent ones displayed.
Statistics about channel usage are also printed.

Usage: `crawl <web site> [-pprof_port <port num>] [more config options]`
 
The well-known commercial websites are generally too large to viably crawl
completely in reasonable time for a demo.  However, I have added handlers
for SIGINT and SIGTERM, so that upon receipt of those signals, the existing
work-in-progress is drained, and the results up to that point are displayed.

As the program optionally starts a pprof HTTP server using the configured
port, it is also useful to monitor the program performance in real time, even
for one of the large sites mentioned above.

If you find the website of an individual proprietor with a small site, the
traversal will only take a few seconds.

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

We observe from the statistics that the longer the program runs,
the more the job submission channel is full.  This is fully expected
of a geometrically expanding algorithm such as a web crawler, and
increasing channel buffer size or goroutines would still eventually
hit a wall for larger sites.  A single Go program on a laptop is far
from the ideal web crawler, but hopefully the program demonstrates
some good Go design concepts.

In examining the heap profile, we note that the code that parses the
lines scanned from the various HTTP pages is a hotspot that has
consequently been optimized some.  In terms of CPU, unsurprisingly,
the synchronization mechanisms use significant time, as does the
regular expression parser that extracts words (space separators are
not sufficient to parse words).

One of the challenges in implementing a recursive-style algorithm
such as a crawler using a fixed thread pool is determining when the
processing is complete.  To accomplish this, the program uses two
channels, one for the goroutines to read URLs to process, and another
for the results to be sent back to the main processing loop.  We use
a looping and counting techique that determines when we're done processing.
This technique is demonstrated in Donovan and Kernighan's "The Go Programming
Language" book.
