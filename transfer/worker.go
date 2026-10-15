package transfer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
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
	// Delete unrecognized files at the target when downloading or uploading a directory
	Delete bool
	// Don't update files in place. If set, a data object will be removed
	// and a new data object will be created, instead of updating the original object
	DisableUpdateInPlace bool
	// SkipTrash indicates whether files should be moved to the trash or not
	SkipTrash bool
	// RemovePerDirectory indicates that, when removing a directory recursively (RemoveDir),
	// each collection should be removed with a single recursive server-side operation
	// (DeleteCollectionAll) instead of removing every data object individually. Data objects
	// are not removed individually in this mode, since they are removed along with their
	// parent collection.
	RemovePerDirectory bool
	// Sync modification time
	SyncModTime bool
	// MaxThreads indicates the maximum threads per transferred file
	MaxThreads int
	// MaxQueued indicates the maximum number of queued files
	// when uploading or downloading a directory
	MaxQueued int
	// OnlyIfNewer indicates whether files should only be transferred
	// if the source file is newer than the destination file,
	// based on the modification time, when syncing directories (UploadDir, DownloadDir, CopyDir, RemoteCopyDir).
	OnlyIfNewer bool
	// CompareChecksums indicates whether checksums should be verified
	// to compare two existing file when syncing directories (UploadDir, DownloadDir, CopyDir, RemoteCopyDir).
	CompareChecksums bool
	// IntegrityChecksums indicates whether checksums should be computed before
	// and after the transfer to verify the integrity of the transfer (Upload, Download, UploadDir, DownloadDir, CopyDir, RemoteCopy, RemoteCopyDir).
	IntegrityChecksums bool
	// CopyMetadata indicates whether AVU metadata should be copied along with
	// data objects and collections when copying a directory (CopyDir, RemoteCopyDir).
	CopyMetadata bool
	// DryRun will only print actions. No actual transfers will be performed. Progress and errors will still be reported.
	DryRun bool
	// FilterPatterns indicates patterns to include when uploading, downloading or copying a directory (UploadDir, DownloadDir, CopyDir, RemoteCopyDir).
	// The pattern syntax is the same as filepath.Match. The matching is done against the base name of the file, directories are always included.
	// If multiple patterns are specified, a file must match at least one of them to be included.
	FilterPatterns []string
	// IgnorePatterns indicates patterns to ignore when uploading, downloading or copying a directory (UploadDir, DownloadDir, CopyDir, RemoteCopyDir).
	// The pattern syntax is the same as filepath.Match. The matching is done against the base name of the file or directory.
	// IgnorePatterns takes precedence over FilterPatterns. If a file or directory matches an ignore pattern, it will be excluded even if it matches an include pattern.
	IgnorePatterns []string
	// Output will, if set, display a progress bar and occurring errors
	// If ErrorHandler or ProgressHandler is set, this option is ignored
	Output io.Writer
	// Progress handler, can be used to track the progress of the transfers
	ProgressHandler func(progress Progress)
	// Error handler, called when an error occurs
	// If this callback is not set or returns an error, the worker will stop and Wait() will return the error
	ErrorHandler func(source, target string, err error) error
}

type Worker struct {
	IndexPool    *api.API
	TransferPool *api.API

	// Remote pools for handling transfers to a different iRODS instance
	// Only used when copying remotely (RemoteCopy, RemoteCopyDir)
	RemoteIndexPool    *api.API
	RemoteTransferPool *api.API

	// Options
	options Options

	// Internal waitgroup
	wg errgroup.Group

	// Hooks for Wait() function
	onwait func()
	closer func() error
}

// maxPerDirectorySkip limits the number of data objects directly inside a single directory
// for which removeAll skips an individual RemoveFile task when Options.RemovePerDirectory is
// set, since the recursive server-side removal of that directory may not scale to an
// unbounded number of objects. The limit applies per directory, not across the whole tree.
const maxPerDirectorySkip = 10000

func New(indexPool, transferPool *api.API, options Options) *Worker {
	var (
		onwait func()
		closer func() error
	)

	if options.Output != nil && options.ProgressHandler == nil && options.ErrorHandler == nil {
		p := ProgressBar(options.Output)

		options.ProgressHandler = p.Handler
		options.ErrorHandler = p.ErrorHandler
		onwait = p.ScanCompleted
		closer = p.Close
	}

	if options.ErrorHandler == nil {
		options.ErrorHandler = func(source, _ string, err error) error {
			return fmt.Errorf("%s: %w", source, err)
		}
	}

	if options.ProgressHandler == nil {
		options.ProgressHandler = func(progress Progress) {
			// Ignore
		}
	}

	if options.MaxThreads <= 0 {
		options.MaxThreads = 1
	}

	return &Worker{
		IndexPool:    indexPool,
		TransferPool: transferPool,
		options:      options,
		onwait:       onwait,
		closer:       closer,
	}
}

type Progress struct {
	Action      Action
	Label       string
	Size        int64
	Transferred int64
	Increment   int64
	StartedAt   time.Time
	FinishedAt  time.Time
}

type progressWriter struct {
	progress Progress
	handler  func(progress Progress)
	sync.Mutex
}

func (pw *progressWriter) Write(buf []byte) (int, error) {
	pw.Lock()
	defer pw.Unlock()

	if n := len(buf); n > 0 {
		pw.progress.Transferred += int64(n)
		pw.progress.Increment = int64(n)

		pw.handler(pw.progress)
	}

	return len(buf), nil
}

func (pw *progressWriter) Close() error {
	pw.Lock()
	defer pw.Unlock()

	pw.progress.FinishedAt = time.Now()
	pw.progress.Increment = 0

	pw.handler(pw.progress)

	return nil
}

// Wait for all transfers to finish
func (worker *Worker) Wait() error {
	if worker.onwait != nil {
		worker.onwait()
	}

	err := worker.wg.Wait()

	if worker.closer != nil {
		err = multierr.Append(err, worker.closer())
	}

	return err
}

// Upload schedules the upload of a local file to the iRODS server using parallel transfers.
// The local file refers to the local file system. The remote file refers to an iRODS path.
// The call blocks until the transfer of all chunks has started.
func (worker *Worker) Upload(ctx context.Context, local, remote string) {
	r, err := os.Open(local)
	if err != nil {
		worker.Error(local, remote, err)

		return
	}

	stat, err := r.Stat()
	if err != nil {
		worker.Error(local, remote, multierr.Append(err, r.Close()))

		return
	}

	worker.FromReader(ctx, &fileReader{
		name: local,
		stat: stat,
		File: r,
	}, remote)
}

type Reader interface {
	Name() string
	Size() int64
	ModTime() time.Time
	io.ReaderAt
	io.Closer
}

type ChecksumReader interface {
	Reader
	Checksum(ctx context.Context) ([]byte, error)
}

type fileReader struct {
	name string
	stat os.FileInfo
	*os.File
	checksum []byte
}

func (r fileReader) Name() string {
	return r.name
}

func (r fileReader) Size() int64 {
	return r.stat.Size()
}

func (r fileReader) ModTime() time.Time {
	return r.stat.ModTime()
}

func (r fileReader) Checksum(ctx context.Context) ([]byte, error) {
	if len(r.checksum) > 0 {
		return r.checksum, nil
	}

	return Sha256Checksum(ctx, r.name)
}

// FromReader schedules the upload of a reader to the iRODS server using parallel transfers.
// The remote file refers to an iRODS path.
// The call blocks until the transfer of all chunks has started.
func (worker *Worker) FromReader(ctx context.Context, r Reader, remote string) {
	if worker.options.DryRun {
		worker.transferProgress(r.Name(), r.Size())

		return
	}

	mode := api.O_CREAT | api.O_WRONLY | api.O_TRUNC

	if worker.options.Exclusive {
		mode |= api.O_EXCL
	}

	w, err := tryOpenDataObject(ctx, worker.TransferPool, remote, mode)
	if err != nil {
		worker.Error(r.Name(), remote, multierr.Append(err, r.Close()))

		return
	}

	pw := worker.transferProgress(r.Name(), r.Size())

	rr := &ReaderAtRangeReader{ReaderAt: r}

	ww := &ReopenRangeWriter{
		WriteSeekCloser: w,
		Reopen: func() (WriteSeekCloser, error) {
			return w.Reopen(nil, api.O_WRONLY)
		},
	}

	wg := worker.copyRanges(ww, rr, pw, r.Size())

	worker.wg.Go(func() error {
		defer pw.Close()

		err := errors.Join(wg.Wait(), ww.Close())

		// Sync the modification time of the remote file if the option is enabled.
		if worker.options.SyncModTime {
			err = errors.Join(err, w.Touch(r.ModTime()))
		}

		// Retrieve the connection from the writer
		conn, closeErr := w.CloseReturnConnection()
		if err != nil || closeErr != nil {
			return errors.Join(err, closeErr, conn.Close(), r.Close())
		}

		// Recreate an API instance using the retrieved connection
		a := ConnectionAsAPI(worker.TransferPool, conn)

		defer a.Close()

		return worker.finishUpload(ctx, a.API, r, remote)
	})
}

func (worker *Worker) finishUpload(ctx context.Context, a *api.API, r Reader, remote string) error {
	var err error

	if cr, ok := r.(ChecksumReader); ok && worker.options.IntegrityChecksums {
		_, _, err = compareChecksums(ctx, cr.Checksum, func(ctx context.Context) ([]byte, error) {
			return a.Checksum(ctx, remote, false)
		})
	}

	err = errors.Join(err, r.Close())
	if err == nil {
		return nil
	}

	err = errors.Join(err, a.DeleteDataObject(ctx, remote, true))

	return worker.options.ErrorHandler(r.Name(), remote, err)
}

func tryOpenDataObject(ctx context.Context, a *api.API, remote string, mode int) (api.File, error) {
	w, err := a.OpenDataObject(ctx, remote, mode)
	if err == nil {
		return w, nil
	}

	code, ok := api.ErrorCode(err)
	if !ok {
		return nil, err
	}

	if code == msg.HIERARCHY_ERROR {
		if err2 := a.RenameDataObject(ctx, remote, remote+".bad"); err2 == nil {
			return a.OpenDataObject(ctx, remote, mode|api.O_EXCL)
		}
	}

	if code == -510017 && mode&api.O_EXCL != 0 {
		return nil, fmt.Errorf("cannot upload exclusively: %s", os.ErrExist)
	}

	return nil, err
}

func verifyChecksumAndClose(ctx context.Context, integrityChecksums bool, sourceChecksum func(ctx context.Context) ([]byte, error), remote api.File) error {
	if !integrityChecksums {
		return remote.Close()
	}

	_, _, err := compareChecksums(ctx, sourceChecksum, func(ctx context.Context) ([]byte, error) {
		return computeChecksumAndClose(ctx, remote)
	})

	return err
}

func computeChecksumAndClose(ctx context.Context, f api.File) ([]byte, error) {
	conn, err := f.CloseReturnConnection()
	if err != nil {
		return nil, multierr.Append(err, conn.Close())
	}

	request := msg.DataObjectRequest{
		Path: f.Name(),
	}

	var irodsChecksum msg.String

	err = conn.Request(ctx, msg.DATA_OBJ_CHKSUM_AN, request, &irodsChecksum)

	err = multierr.Append(err, conn.Close())
	if err != nil {
		return nil, err
	}

	return api.ParseIrodsChecksum(irodsChecksum.String)
}

// FromStream schedules the upload of a io.Reader to the iRODS server using parallel transfers.
// In contrast to FromReader, FromStream will block until the full file has been uploaded.
// The remote file refers to an iRODS path.
func (worker *Worker) FromStream(ctx context.Context, name string, r io.Reader, remote string, appendToFile bool) {
	mode := api.O_CREAT | api.O_WRONLY | api.O_TRUNC

	if appendToFile {
		mode = api.O_CREAT | api.O_WRONLY | api.O_APPEND
	}

	if worker.options.Exclusive {
		mode |= api.O_EXCL
	}

	w, err := tryOpenDataObject(ctx, worker.TransferPool, remote, mode)
	if err != nil {
		worker.Error(name, remote, err)

		return
	}

	// Schedule the upload
	pw := &progressWriter{
		progress: Progress{
			Action:    TransferFile,
			Label:     name,
			Size:      0, // Size is unknown
			StartedAt: time.Now(),
		},
		handler: worker.options.ProgressHandler,
	}

	ww := &CircularWriter{
		WriteSeekCloser: w,
		MaxThreads:      worker.options.MaxThreads,
		Reopen: func() (WriteSeekCloser, error) {
			return w.Reopen(nil, api.O_WRONLY)
		},
	}

	err = multierr.Append(copyBuffer(ww, r, pw), ww.Close())
	if err != nil {
		err = multierr.Append(err, worker.IndexPool.DeleteDataObject(ctx, remote, true))

		worker.Error(name, remote, err)
	}
}

// Download schedules the download of a remote file from the iRODS server using parallel transfers.
// The local file refers to the local file system. The remote file refers to an iRODS path.
// The call blocks until the transfer of all chunks has started.
func (worker *Worker) Download(ctx context.Context, local, remote string) {
	if worker.options.DryRun {
		worker.ToWriter(ctx, &fileWriter{
			name: local,
			File: nil,
		}, remote)

		return
	}

	mode := os.O_CREATE | os.O_WRONLY | os.O_TRUNC

	if worker.options.Exclusive {
		mode |= os.O_EXCL
	}

	w, err := os.OpenFile(local, mode, 0o600)
	if errors.Is(err, os.ErrExist) && worker.options.Exclusive {
		worker.Error(local, remote, fmt.Errorf("cannot download exclusively: %w", os.ErrExist))

		return
	} else if err != nil {
		worker.Error(local, remote, err)

		return
	}

	worker.ToWriter(ctx, &fileWriter{
		name: local,
		File: w,
	}, remote)
}

type Writer interface {
	Name() string
	io.WriterAt
	io.Closer
	Remove() error
	Touch(mtime time.Time) error
}

type ChecksumWriter interface {
	Writer
	Checksum(ctx context.Context) ([]byte, error)
}

type fileWriter struct {
	name string
	*os.File
}

func (w fileWriter) Name() string {
	return w.name
}

func (w fileWriter) Remove() error {
	return os.Remove(w.name)
}

func (w fileWriter) Touch(mtime time.Time) error {
	return os.Chtimes(w.name, time.Time{}, mtime)
}

func (w fileWriter) Checksum(ctx context.Context) ([]byte, error) {
	return Sha256Checksum(ctx, w.name)
}

// ToWriter schedules the download of a remote file from the iRODS server using parallel transfers.
// The remote file refers to an iRODS path.
// The call blocks until the transfer of all chunks has started.
func (worker *Worker) ToWriter(ctx context.Context, w Writer, remote string) {
	r, err := worker.TransferPool.OpenDataObject(ctx, remote, api.O_RDONLY)
	if err != nil {
		err = multierr.Append(err, w.Close())
		err = multierr.Append(err, w.Remove())

		worker.Error(w.Name(), remote, err)

		return
	}

	size, err := findSize(r)
	if err != nil {
		err = multierr.Append(err, w.Close())
		err = multierr.Append(err, w.Remove())

		worker.Error(w.Name(), remote, err)

		return
	}

	if worker.options.DryRun {
		defer r.Close()

		worker.transferProgress(r.Name(), size)

		return
	}

	pw := worker.transferProgress(w.Name(), size)

	ww := &WriterAtRangeWriter{WriterAt: w}

	rr := &ReopenRangeReader{
		ReadSeekCloser: r,
		Reopen: func() (io.ReadSeekCloser, error) {
			return r.Reopen(nil, api.O_RDONLY)
		},
	}

	wg := worker.copyRanges(ww, rr, pw, size)

	worker.wg.Go(func() error {
		defer pw.Close()

		err := errors.Join(wg.Wait(), rr.Close(), w.Close())

		// Retrieve the connection from the writer
		conn, closeErr := r.CloseReturnConnection()
		if err != nil || closeErr != nil {
			return errors.Join(err, closeErr, conn.Close())
		}

		// Recreate an API instance using the retrieved connection
		a := ConnectionAsAPI(worker.TransferPool, conn)

		defer a.Close()

		return worker.finishDownload(ctx, a.API, w, remote)
	})
}

func (worker *Worker) finishDownload(ctx context.Context, a *api.API, w Writer, remote string) error {
	if cw, ok := w.(ChecksumWriter); ok && worker.options.IntegrityChecksums {
		_, _, err := compareChecksums(ctx, cw.Checksum, func(ctx context.Context) ([]byte, error) {
			return a.Checksum(ctx, remote, false)
		})
		if err != nil {
			return worker.options.ErrorHandler(w.Name(), remote, errors.Join(err, w.Remove()))
		}
	}

	if !worker.options.SyncModTime {
		return nil
	}

	obj, err := a.GetDataObject(ctx, remote)
	if err != nil {
		return worker.options.ErrorHandler(w.Name(), remote, err)
	}

	if err := w.Touch(obj.ModTime()); err != nil {
		return worker.options.ErrorHandler(w.Name(), remote, err)
	}

	return nil
}

func (worker *Worker) transferProgress(label string, size int64) *progressWriter {
	pw := &progressWriter{
		progress: Progress{
			Action:    TransferFile,
			Label:     label,
			Size:      size,
			StartedAt: time.Now(),
		},
		handler: worker.options.ProgressHandler,
	}

	if worker.options.DryRun {
		pw.progress.Transferred = size
		pw.progress.FinishedAt = time.Now()
	}

	pw.handler(pw.progress)

	return pw
}

func (worker *Worker) copyRanges(dst RangeWriter, src RangeReader, pw *progressWriter, size int64) *errgroup.Group {
	var wg errgroup.Group

	rangeSize := calculateRangeSize(size, worker.options.MaxThreads)

	for offset := int64(0); offset < size; offset += rangeSize {
		w := dst.Range(offset, rangeSize)
		r := src.Range(offset, rangeSize)

		wg.Go(func() error {
			return copyBuffer(w, r, pw)
		})
	}

	return &wg
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

// ToStream schedules the download into a io.Writer from the iRODS server.
// The remote file refers to an iRODS path.
// The call blocks until the transfer has started.
func (worker *Worker) ToStream(ctx context.Context, name string, w io.Writer, remote string) {
	r, err := worker.TransferPool.OpenDataObject(ctx, remote, api.O_RDONLY)
	if err != nil {
		worker.Error(name, remote, err)

		return
	}

	size, err := findSize(r)
	if err != nil {
		worker.Error(name, remote, err)

		return
	}

	// Schedule the download
	pw := &progressWriter{
		progress: Progress{
			Action:    TransferFile,
			Label:     name,
			Size:      size,
			StartedAt: time.Now(),
		},
		handler: worker.options.ProgressHandler,
	}

	pw.handler(pw.progress)

	rr := &CircularReader{
		ReadSeekCloser: r,
		MaxThreads:     worker.options.MaxThreads,
		Reopen: func() (io.ReadSeekCloser, error) {
			return r.Reopen(nil, api.O_RDONLY)
		},
		Size: size,
	}

	worker.wg.Go(func() error {
		err = multierr.Append(copyBuffer(w, rr, pw), rr.Close())
		if err != nil {
			return worker.options.ErrorHandler(name, remote, err)
		}

		return nil
	})
}

// RemoteCopy schedules a remote copy between two iRODS servers using parallel transfers.
// The remote files refer to an iRODS path.
// The call blocks until the transfer of all chunks has started.
func (worker *Worker) RemoteCopy(ctx context.Context, remote1, remote2 string) {
	// Find out size and other metadata of the source file (remote1) before proceeding with the remote copy.
	r, err := worker.TransferPool.OpenDataObject(ctx, remote1, api.O_RDONLY)
	if err != nil {
		worker.Error(remote1, remote2, err)

		return
	}

	size, err := r.Size()
	if err != nil {
		defer r.Close()

		worker.Error(remote1, remote2, err)

		return
	}

	if worker.options.DryRun {
		defer r.Close()

		worker.transferProgress(remote1, size)

		return
	}

	worker.remoteCopyFrom(ctx, r, remote1, remote2, size)
}

func (worker *Worker) remoteCopyFrom(ctx context.Context, r api.File, remote1, remote2 string, size int64) {
	mode := api.O_CREAT | api.O_WRONLY | api.O_TRUNC

	if worker.options.Exclusive {
		mode |= api.O_EXCL
	}

	w, err := tryOpenDataObject(ctx, worker.RemoteTransferPool, remote2, mode)
	if err != nil {
		worker.Error(remote1, remote2, multierr.Append(err, r.Close()))

		return
	}

	pw := worker.transferProgress(remote1, size)

	rr := &ReopenRangeReader{
		ReadSeekCloser: r,
		Reopen: func() (io.ReadSeekCloser, error) {
			return r.Reopen(nil, api.O_RDONLY)
		},
	}

	ww := &ReopenRangeWriter{
		WriteSeekCloser: w,
		Reopen: func() (WriteSeekCloser, error) {
			return w.Reopen(nil, api.O_WRONLY)
		},
	}

	wg := worker.copyRanges(ww, rr, pw, size)

	worker.wg.Go(func() error {
		defer pw.Close()

		err := errors.Join(wg.Wait(), rr.Close(), ww.Close())

		// Retrieve the connection from the reader
		conn, closeErr := r.CloseReturnConnection()
		if err != nil || closeErr != nil {
			return errors.Join(err, closeErr, conn.Close(), w.Close())
		}

		// Recreate an API instance using the retrieved connection
		a := ConnectionAsAPI(worker.TransferPool, conn)

		defer a.Close()

		return worker.finishRemoteCopy(ctx, a.API, w, remote1, remote2)
	})
}

func (worker *Worker) finishRemoteCopy(ctx context.Context, a *api.API, w api.File, remote1, remote2 string) error {
	var err error

	if worker.options.SyncModTime {
		var obj *api.DataObject

		if obj, err = a.GetDataObject(ctx, remote1); err == nil {
			err = w.Touch(obj.ModTime())
		}
	}

	conn, closeErr := w.CloseReturnConnection()
	if err != nil || closeErr != nil {
		return errors.Join(err, closeErr, conn.Close())
	}

	b := ConnectionAsAPI(worker.RemoteTransferPool, conn)

	defer b.Close()

	if worker.options.IntegrityChecksums {
		_, _, err = compareChecksums(ctx, func(ctx context.Context) ([]byte, error) {
			return a.Checksum(ctx, remote1, false)
		}, func(ctx context.Context) ([]byte, error) {
			return b.Checksum(ctx, remote2, false)
		})
	}

	if worker.options.CopyMetadata {
		err = errors.Join(err, RemoteCopyMetadata(ctx, a, b.API, remote1, api.DataObjectType, remote2, api.DataObjectType))
	}

	if err == nil {
		return nil
	}

	err = errors.Join(err, worker.RemoteIndexPool.DeleteDataObject(ctx, remote2, true))

	return worker.options.ErrorHandler(remote1, remote2, err)
}

// UploadDir uploads a local directory to the iRODS server using parallel transfers.
// The local file refers to the local file system. The remote file refers to an iRODS path.
// The call blocks until the source directory has been completely scanned.
func (worker *Worker) UploadDir(ctx context.Context, local, remote string) {
	if err := worker.IndexPool.CreateCollectionAll(ctx, remote); err != nil {
		worker.Error(local, remote, err)

		return
	}

	queue := make(chan Task, worker.options.MaxQueued)

	// Execute the uploads
	worker.wg.Go(func() error {
		for u := range queue {
			if ctx.Err() != nil {
				continue
			}

			switch u.Action {
			case ComputeChecksum: // This is a special case for the progress handler, it doesn't correspond to an actual task

			case SetModificationTime:
				worker.action(u, func() error { return worker.TransferPool.ModifyModificationTime(ctx, u.Target, u.ModTime) })

			case TransferFile:
				worker.uploadAction(ctx, u)

			case RemoveFile:
				worker.action(u, func() error { return worker.TransferPool.DeleteDataObject(ctx, u.Target, worker.options.SkipTrash) })

			case RemoveDirectory:
				worker.action(u, func() error { return worker.TransferPool.DeleteCollection(ctx, u.Target, worker.options.SkipTrash) })

			case RemoveDirectoryAll:
				worker.action(u, func() error {
					return worker.TransferPool.DeleteCollectionAll(ctx, u.Target, worker.options.SkipTrash)
				})

			case CreateDirectory:
				worker.action(u, func() error { return worker.TransferPool.CreateCollection(ctx, u.Target) })
			}
		}

		return ctx.Err()
	})

	if err := worker.ComputeUploadDir(ctx, local, remote, queue); err != nil {
		worker.wg.Go(func() error {
			return err
		})
	}
}

func (worker *Worker) uploadAction(ctx context.Context, u Task) {
	if worker.options.DryRun {
		worker.log(u)

		return
	}

	r, err := os.Open(u.Source)
	if err != nil {
		worker.Error(u.Source, u.Target, err)

		return
	}

	worker.FromReader(ctx, &taskReader{
		task: u,
		File: r,
	}, u.Target)
}

type taskReader struct {
	task Task
	*os.File
}

func (tr *taskReader) Name() string {
	return tr.task.Source
}

func (tr *taskReader) Size() int64 {
	return tr.task.Size
}

func (tr *taskReader) ModTime() time.Time {
	return tr.task.ModTime
}

func (tr *taskReader) Checksum(ctx context.Context) ([]byte, error) {
	if len(tr.task.Checksum) > 0 {
		return tr.task.Checksum, nil
	}

	return Sha256Checksum(ctx, tr.task.Source)
}

// DownloadDir downloads a remote directory from the iRODS server using parallel transfers.
// The local file refers to the local file system. The remote file refers to an iRODS path.
// The call blocks until the source directory has been completely scanned.
func (worker *Worker) DownloadDir(ctx context.Context, local, remote string) {
	if err := os.MkdirAll(local, 0o755); err != nil {
		worker.Error(local, remote, err)

		return
	}

	queue := make(chan Task, worker.options.MaxQueued)

	// Execute the uploads
	worker.wg.Go(func() error {
		for u := range queue {
			if ctx.Err() != nil {
				continue
			}

			switch u.Action {
			case ComputeChecksum: // This is a special case for the progress handler, it doesn't correspond to an actual task

			case SetModificationTime:
				worker.action(u, func() error { return os.Chtimes(u.Target, time.Time{}, u.ModTime) })

			case TransferFile:
				worker.downloadAction(ctx, u)

			case RemoveFile, RemoveDirectory:
				worker.action(u, func() error { return os.Remove(u.Target) })

			case RemoveDirectoryAll:
				worker.action(u, func() error { return os.RemoveAll(u.Target) })

			case CreateDirectory:
				worker.action(u, func() error { return os.Mkdir(u.Target, 0o755) })
			}
		}

		return ctx.Err()
	})

	if err := worker.ComputeDownloadDir(ctx, remote, local, queue); err != nil {
		worker.wg.Go(func() error {
			return err
		})
	}
}

func (worker *Worker) downloadAction(ctx context.Context, u Task) {
	if worker.options.DryRun {
		worker.log(u)

		return
	}

	worker.Download(ctx, u.Target, u.Source)
}

// RemoveDir removes a directory from the iRODS server entirely,
// but instead of calling the recursive RemoveCollection API,
// it handles the recursion client side. This allows the caller
// to track the deletion progress better.
// The call blocks until the source directory has been completely scanned.
func (worker *Worker) RemoveDir(ctx context.Context, remote string) {
	queue := make(chan Task, worker.options.MaxQueued)

	// Execute the deletions
	worker.wg.Go(func() error {
		for u := range queue {
			if ctx.Err() != nil {
				continue
			}

			switch u.Action { //nolint:exhaustive
			case RemoveFile:
				worker.action(u, func() error { return worker.TransferPool.DeleteDataObject(ctx, u.Target, worker.options.SkipTrash) })

			case RemoveDirectory:
				worker.action(u, func() error { return worker.TransferPool.DeleteCollection(ctx, u.Target, worker.options.SkipTrash) })

			case RemoveDirectoryAll:
				worker.action(u, func() error {
					return worker.TransferPool.DeleteCollectionAll(ctx, u.Target, worker.options.SkipTrash)
				})
			}
		}

		return ctx.Err()
	})

	ch := make(chan *object)

	// Walk the directory
	worker.wg.Go(func() error {
		defer close(ch)

		return worker.IndexPool.Walk(ctx, remote, func(irodsPath string, record api.Record, err error) error {
			if err != nil {
				return worker.options.ErrorHandler("", irodsPath, err)
			}

			ch <- &object{
				target: irodsPath,
				info:   record,
			}

			return nil
		}, api.LexographicalOrder, api.NoSkip)
	})

	defer close(queue)

	// Process the objects
	obj, ok := <-ch

	for ok {
		obj, ok = removeAll(ch, obj, queue, SynchronizeOptions{
			RemovePerDirectory: worker.options.RemovePerDirectory,
		})
	}
}

// ComputeChecksums computes the checksums of all files in a directory on the iRODS server.
// It handles the recursion client side, but individual files are processed server-side only.
// The call blocks until the source directory has been completely scanned.
func (worker *Worker) ComputeChecksums(ctx context.Context, remote string) {
	queue := make(chan Task, worker.options.MaxQueued)

	// Execute the tasks
	worker.wg.Go(func() error {
		for u := range queue {
			if ctx.Err() != nil {
				continue
			}

			switch u.Action { //nolint:exhaustive
			case ComputeChecksum:
				worker.action(u, func() error {
					_, err := worker.TransferPool.Checksum(ctx, u.Target, true)

					return err
				})

			case TransferFile: // Actually we do VerifyChecksums here, but we want to report the progress as a transfer
				worker.action(u, func() error {
					return worker.TransferPool.VerifyChecksum(ctx, u.Target)
				})
			}
		}

		return ctx.Err()
	})

	// Walk the directory
	defer close(queue)

	err := worker.IndexPool.Walk(ctx, remote, func(irodsPath string, record api.Record, err error) error {
		if err != nil {
			return worker.options.ErrorHandler("", irodsPath, err)
		}

		worker.computeChecksum(irodsPath, record, queue)

		return nil
	}, api.NoSkip)
	if err != nil {
		worker.wg.Go(func() error {
			return err
		})
	}
}

func (worker *Worker) computeChecksum(irodsPath string, record api.Record, queue chan<- Task) {
	if record.IsDir() {
		return
	}

	if _, hasChecksum := parseChecksum(record); hasChecksum && worker.options.CompareChecksums {
		queue <- Task{
			Action: TransferFile, // TransferFile = verify checksum
			Target: irodsPath,
		}

		return
	} else if hasChecksum || !worker.options.IntegrityChecksums {
		return
	}

	queue <- Task{
		Action: ComputeChecksum,
		Target: irodsPath,
	}
}

// CopyDir copies one directory to another on the same iRODS server.
// It handles the recursion client side, but individual files are copied server-side only.
// The call blocks until the source directory has been completely scanned.
func (worker *Worker) CopyDir(ctx context.Context, remote1, remote2 string) {
	if err := worker.IndexPool.CreateCollectionAll(ctx, remote2); err != nil {
		worker.Error(remote1, remote2, err)

		return
	}

	if worker.options.CopyMetadata && !worker.options.DryRun {
		if err := worker.IndexPool.CopyMetadata(ctx, remote1, api.CollectionType, remote2, api.CollectionType); err != nil {
			worker.Error(remote1, remote2, err)

			return
		}
	}

	queue := make(chan Task, worker.options.MaxQueued)

	// Execute the uploads
	worker.wg.Go(func() error { //nolint:dupl
		for u := range queue {
			if ctx.Err() != nil {
				continue
			}

			switch u.Action {
			case ComputeChecksum: // This is a special case for the progress handler, it doesn't correspond to an actual task

			case SetModificationTime:
				worker.action(u, func() error { return worker.TransferPool.ModifyModificationTime(ctx, u.Target, u.ModTime) })

			case TransferFile:
				worker.copyObject(ctx, u)

			case RemoveFile:
				worker.action(u, func() error { return worker.TransferPool.DeleteDataObject(ctx, u.Target, worker.options.SkipTrash) })

			case RemoveDirectory:
				worker.action(u, func() error { return worker.TransferPool.DeleteCollection(ctx, u.Target, worker.options.SkipTrash) })

			case RemoveDirectoryAll:
				worker.action(u, func() error {
					return worker.TransferPool.DeleteCollectionAll(ctx, u.Target, worker.options.SkipTrash)
				})

			case CreateDirectory:
				worker.action(u, func() error { return worker.copyCollection(ctx, u) })
			}
		}

		return ctx.Err()
	})

	if err := worker.ComputeCopyDir(ctx, remote1, remote2, queue); err != nil {
		worker.wg.Go(func() error {
			return err
		})
	}
}

func (worker *Worker) copyObject(ctx context.Context, u Task) {
	if worker.options.DryRun {
		worker.log(u)

		return
	}

	remote1 := u.Source
	remote2 := u.Target

	conn, err := worker.TransferPool.Connect(ctx)
	if err != nil {
		worker.Error(remote1, remote2, err)

		return
	}

	startTime := time.Now()

	worker.Progress(Progress{
		Action:    TransferFile,
		Label:     ProgressLabel(remote1, remote2),
		Size:      u.Size,
		StartedAt: startTime,
	})

	worker.wg.Go(func() error {
		a := ConnectionAsAPI(worker.TransferPool, conn)

		defer a.Close()

		if err := a.CopyDataObject(ctx, remote1, remote2); err != nil {
			return worker.options.ErrorHandler(remote1, remote2, err)
		}

		// Copy the AVU metadata after copying the data object if requested
		if worker.options.CopyMetadata {
			if err := a.CopyMetadata(ctx, remote1, api.DataObjectType, remote2, api.DataObjectType); err != nil {
				return worker.options.ErrorHandler(remote1, remote2, err)
			}
		}

		worker.Progress(Progress{
			Action:      TransferFile,
			Label:       ProgressLabel(remote1, remote2),
			Size:        u.Size,
			Increment:   u.Size,
			Transferred: u.Size,
			StartedAt:   startTime,
			FinishedAt:  time.Now(),
		})

		// Verify the checksum after copying if integrity checksums are enabled
		if worker.options.IntegrityChecksums {
			_, _, err := VerifyRemoteToRemote(a.API, a.API, worker.options.ProgressHandler)(ctx, remote1, remote2, nil, nil)
			if err != nil {
				return worker.options.ErrorHandler(remote1, remote2, err)
			}
		}

		return nil
	})
}

// copyCollection creates the target collection and, if requested, copies the
// AVU metadata of the source collection onto it.
func (worker *Worker) copyCollection(ctx context.Context, u Task) error {
	if err := worker.TransferPool.CreateCollection(ctx, u.Target); err != nil {
		return err
	}

	if !worker.options.CopyMetadata {
		return nil
	}

	return worker.TransferPool.CopyMetadata(ctx, u.Source, api.CollectionType, u.Target, api.CollectionType)
}

// RemoteCopyDir copies one collection to another on a different iRODS server.
// The worker needs to have OtherIndexPool and OtherTransferPool set.
// It handles the recursion client side. Files are streamed through the local client.
// The call blocks until the source directory has been completely scanned.
func (worker *Worker) RemoteCopyDir(ctx context.Context, remote1, remote2 string) {
	if err := worker.RemoteIndexPool.CreateCollectionAll(ctx, remote2); err != nil {
		worker.Error(remote1, remote2, err)

		return
	}

	if worker.options.CopyMetadata && !worker.options.DryRun {
		if err := RemoteCopyMetadata(ctx, worker.IndexPool, worker.RemoteIndexPool, remote1, api.CollectionType, remote2, api.CollectionType); err != nil {
			worker.Error(remote1, remote2, err)

			return
		}
	}

	queue := make(chan Task, worker.options.MaxQueued)

	// Execute the uploads
	worker.wg.Go(func() error { //nolint:dupl
		for u := range queue {
			if ctx.Err() != nil {
				continue
			}

			switch u.Action {
			case ComputeChecksum: // This is a special case for the progress handler, it doesn't correspond to an actual task

			case SetModificationTime:
				worker.action(u, func() error { return worker.RemoteTransferPool.ModifyModificationTime(ctx, u.Target, u.ModTime) })

			case TransferFile:
				worker.remoteCopyObject(ctx, u)

			case RemoveFile:
				worker.action(u, func() error {
					return worker.RemoteTransferPool.DeleteDataObject(ctx, u.Target, worker.options.SkipTrash)
				})

			case RemoveDirectory:
				worker.action(u, func() error {
					return worker.RemoteTransferPool.DeleteCollection(ctx, u.Target, worker.options.SkipTrash)
				})

			case RemoveDirectoryAll:
				worker.action(u, func() error {
					return worker.RemoteTransferPool.DeleteCollectionAll(ctx, u.Target, worker.options.SkipTrash)
				})

			case CreateDirectory:
				worker.action(u, func() error { return worker.remoteCopyCollection(ctx, u) })
			}
		}

		return ctx.Err()
	})

	if err := worker.ComputeRemoteCopyDir(ctx, remote1, remote2, queue); err != nil {
		worker.wg.Go(func() error {
			return err
		})
	}
}

func (worker *Worker) remoteCopyObject(ctx context.Context, u Task) {
	if worker.options.DryRun {
		worker.log(u)

		return
	}

	worker.RemoteCopy(ctx, u.Source, u.Target)
}

// remoteCopyCollection creates the target collection and, if requested, copies the
// AVU metadata of the source collection onto it.
func (worker *Worker) remoteCopyCollection(ctx context.Context, u Task) error {
	if err := worker.RemoteTransferPool.CreateCollection(ctx, u.Target); err != nil {
		return err
	}

	if !worker.options.CopyMetadata {
		return nil
	}

	return RemoteCopyMetadata(ctx, worker.TransferPool, worker.RemoteTransferPool, u.Source, api.CollectionType, u.Target, api.CollectionType)
}

// log logs a task without performing it, for dry-run mode.
func (worker *Worker) log(u Task) {
	fmt.Printf("\rwould %s\n", u.Action.Format(ProgressLabel(u.Source, u.Target)))
}

// action runs a simple action and schedules an error
func (worker *Worker) action(u Task, callback func() error) {
	if worker.options.DryRun {
		worker.log(u)

		return
	}

	startTime := time.Now()

	worker.Progress(Progress{
		Action:    u.Action,
		Label:     ProgressLabel(u.Source, u.Target),
		StartedAt: startTime,
	})

	if err := callback(); err != nil {
		worker.Error(u.Source, u.Target, err)

		return
	}

	worker.Progress(Progress{
		Action:     u.Action,
		Label:      ProgressLabel(u.Source, u.Target),
		StartedAt:  startTime,
		FinishedAt: time.Now(),
	})
}

// Error schedules an error
func (worker *Worker) Error(source, target string, err error) {
	worker.wg.Go(func() error {
		return worker.options.ErrorHandler(source, target, err)
	})
}

// Progress triggers the progress handler with the given progress.
func (worker *Worker) Progress(progress Progress) {
	if worker.options.ProgressHandler != nil {
		worker.options.ProgressHandler(progress)
	}
}
