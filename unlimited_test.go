package main

import (
	"fmt"
	"sync"
	"testing"
)

func TestUnlimitedBuffering(t *testing.T) {
	snd, rdr := unlimitedStringChannel(0)
	lcnt := 100
	gcnt := 50

	var wg sync.WaitGroup
	for i := 0; i < gcnt; i++ {
		wg.Add(1)
		i := i
		go func() {
			defer wg.Done()

			for n := 0; n < lcnt; n++ {
				snd <- fmt.Sprintf("%d %d)", i, n)
			}
		}()
	}

	wg.Wait()
	close(snd)
	res := make([][]bool, gcnt)
	for i := 0; i < gcnt; i++ {
		res[i] = make([]bool, lcnt)
	}
	for m := range rdr {
		var i, j int
		n, err := fmt.Sscanf(m, "%d %d", &i, &j)
		if err != nil {
			t.Fatalf("scanf error: %v\n", err)
		}
		if n != 2 {
			t.Fatalf("expcted 2 items, but got %d\n", n)
		}
		res[i][j] = true
	}
	for i, v := range res {
		for j, w := range v {
			if !w {
				t.Fatalf("element[%d][%d] was not set\n", i, j)
			}
		}
	}
}
