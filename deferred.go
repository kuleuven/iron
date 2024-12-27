package iron

import (
	"context"
	"net"
	"sync"

	"gitea.icts.kuleuven.be/coz/iron/msg"
)

type deferred struct {
	Context context.Context //nolint:containedctx
	Env     Env
	Option  Option

	*conn
	dialErr error
	sync.Mutex
}

func (d *deferred) Conn() net.Conn {
	d.Lock()
	defer d.Unlock()

	if d.conn == nil {
		return nil
	}

	return d.conn.Conn()
}

func (d *deferred) Request(ctx context.Context, apiNumber msg.APINumber, request, response any) error {
	return d.RequestWithBuffers(ctx, apiNumber, request, response, nil, nil)
}

func (d *deferred) RequestWithBuffers(ctx context.Context, apiNumber msg.APINumber, request, response any, requestBuf, responseBuf []byte) error {
	d.Lock()
	defer d.Unlock()

	if d.dialErr != nil {
		return d.dialErr
	}

	if d.conn == nil {
		d.conn, d.dialErr = dial(ctx, d.Env, d.Option)
	}

	if d.dialErr != nil {
		return d.dialErr
	}

	return d.conn.RequestWithBuffers(ctx, apiNumber, request, response, requestBuf, responseBuf)
}

func (d *deferred) Close() error {
	d.Lock()
	defer d.Unlock()

	if d.conn == nil {
		return nil
	}

	return d.conn.Close()
}
