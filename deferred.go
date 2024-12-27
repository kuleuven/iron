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
	Context     context.Context //nolint:containedctx
	EnvCallback func() (Env, error)
	Option      Option

	*conn
	dialErr error
	sync.Mutex
}

func (d *Deferred) init() {
	if d.conn != nil || d.dialErr != nil {
		return
	}

	env, err := d.EnvCallback()
	if err != nil {
		d.dialErr = err

		return
	}

	d.conn, d.dialErr = dial(d.Context, env, d.Option)
}

// Env returns the connection environment
// If the connection is not yet established, an empty Env is returned.
func (d *Deferred) Env() Env {
	d.Lock()
	defer d.Unlock()

	if d.conn == nil {
		return Env{}
	}

	return d.conn.Env()
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

// Initialize dials the connection if it is not yet established.
func (d *Deferred) Initialize() error {
	d.Lock()
	defer d.Unlock()

	d.init()

	return d.dialErr
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

	d.init()

	if d.dialErr != nil {
		return d.dialErr
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
