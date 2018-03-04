# site_word_freq
Crawls a web site and returns the most commonly occurring words within a specified length range.

Subtitle: Yes, Go channels *can* be used to solve recursive problems.

External dependencies: The *"golang.org/x/net/html"* HTML parser package is required to build.

## Why Did I Do This?
I've been reading a lot of posts stating that Go channels weren't good for solving
recursive problems, so I wanted to find a good use case for channels that could eliminate
the recursion.  One obvious reason recursion can bad is because of the potential quick depletion of
stack space.  To be fair, recursion is a powerful model that often works well for solving
certain mathematical problems, especially when used with functional paradigms, or in cases where
the solution is expressed as a recursive function that is tail-recursive, where some smart compilers
(or alert developers) can recognize this case and unwind the tail recursion into iterative non-recursive
code in a rather mechanical fashion.

But for many use cases, solving a conceptually recursive problem by totally non-recursive means
is a win. How to eliminate the recursion isn't always obvious, and channels are no exception,
especially when doing something with potentially unbounded growth, such as web crawling.

After a bit of searching, I found a technique in Donovan and Kernighan's "The Go Programming
Language" book that presents a "counting" technique, wherein each request to scan
a page read is sent one channel, and is balanced by a response from that page sent
to another channel.  When the count of outstanding requests reaches 0, we are done.  To use
this algorithm in a robust way, especially in the face of user or cloud-container based
cancellation of the process required significantly refining the idea to make it bullet-proof.

Two final notes: to make the problem a little more interesting, I added two new requirements:
the first is for the crawl to build a histogram of the most commonly occurring words it finds
in the text, as it parses the HTML for _hrefs_.  So by default, it finds the most common words
between length 5 and 8 . I was always curious about stuff like that, so why not?

Second requirement is that we use a fixed, but configurable number of goroutines.  This models
a situation I have encountered in my work, where multiple containers from multiple apps are sharing
a single VM, and we need to be able to throttle the behavior of individual apps, so they will
be well-behaved VM denizens.


## Back To Or Regularly Scheduled Program ...
This program finds the most frequently occurring words of a specified minimum length,
and optionally a maximum length, for a given site.  It is essentially a web crawler
that makes its best effort to stay within the hostname of the original site.
On a given page, it both scans for text, for which it builds a frequency histogram,
plus it extracts the "href" links for further processing.  At the end, the accumulated word
count results for all pages visited is sorted, with the most frequent ones displayed.

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
goroutines and two channels (as previously mentioned).

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
This thing seems to run well enough.  The only question is whether the unlimited channel or
pushing the blocked sends to goroutines performs better.  My gut feeling was that despite
the ~2K penalty, the sleeping goroutine would be the winner, maybe using a little more memory,
but taxing the system less than the unlimited channel, which requires an extra physical channel,
and also loops around checking both of its internal channels in a `select`, both of which I thought
would cause the system to be a little more taxed.

Well, it appears I was wrong, the unbuffered channel uses less memory as expected, and is marginally
faster.  I did some profiling, and here are some typical runs:

```
\# GNU time command:
21.07u 0.87s 12.40r 115640kB bin/site_word_freq -unlimited_chan=false -iter=1000 http://www.cnn.com
19.12u 0.83s 11.60r 97568kB bin/site_word_freq -unlimited_chan=true -iter=1000 http://www.cnn.com
```
I also did CPU and memory pprofs.  The CPU results are fairly similar, because both stacks are dominated
by the regexp evaluation that goes on in extracting words from pages (I also heard that regexp was slow in
Go, and I may have to agree with that!).

The memory reports from pprof confirmed exactly what we expected (I ran these many times to
"warm up" the system and URL caches, these are typical results):
<pre><code>
using goroutines:
 flat  flat%   sum%        cum   cum%
22321.89kB 53.95% 53.95% 32858.75kB 79.42%  main.(*SearchRecord).processHTML
 8980.54kB 21.71% 75.66%  8980.54kB 21.71%  main.convertUnicodeEscapes
 <b>4097.50kB  9.90% 85.56%  4097.50kB  9.90%  runtime.malg*</b>
 1825.78kB  4.41% 89.97%  1825.78kB  4.41%  main.(*WordFinder).addLinkData
 1056.33kB  2.55% 92.53%  1056.33kB  2.55%  crypto/tls.(*block).reserve
 1024.09kB  2.48% 95.00%  1024.09kB  2.48%  main.(*WordFinder).run.func2.1
 1024.06kB  2.48% 97.48%  1024.06kB  2.48%  bytes.(*Buffer).String (inline)
  532.26kB  1.29% 98.76%   532.26kB  1.29%  regexp.(*bitState).push (inline)
  512.05kB  1.24%   100%   512.05kB  1.24%  net.cgoLookupIP
  
using unlimited channels:
 flat  flat%   sum%        cum   cum%
23699.06kB 58.73% 58.73% 35370.63kB 87.66%  main.(*SearchRecord).processHTML
10032.48kB 24.86% 83.60% 10032.48kB 24.86%  main.convertUnicodeEscapes
 1825.78kB  4.52% 88.12%  1825.78kB  4.52%  main.(*WordFinder).addLinkData
 1025.69kB  2.54% 90.67%  1025.69kB  2.54%  encoding/pem.Decode
  <b>583.01kB  1.44% 92.11%   583.01kB  1.44%  main.unlimitedStringChannel.func1*</b>
  578.66kB  1.43% 93.54%  1106.83kB  2.74%  golang.org/x/net/html.(*Tokenizer).readByte
  532.26kB  1.32% 94.86%   532.26kB  1.32%  regexp.(*bitState).push (inline)
  528.17kB  1.31% 96.17%   528.17kB  1.31%  compress/flate.(*dictDecoder).init (inline)
  520.04kB  1.29% 97.46%   520.04kB  1.29%  net/http.glob..func5
  512.28kB  1.27% 98.73%   512.28kB  1.27%  math/big.nat.make
</code></pre>

So `runtime.malg` is allocating a lot of memory for the goroutine case, while for the unlimited channel case,
the loop adding requests to the queue is a big drain.  But at the end of the day, neither approach seems
to be a significant deteriment to performance, so both seem fine.

## Conclusion
In my Go experience, many developers shun channels for the more familiar and comfortable and familiar
mutexes, condition variables, and easily implemented counting semaphores.  There are clearly cases
where a simple mutex seems to do the job better, but hopefully this exercise demonstrates that channels
can be an elegant and not overly inefficient solution.
