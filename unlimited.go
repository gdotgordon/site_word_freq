package main

// Function implementing an unlimited length buffered chan string.
// Caller is provided send and receive channels which shoud be used
// as any other channel. Go has no generics, so the only way to
// generalize this is to use chan (interface{}), which makes things
// inconvenient for the caller.  Might consider rewriting it that way
// if I end up liking this approach :-)
// This excellent blog post was the seed for this
// https://medium.com/capital-one-developers/building-an-unbounded-channel-in-go-789e175cd2cd
func unlimitedStringChannel(capacity int) (chan<- string, <-chan string) {
	snd := make(chan (string))
	rcv := make(chan (string))
	data := make([]string, 0, capacity)

	go func(snd <-chan string, rcv chan<- string) {
		var r chan<- string
		var nxt string
		s := snd
		for {

			// No data to send, then can't write to channel.
			if len(data) == 0 {
				r = nil
				nxt = ""
			} else {
				r = rcv
				nxt = data[0]
			}

			select {
			case r <- nxt:
				data = data[1:]
			case d, ok := <-s:
				if !ok {
					// Channel was closed by user.
					s = nil
				} else {
					data = append(data, d)
				}
			}
			if s == nil && len(data) == 0 {
				close(rcv)
				break
			}
		}
	}(snd, rcv)
	return snd, rcv
}
