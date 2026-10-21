package transfer

import (
	"bytes"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/dustin/go-humanize"
)

func ProgressLabel(source, target string) string {
	if target == "" {
		return source
	}

	return target
}

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
	actionsDone      int
	actionsTotal     int
	scanCompleted    bool
	outputBuffer     *bytes.Buffer
	errors           int
	w                io.Writer
	sync.Mutex
}

func (pb *PB) Handler(progress Progress) {
	pb.Lock()
	defer pb.Unlock()

	if progress.Size == 0 {
		pb.handleUnknownSize(progress)

		return
	}

	pb.handleKnownSize(progress)
}

func (pb *PB) handleUnknownSize(progress Progress) {
	switch {
	case progress.FinishedAt.IsZero():
		// A transfer with unknown size has been started or is busy, e.g. FromStream.
		if _, ok := pb.actual[progress.Label]; !ok {
			pb.actionsTotal++
		}

		pb.actual[progress.Label] = progress

		pb.bytesTransferred += progress.Increment
		pb.bytesTotal += progress.Increment

	default:
		// A transfer with unknown size is complete.
		if _, ok := pb.actual[progress.Label]; !ok {
			pb.actionsTotal++
		}

		delete(pb.actual, progress.Label)

		pb.bytesTransferred += progress.Increment
		pb.bytesTotal += progress.Increment
		pb.actionsDone++

		if progress.Label == "" || progress.Failed {
			// Assume error, don't print
			return
		}

		fmt.Fprintf(pb.outputBuffer, "%s\n", progress.Action.Format(progress.Label))
	}
}

func (pb *PB) handleKnownSize(progress Progress) {
	switch {
	case progress.StartedAt.IsZero():
		// Registration of a transfer
		if registration, ok := pb.actual[progress.Label]; ok {
			pb.bytesTotal += progress.Size - registration.Size
		} else {
			pb.bytesTotal += progress.Size
			pb.actionsTotal++
		}

		pb.actual[progress.Label] = progress

	case progress.FinishedAt.IsZero():
		// A transfer with known size is ongoing.
		if registration, ok := pb.actual[progress.Label]; ok {
			pb.bytesTotal += progress.Size - registration.Size
		} else {
			pb.bytesTotal += progress.Size
			pb.actionsTotal++
		}

		pb.actual[progress.Label] = progress

		pb.bytesTransferred += progress.Increment

	default:
		// A transfer is complete.
		// Check if the transfer hasn't been registered previously,
		// or is terminated early.
		if registration, ok := pb.actual[progress.Label]; ok {
			pb.bytesTotal -= registration.Size - progress.Transferred
		} else {
			pb.bytesTotal += progress.Transferred
			pb.actionsTotal++
		}

		delete(pb.actual, progress.Label)

		pb.bytesTransferred += progress.Increment
		pb.actionsDone++

		if progress.Label == "" || progress.Failed {
			// Assume error, don't print
			return
		}

		fmt.Fprintf(pb.outputBuffer, "%s\n", progress.Action.Format(progress.Label))
	}
}

func (pb *PB) ErrorHandler(source, target string, err error) error {
	pb.Lock()
	defer pb.Unlock()

	pb.errors++

	fmt.Fprintf(pb.outputBuffer, "\x1B[31m%s FAILED: %s\x1B[0m\n", ProgressLabel(source, target), err.Error())

	return nil
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

func (pb *PB) Elapsed() time.Duration {
	pb.Lock()
	defer pb.Unlock()

	return time.Since(pb.started)
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
	percent, speed, eta := pb.computeProgress(elapsed)

	if !pb.scanCompleted {
		return fmt.Sprintf("--.--%% |%s| %s/%s | %s/s",
			wheel(elapsed),
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

func (pb *PB) computeProgress(elapsed time.Duration) (float64, float64, time.Duration) {
	percent := float64(pb.bytesTransferred) / float64(pb.bytesTotal) * 100
	speed := float64(pb.bytesTransferred) / elapsed.Seconds()
	eta := time.Duration(float64(pb.bytesTotal-pb.bytesTransferred)/speed) * time.Second

	if speed == 0 {
		eta = 0
	}

	return percent, speed, eta
}

func wheel(elapsed time.Duration) string {
	seconds := int(elapsed.Seconds() * 2)

	b := bytes.NewBuffer(make([]byte, 0, 20))

	for i := range 18 {
		if i == seconds%18 {
			fmt.Fprint(b, "<#>")
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

func (pb *PB) Close() error {
	close(pb.done)
	<-pb.wait

	if pb.errors > 0 {
		return fmt.Errorf("%d errors", pb.errors)
	}

	return nil
}
