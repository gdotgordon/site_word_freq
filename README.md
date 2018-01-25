# site_word_freq
Crawls a web site and returns the most commonly occurring words longer than a specified length.

External dependencies: The scanner uses the *"golang.org/x/net/html"* HTML parser package,
so you'll need that to build.

This program finds the most frequently occurring words of a
specified minimum length for a given site.  It is essentially a
web crawler that makes its best effort to stay within the hostname
of the original site.  On a given page, it both scans for text, for
which it builds a frequncy histogram, plus it extracts the "href"
links for further processing.  At the end, the accumulated word count
results for all sites are sorted, with the most frequent ones displayed.
Statistics useful for performance tuning are also printed.

Usage: `crawl <web site>`
 
I found the well-known commercial websites to be too large to viably crawl
completely in reasonable time for a demo.  However, I have added handlers
for SIGINT and SIGTERM, so that upon receipt of those signals, the exisiting
work-in-progress is drained, and the results up to that point are displayed.

Or if you find the website of an individual proprietor with a small site, the
traversal will only take a few seconds.

The sequence of sites being visited is disaplyed on a single line, and
changes color to red if the crawl is interrupted, as the queue is drained.

Architecturally it uses the following elements:
- A configurable fixed number of goroutines with the performance
enhancement of creating a new goroutine to submit the child URLs
if the send channel would block.  It is useful to be able to scale
a backend service without rebuilding it.
- Rich error reporting per goroutine.  This is accomplished by
sending a struct which contains an error field in addition to the
input parameters into the task channel.  Using this technique, we
can clearly sort out which errors are tied to which URLs.

We observe from the statistics that the longer the program runs,
the more the job submission channel is full.  This is fully expected
of a geomertrically expanding algorithm such as a web crawler, and
increasing channel buffer size or goroutines would still eventually
hit a wall for larger sites.  A single Go program on a laptop is far
from the ideal web crawler, but hopefully the program demostrates
some good Go design concepts.

One of the challenges in implementing a recursive-style algorithm
such as a crawler using a fixed thread pool is determining when the
processing is complete.  To accomplish this, The program uses two
channels, one for the goroutines to read URLs to process, and another
for the results to be sent back to the main processing loop.  We use
a looping and counting techique that is determines when we're done processing.

The code loops, first waiting for new URLs to process, removing any sites
already visited, and then sends the unique urls to the task channel
to be processed.  It adds 1 to the count for each record sent to the
task queue, and decrements by one before it is about to read the results
channel.  This counting technique is demonstrated in Donovan and Kernighan's
"The Go Programming Language" book.
