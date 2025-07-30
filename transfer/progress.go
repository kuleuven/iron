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
		actual:       map[string]Progress{},
		done:         make(chan struct{}),
		wait:         make(chan struct{}),
		started:      time.Now(),
		outputBuffer: &bytes.Buffer{},
	}

	ticker := time.NewTicker(500 * time.Millisecond)

	go func() {
		defer close(pb.wait)
		defer ticker.Stop()

		for {
			select {
			case <-pb.done:
				pb.printDone()
				return
			case t := <-ticker.C:
				pb.print(t)
			}
		}
	}()

	return pb
}

type PB struct {
	actual        map[string]Progress
	done          chan struct{}
	wait          chan struct{}
	started       time.Time
	bytesDone     int64
	scanCompleted bool
	outputBuffer  *bytes.Buffer
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

	if pb.outputBuffer.Len() == 0 {
		fmt.Fprintf(pb.outputBuffer, "\r%s\033[K\n", progress.Label)
	} else {
		fmt.Fprintf(pb.outputBuffer, "%s\n", progress.Label)
	}
}

func (pb *PB) ScanCompleted() {
	pb.Lock()
	defer pb.Unlock()

	pb.scanCompleted = true
}

func (pb *PB) print(t time.Time) {
	pb.Lock()
	defer pb.Unlock()

	fmt.Printf("%s\r%s\033[K", pb.outputBuffer.String(), pb.bar(t))

	pb.outputBuffer.Reset()
}

func (pb *PB) bar(t time.Time) string {
	elapsed := t.Sub(pb.started)

	bytesTransferred := pb.bytesDone
	bytesTotal := pb.bytesDone

	for _, p := range pb.actual {
		bytesTransferred += p.Transferred
		bytesTotal += p.Size
	}

	percent := float64(bytesTransferred) / float64(bytesTotal) * 100
	speed := float64(bytesTransferred) / elapsed.Seconds()
	eta := time.Duration(float64(bytesTotal-bytesTransferred)/speed) * time.Second

	if speed == 0 {
		eta = 0
	}

	if !pb.scanCompleted {
		return fmt.Sprintf("--.--%% |%s| %s/%s | %s/s",
			wheel(t),
			humanize.Bytes(uint64(bytesTransferred)),
			humanize.Bytes(uint64(bytesTotal)),
			humanize.Bytes(uint64(speed)),
		)
	}

	return fmt.Sprintf("%.2f%% |%s| %s/%s | %s/s [%s]",
		percent,
		bar(percent),
		humanize.Bytes(uint64(bytesTransferred)),
		humanize.Bytes(uint64(bytesTotal)),
		humanize.Bytes(uint64(speed)),
		eta,
	)
}

func wheel(t time.Time) string {
	b := bytes.NewBuffer(make([]byte, 0, 20))

	for i := range 20 {
		if i == t.Second()%20 {
			fmt.Fprint(b, "#")
		} else {
			fmt.Fprint(b, " ")
		}
	}

	return b.String()
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

func (pb *PB) printDone() {
	pb.Lock()
	defer pb.Unlock()

	fmt.Printf("%s\r\033[K", pb.outputBuffer.String())

	pb.outputBuffer.Reset()
}

func (pb *PB) Close() {
	close(pb.done)
	<-pb.wait
}
