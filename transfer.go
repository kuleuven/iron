package iron

import (
	"context"
	"io"

	"github.com/kuleuven/iron/transfer"
)

// Upload uploads a local file to the iRODS server using parallel transfers.
// The local file refers to the local file system. The remote file refers to an iRODS path.
func (c *Client) Upload(ctx context.Context, local, remote string, options transfer.Options) error {
	return c.runWorker(options, func(worker *transfer.Worker) {
		worker.Upload(ctx, local, remote)
	})
}

// UploadDir uploads a local directory to the iRODS server using parallel transfers.
// The local file refers to the local file system. The remote file refers to an iRODS path.
func (c *Client) UploadDir(ctx context.Context, local, remote string, options transfer.Options) error {
	return c.runWorker(options, func(worker *transfer.Worker) {
		worker.UploadDir(ctx, local, remote)
	})
}

// FromReader streams an io.Reader to a remote file on the iRODS server using parallel transfers.
// The remote file refers to an iRODS path.
func (c *Client) FromReader(ctx context.Context, r io.Reader, remote string, appendToFile bool, options transfer.Options) error {
	return c.runWorker(options, func(worker *transfer.Worker) {
		worker.FromStream(ctx, remote, r, remote, appendToFile)
	})
}

// Download downloads a remote file from the iRODS server using parallel transfers.
// The local file refers to the local file system. The remote file refers to an iRODS path.
func (c *Client) Download(ctx context.Context, local, remote string, options transfer.Options) error {
	return c.runWorker(options, func(worker *transfer.Worker) {
		worker.Download(ctx, local, remote)
	})
}

// DownloadDir downloads a remote directory from the iRODS server using parallel transfers.
// The local file refers to the local file system. The remote file refers to an iRODS path.
func (c *Client) DownloadDir(ctx context.Context, local, remote string, options transfer.Options) error {
	return c.runWorker(options, func(worker *transfer.Worker) {
		worker.DownloadDir(ctx, local, remote)
	})
}

// ToWriter streams a remote file from the iRODS server to an io.Writer using parallel transfers.
// The remote file refers to an iRODS path.
func (c *Client) ToWriter(ctx context.Context, w io.Writer, remote string, options transfer.Options) error {
	return c.runWorker(options, func(worker *transfer.Worker) {
		worker.ToStream(ctx, remote, w, remote)
	})
}

// RemoveDir removes a remote directory from the iRODS server using client recursion.
// The remote file refers to an iRODS path.
func (c *Client) RemoveDir(ctx context.Context, remote string, options transfer.Options) error {
	return c.runWorker(options, func(worker *transfer.Worker) {
		worker.RemoveDir(ctx, remote)
	})
}

// CopyDir copies a remote directory to another remote directory from the iRODS server using client recursion.
// The remote files refers to an iRODS path.
func (c *Client) CopyDir(ctx context.Context, remote1, remote2 string, options transfer.Options) error {
	return c.runWorker(options, func(worker *transfer.Worker) {
		worker.CopyDir(ctx, remote1, remote2)
	})
}

// ComputeChecksums computes the checksums of a remote directory on the iRODS server using client recursion.
// The remote file refers to an iRODS path.
func (c *Client) ComputeChecksums(ctx context.Context, remote string, options transfer.Options) error {
	return c.runWorker(options, func(worker *transfer.Worker) {
		worker.ComputeChecksums(ctx, remote)
	})
}

// runWorker creates a new pool with the given number of threads and
// creates a new transfer.Worker with it. The callback function is called
// with the created worker. The worker is started and the error returned
// is the error returned by the worker's Wait() function.
func (c *Client) runWorker(options transfer.Options, callback func(worker *transfer.Worker)) error {
	pool, err := c.defaultPool.Pool(options.MaxThreads)
	if err != nil {
		return err
	}

	defer pool.Close()

	worker := transfer.New(c.API, pool.API, options)

	callback(worker)

	return worker.Wait()
}

// Verify checks the checksum of a local file against the checksum of a remote file
func (c *Client) Verify(ctx context.Context, local, remote string) error {
	return transfer.Verify(c.API, nil)(ctx, local, remote, nil, nil)
}
