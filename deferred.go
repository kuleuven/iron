package iron

import (
	"context"
	"net"
	"sync"

	"gitea.icts.kuleuven.be/coz/iron/msg"
)

var _ Conn = (*Deferred)(nil)

// Deferred represents a deferred connection to an iRODS server.
// The provided Env function is only called on first use by a Request method,
// to retrieve connection parameters and dial the connection.
// Option.ConnectAtFirstUse is ignored (or assumed to be true).
type Deferred struct {
	Context context.Context //nolint:containedctx
	Env     func() (Env, error)
	Option  Option

	*conn
	dialErr error
	sync.Mutex
}

// Conn returns the underlying network connection.
// If the connection is not yet established, nil is returned.
func (d *Deferred) Conn() net.Conn {
	d.Lock()
	defer d.Unlock()

	if d.conn == nil {
		return nil
	}

	return d.conn.Conn()
}

// Request sends an API request to the server and expects a API reply.
// The underlying connection is established on first use, using the provided Env
// function to retrieve connection parameters and dial the connection.
func (d *Deferred) Request(ctx context.Context, apiNumber msg.APINumber, request, response any) error {
	return d.RequestWithBuffers(ctx, apiNumber, request, response, nil, nil)
}

// RequestWithBuffers sends an API request to the server and expects a API reply.
// The underlying connection is established on first use, using the provided Env
// function to retrieve connection parameters and dial the connection.
func (d *Deferred) RequestWithBuffers(ctx context.Context, apiNumber msg.APINumber, request, response any, requestBuf, responseBuf []byte) error {
	d.Lock()
	defer d.Unlock()

	if d.dialErr != nil {
		return d.dialErr
	}

	if d.conn == nil {
		env, err := d.Env()
		if err != nil {
			d.dialErr = err

			return err
		}

		d.conn, d.dialErr = dial(ctx, env, d.Option)
		if d.dialErr != nil {
			return d.dialErr
		}
	}

	return d.conn.RequestWithBuffers(ctx, apiNumber, request, response, requestBuf, responseBuf)
}

// Close closes the underlying connection if it was established.
// If the connection was not yet established, this is a no-op.
func (d *Deferred) Close() error {
	d.Lock()
	defer d.Unlock()

	if d.conn == nil {
		return nil
	}

	return d.conn.Close()
}
