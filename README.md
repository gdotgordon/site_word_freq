# site_word_freq
Crawls a web site and returns the most commonly occurring words within a specified length range.

External dependencies: The ["golang.org/x/net/html"] HTML parser package is required to build.

This program finds the most frequently occurring words of a
specified minimum length, and optionally a maximum length,
for a given site.  It is essentially a web crawler that makes
its best effort to stay within the hostname of the original site.
On a given page, it both scans for text, for which it builds a
frequency histogram, plus it extracts the "href" links for further
processing.  At the end, the accumulated word count results for all
pages visited is sorted, with the most frequent ones displayed.

Usage: `crawl <web site> [-pprof_port <port num>] [more config options]`
 
The well-known commercial websites are generally too large to viably crawl
completely in reasonable time on a single-machine demo.  However, handlers
for SIGINT and SIGTERM are installed that drain the existing work-in-progress,
and display the results up to that point.  In fact, running with a large site
is the best way to profile the execution of the code.  The program optionally
starts a `pprof` HTTP server using the configured port for this purpose.

If you find a smaller site, the traversal will only take a few seconds, and
proper completion of the algorithm can be verified (i.e. no deadlocked
goroutines or writes on closed channels, etc.).

The sequence of sites being visited is displayed on a single line, and
if interrupted, the remaining length is displayed as the queue is drained.

## Archtecture
Architecturally the solution uses the following elements:
- A configurable (via a flag) fixed number of HTML processing
goroutines with the performance enhancement of creating a new
goroutine to submit the child URLs from that page if the send
channel would block.  It is useful to be able to tune a backend
service without rebuilding it.  Also, we want to limit the number
of simultaneously open HTTP connections, which this solution accomplishes.
- Rich error reporting per goroutine.  This is accomplished by
sending a struct which contains an error field in addition to the
input parameters into the task channel.  Using this technique, we
can clearly sort out which errors are tied to which URLs.

## Implmentation Notes
The use of Go channels is an elegant and efficient solution to a
"conceptually" recursive problem.  By avoiding recursion, we save
on all the stack space that would have been used in pure recursion,
at the cost of the synchronization and queueing mechanisms of the
two channels, plus the overhead of goroutines that are dynamically
created waiting for channel space to free up (if needed).  Instead of
deep execution stacks, we essentially end up with lists of URLs that
need to be traversed, and the overhead of managing those lists.

One challenge in implementing a recursive "producer/consumer" style
algorithm using channels is knowing when we are done, and can safely
close all the channels.  To accomplish this, the program uses two
channels, one to read new URLs to be processed (producer), and another for
sending the read links out for processing (consumer).  We use a looping
and counting technique such that every new URL read is guaranteed to get
one response back.  This technique is demonstrated in Donovan and Kernighan's
"The Go Programming Language" book.
