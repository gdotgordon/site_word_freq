# site_word_freq
Crawls a web site and returns the most commonly occurring words within a specified length range.

External dependencies: The *"golang.org/x/net/html"* HTML parser package is required to build.

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

## Architecture
Architecturally the solution uses the following elements:
- A configurable (via a flag) fixed number of HTML processing
goroutines and two channels.  The fixed number of goroutines is a plausible
use-case, for example, if there is a need to limit use of resources, such as
network connections.

- Rich error reporting per goroutine.  This is accomplished by
sending a struct which contains an error field in addition to the
input parameters into the task channel.  Using this technique, we
can clearly sort out which errors are tied to which URLs.

#### Aside: implications of using a fixed number of goroutines
As it turns out, due to the fixed number of goroutines, there could be the
potential for deadlock if additional steps aren't taken to ensure this can't
happen.  The problem is that the work producers (scans of web pages for links)
send an exponentially growing amount data back in to be processed into the same
loop.  Thus it is theoretically possible for all the senders of new work to be blocked
sending while waiting for new senders to be available!  There are two switchable
solutions provided here.  One is that if all send channels are blocked, the blocking
send request is put off in a goroutine, so we can keep the process moving.  The other
option implemented is to use a "virtual" channel that implements an unlimited
buffer size (using fixed channel buffer sizes is not that useful, because we can't
determine a good buffer size that will never block which gets us back to the potential
for deadlock).

The blocked goroutines and unlimited channel are basically solving the problem the same
way.  If we create a bunch of extra goroutines, these are basically stack frames waiting
to be run, and due to the nature of Go, don't waste an OS thread.  Each stack frame
is something like 2K.  With the unlimited channel, each item only takes the size of a
string held in a slice, but this entails the complexity and performance penalty of having
to manage two channels (see `unlimitedStringChannel` in unlimited.go).  But either way, we
are deferring extra work we can't currently accommodate.  This allows us to return to
potentially freeing up a goroutine that is blocked trying to send in it results of new work,
so the processing cycle is guaranteed to be able to continue.


## Implementation Notes
One reason I even attempted this was to see how well Go channels
would work for solving a sort of "unbounded" recursive problem
(well, staying within a single site is bounded, but the spurts of
requests can grow exponentially).  By avoiding recursion, we save
on all the stack space that would have been used in pure recursion,
at the cost of the synchronization and queueing mechanisms of the
two channels, plus dealing with the deadlock potential, as mentioned above.
So instead of deep execution stacks, we essentially end up with lists of
URLs that need to be traversed, and the overhead of managing those lists.

One challenge in implementing a recursive "producer/consumer" style
algorithm using channels is knowing when we are done, and can safely
close all the channels.  To accomplish this, the program uses two
channels, one to read new URLs to be processed (producer), and another for
sending the read links out for processing (consumer).  We use a looping
and counting technique such that every new URL read is guaranteed to get
one response back.  This technique is demonstrated in Donovan and Kernighan's
"The Go Programming Language" book.  For this to work, we have to ensure
that absolutely every scan request leads to a response of some kind, even
if an error occurred or the user interrupted.  This is where `defer` works
well to ensure these cases are all handled.
