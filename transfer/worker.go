package transfer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/kuleuven/iron/api"
	"github.com/kuleuven/iron/msg"
	"go.uber.org/multierr"
	"golang.org/x/sync/errgroup"
)

type Options struct {
	// Do not overwrite existing files
	Exclusive bool
	// Sync modification time
	SyncModTime bool
	// MaxThreads indicates the maximum threads per transferred file
	MaxThreads int
	// VerifyChecksums indicates whether checksums should be verified
	// to compare an existing file when syncing (UploadDir, DownloadDir)
	VerifyChecksums bool
	// Error handler
	ErrorHandler func(err error) error
}

type Worker struct {
	IndexPool    *api.API
	TransferPool *api.API

	// Options
	options Options

	// Internal waitgroup
	wg errgroup.Group
}

func New(indexPool, transferPool *api.API, options Options) *Worker {
	if options.ErrorHandler == nil {
		options.ErrorHandler = func(err error) error {
			return err
		}
	}

	if options.MaxThreads <= 0 {
		options.MaxThreads = 1
	}

	return &Worker{
		IndexPool:    indexPool,
		TransferPool: transferPool,
		options:      options,
	}
}

type progress struct {
	Label       string
	Size        int64
	Transferred int64
	StartedAt   time.Time
	FinishedAt  time.Time
	sync.Mutex
}

func (p *progress) Write(buf []byte) (int, error) {
	p.Lock()
	defer p.Unlock()

	p.Transferred += int64(len(buf))

	return len(buf), nil
}

func (p *progress) Close() error {
	p.Lock()
	defer p.Unlock()

	p.FinishedAt = time.Now()

	return nil
}

// Wait for all transfers to finish
func (worker *Worker) Wait() error {
	return worker.wg.Wait()
}

// Upload schedules the upload of a local file to the iRODS server using parallel transfers.
// The local file refers to the local file system. The remote file refers to an iRODS path.
func (worker *Worker) Upload(ctx context.Context, local, remote string) { //nolint:funlen
	mode := api.O_CREAT | api.O_WRONLY | api.O_TRUNC

	if worker.options.Exclusive {
		mode |= api.O_EXCL
	}

	r, err := os.Open(local)
	if err != nil {
		worker.Error(err)

		return
	}

	stat, err := r.Stat()
	if err != nil {
		worker.Error(multierr.Append(err, r.Close()))

		return
	}

	w, err := worker.TransferPool.OpenDataObject(ctx, remote, mode)
	if code, ok := api.ErrorCode(err); ok && code == msg.HIERARCHY_ERROR {
		if err = worker.IndexPool.RenameDataObject(ctx, remote, remote+".bad"); err == nil {
			w, err = worker.TransferPool.OpenDataObject(ctx, remote, mode|api.O_EXCL)
		}
	}

	if err != nil {
		worker.Error(multierr.Append(err, r.Close()))

		return
	}

	// Schedule the upload
	pw := &progress{
		Label:     local,
		Size:      stat.Size(),
		StartedAt: time.Now(),
	}

	rr := &ReaderAtRangeReader{ReaderAt: r}

	ww := &ReopenRangeWriter{
		WriteSeekCloser: w,
		Reopen: func() (WriteSeekCloser, error) {
			return w.Reopen(nil, api.O_WRONLY)
		},
	}

	var wg errgroup.Group

	rangeSize := calculateRangeSize(stat.Size(), worker.options.MaxThreads)

	for offset := int64(0); offset < stat.Size(); offset += rangeSize {
		wg.Go(func() error {
			return copyBuffer(ww.Range(offset, rangeSize), rr.Range(offset, rangeSize), pw)
		})
	}

	worker.wg.Go(func() error {
		defer r.Close()
		defer pw.Close()

		err := wg.Wait()
		err = multierr.Append(err, ww.Close())

		if err == nil && worker.options.SyncModTime {
			err = w.Touch(stat.ModTime())
		}

		err = multierr.Append(err, w.Close())
		if err != nil {
			fmt.Print(err)
			err = multierr.Append(err, worker.IndexPool.DeleteDataObject(ctx, remote, true))

			return worker.options.ErrorHandler(err)
		}

		return nil
	})
}

// Download schedules the download of a remote file from the iRODS server using parallel transfers.
// The local file refers to the local file system. The remote file refers to an iRODS path.
func (worker *Worker) Download(ctx context.Context, local, remote string) { //nolint:funlen
	mode := os.O_CREATE | os.O_WRONLY | os.O_TRUNC

	if worker.options.Exclusive {
		mode |= os.O_EXCL
	}

	r, err := worker.TransferPool.OpenDataObject(ctx, remote, api.O_RDONLY)
	if err != nil {
		worker.Error(err)

		return
	}

	size, err := findSize(r)
	if err != nil {
		worker.Error(multierr.Append(err, r.Close()))

		return
	}

	w, err := os.OpenFile(local, mode, 0o600)
	if err != nil {
		worker.Error(multierr.Append(err, r.Close()))

		return
	}

	// Schedule the download
	pw := &progress{
		Label:     local,
		Size:      size,
		StartedAt: time.Now(),
	}

	ww := &WriterAtRangeWriter{WriterAt: w}

	rr := &ReopenRangeReader{
		ReadSeekCloser: r,
		Reopen: func() (io.ReadSeekCloser, error) {
			return r.Reopen(nil, api.O_RDONLY)
		},
	}

	var wg errgroup.Group

	rangeSize := calculateRangeSize(size, worker.options.MaxThreads)

	for offset := int64(0); offset < size; offset += rangeSize {
		wg.Go(func() error {
			return copyBuffer(ww.Range(offset, rangeSize), rr.Range(offset, rangeSize), pw)
		})
	}

	worker.wg.Go(func() error {
		defer w.Close()
		defer pw.Close()

		err := wg.Wait()
		err = multierr.Append(err, rr.Close())

		err = multierr.Append(err, r.Close())
		if err != nil {
			err = multierr.Append(err, os.Remove(local))

			return worker.options.ErrorHandler(err)
		}

		if !worker.options.SyncModTime {
			return nil
		}

		obj, err := worker.IndexPool.GetDataObject(ctx, remote)
		if err != nil {
			return worker.options.ErrorHandler(err)
		}

		err = os.Chtimes(local, time.Time{}, obj.ModTime())
		if err != nil {
			return worker.options.ErrorHandler(err)
		}

		return nil
	})
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

type pathRecord struct {
	path   string
	record api.Record
}

// UploadDir uploads a local directory to the iRODS server using parallel transfers.
// The local file refers to the local file system. The remote file refers to an iRODS path.
func (worker *Worker) UploadDir(ctx context.Context, local, remote string) {
	if err := worker.IndexPool.CreateCollectionAll(ctx, remote); err != nil {
		worker.Error(err)

		return
	}

	ch := make(chan *pathRecord)

	// Scan the remote directory
	worker.wg.Go(func() error {
		defer close(ch)

		return worker.IndexPool.Walk(ctx, remote, func(irodsPath string, record api.Record, err error) error {
			if err != nil {
				return err
			}

			ch <- &pathRecord{
				path:   irodsPath,
				record: record,
			}

			return nil
		}, api.LexographicalOrder, api.NoSkip)
	})

	// Walk through the local directory
	worker.wg.Go(func() error {
		defer func() {
			for range ch {
				// skip
			}
		}()

		return worker.uploadWalk(ctx, local, remote, ch)
	})
}

func (worker *Worker) uploadWalk(ctx context.Context, local, remote string, ch <-chan *pathRecord) error {
	var (
		remoteRecord *pathRecord // Keeps a record of the last remote path. We'll iterate the remote paths simultaneously
		ok           bool
	)

	return filepath.Walk(local, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return worker.options.ErrorHandler(err)
		}

		relpath, err := filepath.Rel(local, path)
		if err != nil {
			return worker.options.ErrorHandler(err)
		}

		irodsPath := toIrodsPath(remote, relpath)

		// Iterate until we find the remote path
		for remoteRecord == nil || remoteRecord.path < irodsPath {
			remoteRecord, ok = <-ch
			if !ok {
				break
			}
		}

		if remoteRecord != nil && remoteRecord.path == irodsPath {
			return worker.upload(ctx, path, info, irodsPath, remoteRecord.record)
		}

		return worker.upload(ctx, path, info, irodsPath, nil)
	})
}

func toIrodsPath(base, path string) string {
	if path == "" || path == "." {
		return base
	}

	return base + "/" + strings.Join(strings.Split(path, string(os.PathSeparator)), "/")
}

func (worker *Worker) upload(ctx context.Context, path string, info os.FileInfo, irodsPath string, remoteInfo api.Record) error {
	if info.IsDir() {
		if remoteInfo != nil {
			return nil
		}

		if err := worker.IndexPool.CreateCollection(ctx, irodsPath); err != nil {
			return worker.options.ErrorHandler(err)
		}

		return nil
	}

	switch {
	case remoteInfo == nil:
		// file does not exist
	case worker.options.Exclusive:
		return nil // file already exists, don't overwrite
	case remoteInfo.Size() != info.Size():
		// size does not match
	case worker.options.VerifyChecksums:
		err := Verify(ctx, worker.IndexPool, path, irodsPath)
		if err == nil {
			return nil
		} else if !errors.Is(err, ErrChecksumMismatch) {
			return worker.options.ErrorHandler(err)
		}
	case remoteInfo.ModTime().Truncate(time.Second).Equal(info.ModTime().Truncate(time.Second)):
		return nil
	default:
	}

	worker.Upload(ctx, path, irodsPath)

	return nil
}

// DownloadDir downloads a remote directory from the iRODS server using parallel transfers.
// The local file refers to the local file system. The remote file refers to an iRODS path.
func (worker *Worker) DownloadDir(ctx context.Context, local, remote string) {
	worker.wg.Go(func() error {
		return worker.IndexPool.Walk(ctx, remote, func(irodsPath string, record api.Record, err error) error {
			if err != nil {
				return worker.options.ErrorHandler(err)
			}

			path := toLocalPath(local, strings.TrimPrefix(irodsPath, remote))

			fi, err := os.Stat(path)
			if !os.IsNotExist(err) && err != nil {
				return worker.options.ErrorHandler(err)
			}

			return worker.download(ctx, irodsPath, record, path, fi)
		})
	})
}

func toLocalPath(base, path string) string {
	if path == "" {
		return base
	}

	return base + strings.Join(strings.Split(path, "/"), string(os.PathSeparator))
}

func (worker *Worker) download(ctx context.Context, irodsPath string, remoteInfo api.Record, path string, info os.FileInfo) error {
	if remoteInfo.IsDir() {
		if info != nil {
			return nil
		}

		if err := os.MkdirAll(path, 0o755); err != nil {
			return worker.options.ErrorHandler(err)
		}

		return nil
	}

	switch {
	case info == nil:
	// file does not exist
	case worker.options.Exclusive:
		return nil // file already exists, don't overwrite
	case info.Size() != remoteInfo.Size():
		// size does not match
	case worker.options.VerifyChecksums:
		if err := Verify(ctx, worker.IndexPool, path, irodsPath); err == nil {
			return nil
		} else if !errors.Is(err, ErrChecksumMismatch) {
			return worker.options.ErrorHandler(err)
		}
	case remoteInfo.ModTime().Truncate(time.Second).Equal(info.ModTime().Truncate(time.Second)):
		return nil
	}

	worker.Download(ctx, path, irodsPath)

	return nil
}

// Error schedules an error
func (worker *Worker) Error(err error) {
	worker.wg.Go(func() error {
		return worker.options.ErrorHandler(err)
	})
}
