package iron

import (
	"errors"

	"github.com/kuleuven/iron/transfer"
)

// RunWorker creates a new pool with the given number of threads and
// creates a new transfer.Worker with it. The callback function is called
// with the created worker. It waits for the worker and returns its error along
// with any pool cleanup error.
func (c *Client) RunWorker(options transfer.Options, callback func(worker *transfer.Worker)) (err error) {
	pool, err := c.defaultPool.Pool(options.MaxThreads)
	if err != nil {
		return err
	}

	defer func() { err = errors.Join(err, pool.Close()) }()

	worker := transfer.New(c.API, pool.API, options)

	callback(worker)

	return worker.Wait()
}
