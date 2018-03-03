package main

// An unlimited length buffered chan(string).  Caller is
// provided send and receive channels which shoud be
// used as any other channel.  Go has no generics, so
// the only way to generalize this is to use interface{},
// which makes things inconvenient for the caller.  Might
// consider rewriting it that way if I end up liking this :-)
type unlimitedStringChannel struct {
	snd  chan (string)
	rcv  chan (string)
	data []string
}

func NewUnlimitedStringChannel(capacity int) *unlimitedStringChannel {
	usc := &unlimitedStringChannel{
		snd:  make(chan (string)),
		rcv:  make(chan (string)),
		data: make([]string, 0, capacity),
	}

	go func(ur chan<- string, us <-chan string) {
		var r chan<- string
		var nxt string
		s := us
		for {

			// No data to send, then can't write to channel.
			if len(usc.data) == 0 {
				r = nil
				nxt = ""
			} else {
				r = ur
				nxt = usc.data[0]
			}

			select {
			case r <- nxt:
				usc.data = usc.data[1:]
			case d, ok := <-s:
				if !ok {
					// Channel was closed by user.
					s = nil
				} else {
					usc.data = append(usc.data, d)
				}
			}
			if s == nil && len(usc.data) == 0 {
				close(ur)
				break
			}
		}
	}(usc.rcv, usc.snd)
	return usc
}

func (usc *unlimitedStringChannel) receiver() <-chan string {
	return usc.rcv
}

func (usc *unlimitedStringChannel) sender() chan<- string {
	return usc.snd
}
