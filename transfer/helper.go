package transfer

import (
	"context"

	"github.com/kuleuven/iron/api"
)

// ConnectionAsAPI returns a new api.API instance that uses the given connection as each connection.
// The returned api.API instance will always use the provided connection for its Connect method,
// ignoring any connection pooling or other connection logic in the parent API.
// Note that the resulting *ConnAPI must be closed to release the underlying connection.
func ConnectionAsAPI(parent *api.API, conn api.Conn) *connAPI {
	child := *parent

	// Build channel with single connection
	ch := make(chan api.Conn, 1)
	ch <- conn

	// Override the Connect method to always return the single connection from the channel
	child.Connect = func(ctx context.Context) (api.Conn, error) {
		conn := <-ch

		return &returnOnClose{
			Conn: conn,
			ch:   ch,
		}, nil
	}

	return &connAPI{
		ch:  ch,
		API: &child,
	}
}

type connAPI struct {
	ch chan api.Conn
	*api.API
}

func (c *connAPI) Close() error {
	close(c.ch)

	for conn := range c.ch {
		conn.Close()
	}

	return nil
}

type returnOnClose struct {
	api.Conn
	ch chan<- api.Conn
}

func (r *returnOnClose) Close() error {
	r.ch <- r.Conn

	return nil
}
