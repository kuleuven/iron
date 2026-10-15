package transfer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kuleuven/iron/api"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

type Action int

const (
	CreateDirectory Action = iota // CreateDirectory creates a Target directory
	TransferFile                  // TransferFile transfers a file from Source to Target
	RemoveFile                    // RemoveFile removes the Target file
	RemoveDirectory               // RemoveDirectory removes the Target directory
	// RemoveDirectoryAll removes a collection and all its remaining contents (data objects
	// and subcollections) with a single recursive server-side operation. It is used instead
	// of RemoveDirectory when Options.RemovePerDirectory is set.
	RemoveDirectoryAll  // RemoveDirectoryAll removes the Target directory and all its contents
	ComputeChecksum     // ComputeChecksum computes the checksum of the Target file
	SetModificationTime // SetModificationTime sets the modification time of the Target file
)

func (a Action) Format(label string) string {
	switch a {
	case ComputeChecksum:
		return fmt.Sprintf("\x1B[36mc %s\x1B[0m", label)

	case SetModificationTime:
		return fmt.Sprintf("\x1B[35mt %s\x1B[0m", label)

	case CreateDirectory:
		return fmt.Sprintf("\x1B[34m+ %s/\x1B[0m", label)

	case TransferFile:
		return fmt.Sprintf("+ %s", label)

	case RemoveFile:
		return fmt.Sprintf("\x1B[31m- %s\x1B[0m", label)

	case RemoveDirectory, RemoveDirectoryAll:
		return fmt.Sprintf("\x1B[33m- %s/\x1B[0m", label)

	default:
		return label
	}
}

type Task struct {
	Action         Action
	Source, Target string
	Size           int64
	Checksum       []byte
	ModTime        time.Time
}

type object struct {
	source string
	target string
	info   os.FileInfo
}

// ComputeUploadDir schedules tasks to synchronize a local directory to a
// collection on the iRODS server. All individual actions are appended to the queue.
// The Direction dictates the order of deletes and transfers. The deleteFirst parameter
// indicates whether to delete files first before retransferring, or whether files might
// be updated in place.
// The call blocks until the source directory has been completely scanned.
func (worker *Worker) ComputeUploadDir(ctx context.Context, local, remote string, queue chan<- Task) error { //nolint:dupl,funlen
	lch := make(chan *object)
	rch := make(chan *object)

	wg, ctx := errgroup.WithContext(ctx)

	// Walk through the remote directory
	wg.Go(func() error {
		defer close(rch)

		return worker.IndexPool.Walk(ctx, remote, func(irodsPath string, record api.Record, err error) error {
			path := toLocalPath(local, strings.TrimPrefix(irodsPath, remote))

			if err != nil {
				return worker.options.ErrorHandler(path, irodsPath, err)
			}

			rch <- &object{path, irodsPath, record}

			return nil
		}, api.LexographicalOrder, api.NoSkip)
	})

	// Walk through the local directory
	wg.Go(func() error {
		defer close(lch)

		return filepath.Walk(local, func(path string, info os.FileInfo, err error) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}

			relpath, relErr := filepath.Rel(local, path)
			if relErr != nil {
				return relErr
			}

			irodsPath := toIrodsPath(remote, relpath)

			if err != nil {
				return worker.options.ErrorHandler(path, irodsPath, err)
			}

			lch <- &object{path, irodsPath, info}

			return nil
		})
	})

	// Process the records
	wg.Go(func() error {
		opts := SynchronizeOptions{
			Exclusive:                        worker.options.Exclusive,
			OnlyIfNewer:                      worker.options.OnlyIfNewer,
			CompareChecksums:                 worker.options.CompareChecksums,
			Delete:                           worker.options.Delete,
			DisableUpdateInPlace:             worker.options.DisableUpdateInPlace,
			RemovePerDirectory:               worker.options.RemovePerDirectory,
			SyncModTimeForIdenticalChecksums: worker.options.SyncModTime,
			FilterPatterns:                   worker.options.FilterPatterns,
			IgnorePatterns:                   worker.options.IgnorePatterns,
			ChecksumVerify:                   VerifyLocalToRemote(worker.IndexPool, worker.options.ProgressHandler),
			ErrorHandler:                     worker.options.ErrorHandler,
		}

		return Synchronize(ctx, checkOrder(lch), checkOrder(rch), registerProgress(queue, worker.options.ProgressHandler), opts)
	})

	return wg.Wait()
}

// ComputeDownloadDir schedules tasks to synchronize a collection on the iRODS
// server to a local directory. All individual actions are appended to the queue.
// The Direction dictates the order of deletes and transfers. The deleteFirst parameter
// indicates whether to delete files first before retransferring, or whether files might
// be updated in place.
// The call blocks until the source directory has been completely scanned.
func (worker *Worker) ComputeDownloadDir(ctx context.Context, remote, local string, queue chan<- Task) error { //nolint:dupl,funlen
	lch := make(chan *object)
	rch := make(chan *object)

	wg, ctx := errgroup.WithContext(ctx)

	// Walk through the remote directory
	wg.Go(func() error {
		defer close(rch)

		return worker.IndexPool.Walk(ctx, remote, func(irodsPath string, record api.Record, err error) error {
			path := toLocalPath(local, strings.TrimPrefix(irodsPath, remote))

			if err != nil {
				return worker.options.ErrorHandler(irodsPath, path, err)
			}

			rch <- &object{irodsPath, path, record}

			return nil
		}, api.LexographicalOrder, api.NoSkip)
	})

	// Walk through the local directory
	wg.Go(func() error {
		defer close(lch)

		return filepath.Walk(local, func(path string, info os.FileInfo, err error) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}

			relpath, relErr := filepath.Rel(local, path)
			if relErr != nil {
				return relErr
			}

			irodsPath := toIrodsPath(remote, relpath)

			if err != nil {
				return worker.options.ErrorHandler(irodsPath, path, err)
			}

			lch <- &object{irodsPath, path, info}

			return nil
		})
	})

	// Process the records
	wg.Go(func() error {
		opts := SynchronizeOptions{
			Exclusive:                        worker.options.Exclusive,
			OnlyIfNewer:                      worker.options.OnlyIfNewer,
			CompareChecksums:                 worker.options.CompareChecksums,
			Delete:                           worker.options.Delete,
			DisableUpdateInPlace:             worker.options.DisableUpdateInPlace,
			RemovePerDirectory:               worker.options.RemovePerDirectory,
			SyncModTimeForIdenticalChecksums: worker.options.SyncModTime,
			FilterPatterns:                   worker.options.FilterPatterns,
			IgnorePatterns:                   worker.options.IgnorePatterns,
			ChecksumVerify:                   VerifyRemoteToLocal(worker.IndexPool, worker.options.ProgressHandler),
			ErrorHandler:                     worker.options.ErrorHandler,
		}

		return Synchronize(ctx, checkOrder(rch), checkOrder(lch), registerProgress(queue, worker.options.ProgressHandler), opts)
	})

	return wg.Wait()
}

// ComputeCopyDir schedules tasks to synchronize a collection to another
// collection on the same iRODS server. All individual actions are appended to the queue.
// The deleteFirst parameter indicates whether to delete files first before retransferring,
// or whether files might be updated in place.
// The call blocks until the source directory has been completely scanned.
func (worker *Worker) ComputeCopyDir(ctx context.Context, remote1, remote2 string, queue chan<- Task) error {
	r1ch := make(chan *object)
	r2ch := make(chan *object)

	wg, ctx := errgroup.WithContext(ctx)

	// Walk through the source directory
	wg.Go(func() error {
		defer close(r1ch)

		return worker.IndexPool.Walk(ctx, remote1, func(source string, record api.Record, err error) error {
			target := remote2 + strings.TrimPrefix(source, remote1)

			if err != nil {
				return worker.options.ErrorHandler(source, target, err)
			}

			r1ch <- &object{source, target, record}

			return nil
		}, api.LexographicalOrder, api.NoSkip)
	})

	// Walk through the target directory
	wg.Go(func() error {
		defer close(r2ch)

		return worker.IndexPool.Walk(ctx, remote2, func(target string, record api.Record, err error) error {
			source := remote1 + strings.TrimPrefix(target, remote2)

			if err != nil {
				return worker.options.ErrorHandler(source, target, err)
			}

			r2ch <- &object{source, target, record}

			return nil
		}, api.LexographicalOrder, api.NoSkip)
	})

	// Process the records
	wg.Go(func() error {
		opts := SynchronizeOptions{
			Exclusive:                        worker.options.Exclusive,
			OnlyIfNewer:                      worker.options.OnlyIfNewer,
			CompareChecksums:                 worker.options.CompareChecksums,
			Delete:                           worker.options.Delete,
			DisableUpdateInPlace:             true,
			RemovePerDirectory:               worker.options.RemovePerDirectory,
			SyncModTimeForIdenticalChecksums: worker.options.SyncModTime,
			FilterPatterns:                   worker.options.FilterPatterns,
			IgnorePatterns:                   worker.options.IgnorePatterns,
			ChecksumVerify:                   VerifyRemoteToRemote(worker.IndexPool, worker.IndexPool, worker.options.ProgressHandler),
			ErrorHandler:                     worker.options.ErrorHandler,
		}

		return Synchronize(ctx, checkOrder(r1ch), checkOrder(r2ch), registerProgress(queue, worker.options.ProgressHandler), opts)
	})

	return wg.Wait()
}

// ComputeRemoteCopyDir schedules tasks to synchronize a collection to a
// collection on a different iRODS server. All individual actions are appended to the queue.
// The deleteFirst parameter indicates whether to delete files first before retransferring,
// or whether files might be updated in place.
// The call blocks until the source directory has been completely scanned.
func (worker *Worker) ComputeRemoteCopyDir(ctx context.Context, remote1, remote2 string, queue chan<- Task) error {
	r1ch := make(chan *object)
	r2ch := make(chan *object)

	wg, ctx := errgroup.WithContext(ctx)

	// Walk through the source directory
	wg.Go(func() error {
		defer close(r1ch)

		return worker.IndexPool.Walk(ctx, remote1, func(source string, record api.Record, err error) error {
			target := remote2 + strings.TrimPrefix(source, remote1)

			if err != nil {
				return worker.options.ErrorHandler(source, target, err)
			}

			r1ch <- &object{source, target, record}

			return nil
		}, api.LexographicalOrder, api.NoSkip)
	})

	// Walk through the target directory
	wg.Go(func() error {
		defer close(r2ch)

		return worker.RemoteIndexPool.Walk(ctx, remote2, func(target string, record api.Record, err error) error {
			source := remote1 + strings.TrimPrefix(target, remote2)

			if err != nil {
				return worker.options.ErrorHandler(source, target, err)
			}

			r2ch <- &object{source, target, record}

			return nil
		}, api.LexographicalOrder, api.NoSkip)
	})

	// Process the records
	wg.Go(func() error {
		opts := SynchronizeOptions{
			Exclusive:                        worker.options.Exclusive,
			OnlyIfNewer:                      worker.options.OnlyIfNewer,
			CompareChecksums:                 worker.options.CompareChecksums,
			Delete:                           worker.options.Delete,
			DisableUpdateInPlace:             worker.options.DisableUpdateInPlace,
			RemovePerDirectory:               worker.options.RemovePerDirectory,
			SyncModTimeForIdenticalChecksums: worker.options.SyncModTime,
			FilterPatterns:                   worker.options.FilterPatterns,
			IgnorePatterns:                   worker.options.IgnorePatterns,
			ChecksumVerify:                   VerifyRemoteToRemote(worker.IndexPool, worker.RemoteIndexPool, worker.options.ProgressHandler),
			ErrorHandler:                     worker.options.ErrorHandler,
		}

		return Synchronize(ctx, checkOrder(r1ch), checkOrder(r2ch), registerProgress(queue, worker.options.ProgressHandler), opts)
	})

	return wg.Wait()
}

func toIrodsPath(base, path string) string {
	if path == "" || path == "." {
		return base
	}

	return base + "/" + strings.Join(strings.Split(path, string(os.PathSeparator)), "/")
}

func toLocalPath(base, path string) string {
	if path == "" {
		return base
	}

	return base + strings.Join(strings.Split(path, "/"), string(os.PathSeparator))
}

type SynchronizeOptions struct {
	// Exclusive indicates whether the synchronization should be exclusive.
	// If set, no file in the target will be overwritten.
	Exclusive bool

	// OnlyIfNewer indicates whether files should only be synchronized if the source file is newer than the target file.
	OnlyIfNewer bool

	// CompareChecksums indicates whether checksums should be compared to determine if files are identical.
	CompareChecksums bool

	// Delete indicates whether files should be deleted if they are not present in the source.
	Delete bool

	// DisableUpdateInPlace indicates whether files should be updated in place or not.
	// If set, a data object will be removed and a new data object will be created, instead of updating the original object.
	DisableUpdateInPlace bool

	// RemovePerDirectory indicates that, when removing a directory recursively (RemoveDir),
	// each collection should be removed with a single recursive server-side operation
	// (DeleteCollectionAll) instead of removing every data object individually. Data objects
	// are not removed individually in this mode, since they are removed along with their
	// parent collection.
	RemovePerDirectory bool

	// SyncModTimeForIdenticalChecksums indicates whether the modification time of files should be synchronized,
	// in case the source and target files are considered equal but differ in their modification times.
	// Only applies when CompareChecksums is enabled.
	SyncModTimeForIdenticalChecksums bool

	// FilterPatterns indicates patterns to include.
	// The pattern syntax is the same as filepath.Match. The matching is done against the base name of the file, directories are always included.
	// If multiple patterns are specified, a file must match at least one of them to be included.
	FilterPatterns []string

	// IgnorePatterns indicates patterns to ignore.
	// The pattern syntax is the same as filepath.Match. The matching is done against the base name of the file or directory.
	// IgnorePatterns takes precedence over FilterPatterns. If a file or directory matches an ignore pattern, it will be excluded even if it matches an include pattern.
	IgnorePatterns []string

	// ChecksumVerify indicates the function used to verify checksums.
	// If set, it will be used to compare the checksums of the files to determine if they are identical.
	ChecksumVerify CompareFunc

	// ErrorHandler indicates the function used to handle errors that occur during synchronization.
	// If set, it will be called with the error and the corresponding object.
	ErrorHandler func(source, target string, err error) error
}

func Synchronize(ctx context.Context, left, right <-chan *object, queue chan<- Task, opts SynchronizeOptions) error {
	defer close(queue)

	leftObject, hasLeft := <-left
	rightObject, hasRight := <-right

	for {
		switch {
		case !hasLeft && !hasRight:
			return nil

		case hasLeft && opts.shouldIgnore(leftObject):
			leftObject, hasLeft = skipAll(left, leftObject)

		case hasRight && opts.shouldIgnore(rightObject):
			rightObject, hasRight = skipAll(right, rightObject)

		case !hasLeft, hasRight && api.ComparePaths(leftObject.target, rightObject.target) > 0:
			if opts.Delete {
				rightObject, hasRight = removeAll(right, rightObject, queue, opts)
			} else {
				rightObject, hasRight = skipAll(right, rightObject)
			}

		case !hasRight, api.ComparePaths(leftObject.target, rightObject.target) < 0:
			transferNewCollectionOrObject(leftObject, queue)

			leftObject, hasLeft = <-left

		// Now we have the same iRODS path
		case leftObject.target != rightObject.target:
			panic(fmt.Errorf("expected same iRODS path: %s != %s", leftObject.target, rightObject.target))

		case leftObject.info.IsDir() && rightObject.info.IsDir():
			leftObject, hasLeft = <-left
			rightObject, hasRight = <-right

		case opts.Exclusive:
			leftObject, hasLeft = skipAll(left, leftObject)
			rightObject, hasRight = skipAll(right, rightObject)

		case leftObject.info.Mode()&os.ModeType != rightObject.info.Mode()&os.ModeType:
			rightObject, hasRight = removeAll(right, rightObject, queue, opts)

		default:
			if err := compareAndTransferObject(ctx, leftObject, rightObject, queue, opts); err != nil {
				return err
			}

			leftObject, hasLeft = <-left
			rightObject, hasRight = <-right
		}
	}
}

func (opts SynchronizeOptions) shouldIgnore(obj *object) bool {
	// Ignore files from ignore globs
	for _, pattern := range opts.IgnorePatterns {
		if matched, matchErr := filepath.Match(pattern, obj.info.Name()); matchErr != nil {
			continue
		} else if matched {
			return true
		}
	}

	// FilterPatterns never applies to directories.
	if obj.info.IsDir() {
		return false
	}

	var hasFilterPatterns bool

	// Filter files based on filter globs
	// But skip empty patterns
	for _, pattern := range opts.FilterPatterns {
		if pattern == "" {
			continue
		}

		hasFilterPatterns = true

		if matched, matchErr := filepath.Match(pattern, obj.info.Name()); matchErr != nil {
			continue
		} else if matched {
			return false
		}
	}

	// If no filter patterns are specified, do not filter any files
	return hasFilterPatterns
}

func compareAndTransferObject(ctx context.Context, left, right *object, queue chan<- Task, opts SynchronizeOptions) error { //nolint:funlen
	var checksum []byte

	modTimeCompare := left.info.ModTime().Truncate(time.Second).Compare(right.info.ModTime().Truncate(time.Second))

	switch {
	case left.info.IsDir(), !left.info.Mode().IsRegular():
		return nil

	case opts.OnlyIfNewer && modTimeCompare < 0:
		return nil

	case left.info.Size() != right.info.Size():
		// Retransfer

	case opts.CompareChecksums:
		var err error

		checksum, _, err = opts.ChecksumVerify(ctx, left.source, left.target, left.info, right.info)
		if err == nil {
			if !opts.SyncModTimeForIdenticalChecksums || modTimeCompare == 0 {
				return nil
			}

			// Still set metadata in sync
			queue <- Task{
				Action:  SetModificationTime,
				Source:  left.source,
				Target:  left.target,
				ModTime: left.info.ModTime(),
			}

			return nil
		}

		if errors.Is(err, ErrChecksumMismatch) {
			break
		}

		err = opts.ErrorHandler(left.source, left.target, err)
		if err == nil {
			break
		}

		return err

	case modTimeCompare == 0:
		return nil
	}

	if opts.DisableUpdateInPlace {
		queue <- Task{
			Action: RemoveFile,
			Source: left.source,
			Target: left.target,
		}
	}

	queue <- Task{
		Action:   TransferFile,
		Source:   left.source,
		Target:   left.target,
		Size:     left.info.Size(),
		ModTime:  left.info.ModTime(),
		Checksum: checksum,
	}

	return nil
}

// removeAll recursively queues removal tasks for obj and, if obj is a directory, its
// descendants. If Options.RemovePerDirectory is set, a directory is removed together with
// its descendants using a single recursive server-side operation (see RemoveDir), so
// individual data objects directly inside a directory do not need their own RemoveFile
// task. Since that recursive operation may not scale to an unbounded number of objects,
// only the first maxPerDirectorySkip data objects encountered directly inside a given
// directory are skipped this way; any further data objects in that directory are still
// queued for individual removal. The count resets for each directory.
func removeAll(ch <-chan *object, obj *object, queue chan<- Task, opts SynchronizeOptions) (*object, bool) {
	if obj.info.IsDir() {
		next, ok := <-ch

		skipped := 0

		for ok && strings.HasPrefix(next.target, obj.target+"/") {
			if opts.RemovePerDirectory && !next.info.IsDir() && skipped < maxPerDirectorySkip {
				skipped++

				next, ok = <-ch

				continue
			}

			next, ok = removeAll(ch, next, queue, opts)
		}

		action := RemoveDirectory
		if opts.RemovePerDirectory && skipped > 0 {
			action = RemoveDirectoryAll
		}

		queue <- Task{
			Action: action,
			Source: obj.source,
			Target: obj.target,
		}

		return next, ok
	}

	queue <- Task{
		Action: RemoveFile,
		Source: obj.source,
		Target: obj.target,
	}

	obj, ok := <-ch

	return obj, ok
}

func skipAll(ch <-chan *object, obj *object) (*object, bool) {
	logrus.Debugf("skipping %s [%s]", obj.target, obj.info)

	next, ok := <-ch

	if obj.info.IsDir() {
		for ok && strings.HasPrefix(next.target, obj.target+"/") {
			logrus.Debugf("skipping %s [%s]", next.target, next.info)

			next, ok = <-ch
		}
	}

	return next, ok
}

func transferNewCollectionOrObject(obj *object, queue chan<- Task) {
	if obj.info.IsDir() {
		queue <- Task{
			Action: CreateDirectory,
			Source: obj.source,
			Target: obj.target,
		}

		return
	}

	// Ignore non-regular files
	if !obj.info.Mode().IsRegular() {
		return
	}

	queue <- Task{
		Action:  TransferFile,
		Source:  obj.source,
		Target:  obj.target,
		Size:    obj.info.Size(),
		ModTime: obj.info.ModTime(),
	}
}

func checkOrder(ch <-chan *object) <-chan *object {
	out := make(chan *object)

	go func() {
		defer close(out)

		var prev *object

		for obj := range ch {
			if prev != nil && api.ComparePaths(prev.target, obj.target) >= 0 {
				logrus.Errorf("expected lexicographical order: %s >= %s [%s]", prev.target, obj.target, obj.info)
			}

			prev = obj

			out <- obj
		}
	}()

	return out
}

func registerProgress(queue chan<- Task, progressHandler func(Progress)) chan<- Task {
	if progressHandler == nil {
		return queue
	}

	out := make(chan Task)

	go func() {
		defer close(queue)

		for task := range out {
			progressHandler(Progress{
				Action: task.Action,
				Label:  ProgressLabel(task.Source, task.Target),
				Size:   task.Size,
			})

			queue <- task
		}
	}()

	return out
}
