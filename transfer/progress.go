package transfer

import (
	"bytes"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/dustin/go-humanize"
)

func ProgressBar(w io.Writer) *PB {
	pb := &PB{
		actual:       map[string]Progress{},
		done:         make(chan struct{}),
		wait:         make(chan struct{}),
		started:      time.Now(),
		outputBuffer: &bytes.Buffer{},
		w:            w,
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
	actual           map[string]Progress
	done             chan struct{}
	wait             chan struct{}
	started          time.Time
	bytesTransferred int64
	bytesTotal       int64
	scanCompleted    bool
	outputBuffer     *bytes.Buffer
	w                io.Writer
	sync.Mutex
}

func (pb *PB) Handler(progress Progress) {
	pb.Lock()
	defer pb.Unlock()

	switch {
	case progress.Transferred == 0:
		// Registration
		if prev, ok := pb.actual[progress.Label]; ok {
			pb.bytesTotal += progress.Size - prev.Size
		} else {
			pb.bytesTotal += progress.Size
		}

		pb.actual[progress.Label] = progress
	case progress.FinishedAt.IsZero():
		// Transfer ongoing
		pb.actual[progress.Label] = progress

		pb.bytesTransferred += int64(progress.Increment)
	default:
		// Transfer complete
		delete(pb.actual, progress.Label)

		fmt.Fprintf(pb.outputBuffer, "%s\n", progress.Label)
	}
}

func (pb *PB) Write(buf []byte) (int, error) {
	pb.Lock()
	defer pb.Unlock()

	pb.outputBuffer.Write(buf)

	return len(buf), nil
}

func (pb *PB) ScanCompleted() {
	pb.Lock()
	defer pb.Unlock()

	pb.scanCompleted = true
}

func (pb *PB) print(t time.Time) {
	pb.Lock()
	defer pb.Unlock()

	if pb.outputBuffer.Len() > 0 {
		fmt.Fprintf(pb.w, "\r\033[K%s%s", pb.outputBuffer.String(), pb.bar(t))

		pb.outputBuffer.Reset()
	} else {
		fmt.Fprintf(pb.w, "\r%s\033[K", pb.bar(t))
	}
}

func (pb *PB) bar(t time.Time) string {
	elapsed := t.Sub(pb.started)

	percent := float64(pb.bytesTransferred) / float64(pb.bytesTotal) * 100
	speed := float64(pb.bytesTransferred) / elapsed.Seconds()
	eta := time.Duration(float64(pb.bytesTotal-pb.bytesTransferred)/speed) * time.Second

	if speed == 0 {
		eta = 0
	}

	if !pb.scanCompleted {
		return fmt.Sprintf("--.--%% |%s| %s/%s | %s/s",
			wheel(t),
			humanize.Bytes(uint64(pb.bytesTransferred)),
			humanize.Bytes(uint64(pb.bytesTotal)),
			humanize.Bytes(uint64(speed)),
		)
	}

	return fmt.Sprintf("%.2f%% |%s| %s/%s | %s/s [%s]",
		percent,
		bar(percent),
		humanize.Bytes(uint64(pb.bytesTransferred)),
		humanize.Bytes(uint64(pb.bytesTotal)),
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

	fmt.Fprintf(pb.w, "\r\033[K%s", pb.outputBuffer.String())

	pb.outputBuffer.Reset()
}

func (pb *PB) Close() {
	close(pb.done)
	<-pb.wait
}
