package transfer

import (
	"bytes"
	"fmt"
	"sync"
	"time"

	"github.com/dustin/go-humanize"
)

func ProgressBar() *PB {
	pb := &PB{
		actual:  map[string]Progress{},
		done:    make(chan struct{}),
		wait:    make(chan struct{}),
		started: time.Now(),
	}

	ticker := time.NewTicker(500 * time.Millisecond)

	go func() {
		defer close(pb.wait)
		defer ticker.Stop()

		for {
			select {
			case <-pb.done:
				pb.PrintDone()
				return
			case t := <-ticker.C:
				pb.Print(t)
			}
		}
	}()

	return pb
}

type PB struct {
	actual    map[string]Progress
	done      chan struct{}
	wait      chan struct{}
	started   time.Time
	bytesDone int64
	sync.Mutex
}

func (pb *PB) Handler(progress Progress) {
	pb.Lock()
	defer pb.Unlock()

	if progress.FinishedAt.IsZero() {
		pb.actual[progress.Label] = progress

		return
	}

	delete(pb.actual, progress.Label)

	pb.bytesDone += progress.Transferred

	fmt.Printf("\r%s\033[K\n%s", progress.Label, pb.bar(time.Now()))
}

func (pb *PB) Print(t time.Time) {
	pb.Lock()
	defer pb.Unlock()

	fmt.Printf("\r%s\033[K", pb.bar(t))
}

func (pb *PB) bar(t time.Time) string {
	elapsed := t.Sub(pb.started)

	var bytesTransferred, bytesTotal int64

	for _, p := range pb.actual {
		bytesTransferred += p.Transferred
		bytesTotal += p.Size
	}

	percent := float64(bytesTransferred) / float64(bytesTotal) * 100
	speed := float64(bytesTransferred+pb.bytesDone) / elapsed.Seconds()
	eta := time.Duration(float64(bytesTotal-bytesTransferred)/speed) * time.Second

	if speed == 0 {
		eta = 0
	}

	return fmt.Sprintf("%s%.2f%% |%s| %s | %s/s [%s]",
		"",
		percent,
		bar(percent),
		humanize.Bytes(uint64(bytesTransferred+pb.bytesDone)),
		// humanize.Bytes(uint64(bytesTotal)),
		humanize.Bytes(uint64(speed)),
		eta,
	)
}

func bar(percent float64) string {
	b := bytes.NewBuffer(make([]byte, 0, 20))

	for i := range 20 {
		if i < int(percent/5) {
			fmt.Fprint(b, "#")
		} else {
			fmt.Fprint(b, " ")
		}
	}

	return b.String()
}

func (pb *PB) PrintDone() {
	fmt.Printf("\r\033[K")
}

func (pb *PB) Close() {
	close(pb.done)
	<-pb.wait
}
