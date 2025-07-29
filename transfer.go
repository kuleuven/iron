package iron

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/kuleuven/iron/api"
	"github.com/kuleuven/iron/msg"
	"github.com/kuleuven/iron/transfer"
	"go.uber.org/multierr"
	"golang.org/x/sync/errgroup"
)

type Options struct {
	Exclusive   bool     // Do not overwrite existing files
	MinThreads  int      // Minimum number of parallel streams.
	MaxThreads  int      // Maximum number of parallel streams, defaults to maximum number of connections
	SyncModTime bool     // Sync modification time
	Progress    Progress // Optional progress tracking callbacks
}

type Progress interface {
	// AddTotalFiles is called to increase the total number of files. Must be multithread-safe.
	AddTotalFiles(n int)

	// AddTransferredFiles is called to increase the number of transferred files. Must be multithread-safe.
	AddTransferredFiles(n int)

	// AddTotalBytes is called to increase the total number of bytes. Must be multithread-safe.
	AddTotalBytes(bytes int64)

	// AddTransferredBytes is called to increase number of transferred bytes. Must be multithread-safe.
	AddTransferredBytes(bytes int64)
}

// Upload uploads a local file to the iRODS server using parallel transfers.
// The number of threads used will be the number of available connections, up
// to the maximum number of threads specified. If this number is lower than the
// specified minimum number of threads, the minimum number of threads will be used,
// and the copy process will partially block until all connections are available.
func (c *Client) Upload(ctx context.Context, local, remote string, opts Options) error {
	mode := api.O_CREAT | api.O_WRONLY | api.O_TRUNC

	if opts.Exclusive {
		mode |= api.O_EXCL
	}

	w, err := c.OpenDataObject(ctx, remote, mode)
	if code, ok := api.ErrorCode(err); ok && code == msg.HIERARCHY_ERROR {
		if err = c.RenameDataObject(ctx, remote, remote+".bad"); err == nil {
			w, err = c.OpenDataObject(ctx, remote, mode|api.O_EXCL)
		}
	}

	if err != nil {
		return err
	}

	if err := c.upload(ctx, w, local, opts); err != nil {
		err = multierr.Append(err, w.Close())
		err = multierr.Append(err, c.DeleteDataObject(ctx, remote, true))

		return err
	}

	return w.Close()
}

func (c *Client) upload(_ context.Context, w api.File, local string, opts Options) error {
	r, err := os.Open(local)
	if err != nil {
		return err
	}

	defer r.Close()

	stat, err := r.Stat()
	if err != nil {
		return err
	}

	finish := func(err error) error {
		if err != nil || !opts.SyncModTime {
			return err
		}

		return w.Touch(stat.ModTime())
	}

	maxThreads := c.option.MaxConns

	if opts.MaxThreads > 0 {
		maxThreads = min(opts.MaxThreads, c.option.MaxConns)
	}

	if maxThreads <= 1 {
		return finish(transfer.Copy(w, r, stat.Size(), opts.Progress))
	}

	// Acquire all available connections
	pool, err := c.ConnectAvailable(maxThreads - 1)
	if err != nil {
		return err
	}

	defer closeConnections(pool) //nolint:errcheck

	ww := &transfer.ReopenRangeWriter{
		WriteSeekCloser: w,
		Reopen: func() (transfer.WriteSeekCloser, error) {
			var conn api.Conn

			if len(pool) > 0 {
				conn = pool[0]
				pool = pool[1:]
			}

			return w.Reopen(conn, api.O_WRONLY)
		},
	}

	defer ww.Close()

	threads := max(opts.MinThreads, len(pool)+1)

	if c.option.AllowConcurrentUse {
		threads = len(pool) + 1
	}

	return finish(transfer.CopyN(ww, &transfer.ReaderAtRangeReader{ReaderAt: r}, stat.Size(), threads, opts.Progress))
}

// Download downloads a remote file from the iRODS server using parallel transfers.
// The number of threads must be lower or equal to the number of connections the client can make.
// Do not use in combination with AllowConcurrentUse.
func (c *Client) Download(ctx context.Context, local, remote string, opts Options) error {
	mode := os.O_CREATE | os.O_WRONLY | os.O_TRUNC

	if opts.Exclusive {
		mode |= os.O_EXCL
	}

	w, err := os.OpenFile(local, mode, 0o600)
	if err != nil {
		return err
	}

	defer w.Close()

	if err := c.download(ctx, w, remote, opts); err != nil {
		return multierr.Append(err, os.Remove(local))
	}

	if opts.SyncModTime {
		obj, err := c.GetDataObject(ctx, remote)
		if err != nil {
			return err
		}

		return os.Chtimes(local, time.Time{}, obj.ModTime())
	}

	return nil
}

func (c *Client) download(ctx context.Context, file *os.File, remote string, opts Options) error {
	r, err := c.OpenDataObject(ctx, remote, api.O_RDONLY)
	if err != nil {
		return err
	}

	defer r.Close()

	size, err := findSize(r)
	if err != nil {
		return err
	}

	maxThreads := c.option.MaxConns

	if opts.MaxThreads > 0 {
		maxThreads = min(opts.MaxThreads, c.option.MaxConns)
	}

	if maxThreads <= 1 {
		return transfer.Copy(file, r, size, opts.Progress)
	}

	// Acquire all available connections
	pool, err := c.ConnectAvailable(maxThreads - 1)
	if err != nil {
		return err
	}

	defer closeConnections(pool) //nolint:errcheck

	rr := &transfer.ReopenRangeReader{
		ReadSeekCloser: r,
		Reopen: func() (io.ReadSeekCloser, error) {
			var conn api.Conn

			if len(pool) > 0 {
				conn = pool[0]
				pool = pool[1:]
			}

			return r.Reopen(conn, api.O_RDONLY)
		},
	}

	defer rr.Close()

	threads := max(opts.MinThreads, len(pool)+1)

	if c.option.AllowConcurrentUse {
		threads = len(pool) + 1
	}

	return transfer.CopyN(&transfer.WriterAtRangeWriter{WriterAt: file}, rr, size, threads, opts.Progress)
}

func findSize(r io.Seeker) (int64, error) {
	size, err := r.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, err
	}

	_, err = r.Seek(0, io.SeekStart)
	if err != nil {
		return 0, err
	}

	return size, nil
}

var ErrChecksumMismatch = errors.New("checksum mismatch")

// Verify checks the checksum of a local file against the checksum of a remote file
func (c *Client) Verify(ctx context.Context, local, remote string) error {
	g, ctx := errgroup.WithContext(ctx)

	var localHash, remoteHash []byte

	g.Go(func() error {
		var err error

		localHash, err = Sha256Checksum(ctx, local)

		return err
	})

	g.Go(func() error {
		var err error

		remoteHash, err = c.Checksum(ctx, remote, false)

		return err
	})

	if err := g.Wait(); err != nil {
		return err
	}

	if !bytes.Equal(localHash, remoteHash) {
		return fmt.Errorf("%w: local: %s remote: %s", ErrChecksumMismatch, base64.StdEncoding.EncodeToString(localHash), base64.StdEncoding.EncodeToString(remoteHash))
	}

	return nil
}

// Sha256Checksum computes the sha256 checksum of a local file in a goroutine, so that it can be canceled with the context.
// The checksum is computed in a goroutine, so that it can be canceled with the context.
// The function returns the checksum as a byte slice, or an error if either the context is canceled or the checksum computation fails.
func Sha256Checksum(ctx context.Context, local string) ([]byte, error) {
	r, err := os.Open(local)
	if err != nil {
		return nil, err
	}

	defer r.Close()

	// Compute sha256 hash
	h := sha256.New()

	done := make(chan error, 1)

	go func() {
		defer close(done)

		_, err = io.Copy(h, r)

		done <- err
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-done:
		if err != nil {
			return nil, err
		}

		localHash := h.Sum(nil)

		return localHash, nil
	}
}
