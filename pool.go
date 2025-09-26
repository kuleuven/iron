package iron

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/kuleuven/iron/api"
	"go.uber.org/multierr"
)

// Pool returns a subset of connections that can be used
// for dedicated API calls. The connections remain in the pool
// until the pool is Closed.
type Pool struct {
	client *Client

	parent   *Pool
	children []*Pool

	maxConns             int
	allowConcurrentUse   bool
	discardConnectionAge time.Duration

	available, all, reused []*conn
	waiting                int
	ready                  chan *conn
	closed                 bool
	closeErr               error
	lock                   sync.Mutex

	*api.API
}

func newPool(client *Client) *Pool {
	pool := &Pool{
		client:               client,
		maxConns:             client.option.MaxConns,
		allowConcurrentUse:   client.option.AllowConcurrentUse,
		discardConnectionAge: client.option.DiscardConnectionAge,
		ready:                make(chan *conn),
	}

	// Register api
	pool.API = &api.API{
		Username: client.env.Username,
		Zone:     client.env.Zone,
		Connect: func(ctx context.Context) (api.Conn, error) {
			return pool.Connect(ctx)
		},
		DefaultResource: client.env.DefaultResource,
	}

	if client.option.Admin {
		pool.Admin = true
	}

	if pool.discardConnectionAge > 0 {
		go pool.discardOldConnectionsLoop()
	}

	return pool
}

func newChildPool(parent *Pool, size int) *Pool {
	child := newPool(parent.client)

	parent.children = append(parent.children, child)
	parent.maxConns -= size

	child.parent = parent
	child.allowConcurrentUse = false
	child.maxConns = size

	return child
}

// Pool returns a subset of connections that can be used
// for dedicated API calls. The connections remain in the pool
// until the pool is Closed.
// The call will block until the requested number of connections are available.
func (c *Client) Pool(size int) (*Pool, error) {
	return c.defaultPool.Pool(size)
}

// Pool returns a subpool of connections
// It will block until the requested number of connections are available.
func (p *Pool) Pool(size int) (*Pool, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	if size < 1 {
		size = p.maxConns
	}

	if size > p.maxConns {
		return nil, fmt.Errorf("%w: parent pool has %d connections, requested %d", ErrNoConnectionsAvailable, p.maxConns, size)
	}

	child := newChildPool(p, size)

	// We need to shrink the parent pool
	for len(p.all) > p.maxConns {
		if len(p.available) > 0 {
			conn := p.available[0]
			p.available = p.available[1:]

			if p.unregister(conn) {
				child.all = append(child.all, conn)
				child.available = append(child.available, conn)

				continue
			}
		}

		// Need to wait for a connection to be ready
		// None available, block until one becomes available
		p.waiting++
		p.lock.Unlock()

		conn := <-p.ready

		p.lock.Lock()

		if conn == nil || !p.unregister(conn) {
			continue
		}

		child.all = append(child.all, conn)
		child.available = append(child.available, conn)
	}

	return child, nil
}

// Connect returns a new connection to the iRODS server. It will first try to reuse an available connection.
// If all connections are busy, it will create a new one up to the maximum number of connections.
// If the maximum number of connections has been reached, it will block until a connection becomes available,
// or reuse an existing connection in case AllowConcurrentUse is enabled.
func (p *Pool) Connect(ctx context.Context) (Conn, error) {
	p.lock.Lock()

	if conn, err := p.tryConnect(ctx); err != ErrNoConnectionsAvailable {
		defer p.lock.Unlock()

		return conn, err
	}

	if p.allowConcurrentUse {
		defer p.lock.Unlock()

		first := p.all[0]

		// Rotate the connection list
		p.all = append(p.all[1:], first)

		// Mark the connection as reused
		p.reused = append(p.reused, first)

		return &returnOnClose{conn: first, pool: p}, nil
	}

	// None available, block until one becomes available
	p.waiting++
	p.lock.Unlock()

	if conn := <-p.ready; conn != nil {
		return &returnOnClose{conn: conn, pool: p}, nil
	}

	p.lock.Lock()
	defer p.lock.Unlock()

	// We received a token from a returned connection that was closed
	// In this case we are allowed to create a new connection
	conn, err := p.newConn(ctx)
	if err != nil {
		return nil, err
	}

	return &returnOnClose{conn: conn, pool: p}, nil
}

// ConnectAvailable returns a list of available connections to the iRODS server,
// up to the specified number. If no connections are available, it will return
// an empty list. Retrieved connections must be closed by the caller.
// If n is negative, it will return all available connections.
func (p *Pool) ConnectAvailable(ctx context.Context, n int) ([]Conn, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	pool := []Conn{}

	for ; n != 0; n-- {
		conn, err := p.tryConnect(ctx)
		if err == ErrNoConnectionsAvailable {
			break
		} else if err != nil {
			err = multierr.Append(err, closeAll(pool))

			return nil, err
		}

		pool = append(pool, conn)
	}

	return pool, nil
}

func closeAll(pool []Conn) error {
	var err error

	for _, conn := range pool {
		err = multierr.Append(err, conn.Close())
	}

	return err
}

var ErrNoConnectionsAvailable = errors.New("no connections available")

func (p *Pool) tryConnect(ctx context.Context) (Conn, error) {
	p.discardOldConnections()

	if len(p.available) > 0 {
		conn := p.available[0]
		p.available = p.available[1:]

		p.client.firstUse.Do(func() {
			if p.client.option.AtFirstUse != nil {
				p.client.option.AtFirstUse(conn.API())
			}
		})

		return &returnOnClose{conn: conn, pool: p}, nil
	}

	if len(p.all) < p.maxConns {
		conn, err := p.newConn(ctx)
		if err != nil {
			return nil, err
		}

		p.client.firstUse.Do(func() {
			if p.client.option.AtFirstUse != nil {
				p.client.option.AtFirstUse(conn.API())
			}
		})

		return &returnOnClose{conn: conn, pool: p}, nil
	}

	return nil, ErrNoConnectionsAvailable
}

func (p *Pool) newConn(ctx context.Context) (*conn, error) {
	conn, err := p.client.newConn(ctx)
	if err != nil {
		return nil, err
	}

	p.all = append(p.all, conn)

	return conn, nil
}

func (p *Pool) returnConn(conn *conn) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	// If the pool is closed, return the connection to the parent pool
	if p.closed && p.parent != nil {
		return p.parent.returnConn(conn)
	}

	// If the connection is reused, remove it from the reused list and return
	for i := range p.reused {
		if p.reused[i] != conn {
			continue
		}

		p.reused = append(p.reused[:i], p.reused[i+1:]...)

		return nil
	}

	// In case of errors, we must discard the connection
	if conn.transportErrors > 0 || conn.sqlErrors > 0 || p.discardConnectionAge > 0 && time.Since(conn.connectedAt) > p.discardConnectionAge {
		if p.unregister(conn) {
			// If someone is waiting for a connection, we must inform them
			// that it is allowed to call newConn()
			if p.waiting > 0 {
				p.waiting--
				p.ready <- nil
			}
		}

		return conn.Close()
	}

	// If someone is waiting for a connection, pass the connection to them
	if p.waiting > 0 {
		p.waiting--
		p.ready <- conn

		return nil
	}

	p.available = append(p.available, conn)

	return nil
}

func (p *Pool) unregister(conn *conn) bool {
	for i := range p.all {
		if p.all[i] != conn {
			continue
		}

		p.all = append(p.all[:i], p.all[i+1:]...)

		return true
	}

	return false
}

func (p *Pool) discardOldConnectionsLoop() {
	ticker := time.NewTicker(p.discardConnectionAge / 2)

	for range ticker.C {
		p.lock.Lock()
		p.discardOldConnections()
		p.lock.Unlock()
	}
}

func (p *Pool) discardOldConnections() {
	if p.discardConnectionAge <= 0 {
		return
	}

	now := time.Now()

	for _, conn := range p.available {
		if now.Sub(conn.connectedAt) <= p.discardConnectionAge {
			continue
		}

		i := slices.Index(p.available, conn)
		j := slices.Index(p.all, conn)

		if i == -1 || j == -1 {
			continue
		}

		p.available = append(p.available[:i], p.available[i+1:]...)
		p.all = append(p.all[:j], p.all[j+1:]...)

		conn.Close()
	}
}

// Close returns all connections managed by the pool to the parent pool.
// Connections that were not returned to the pool yet, will be returned
// to the parent pool later.
func (p *Pool) Close() error {
	p.lock.Lock()

	if p.closed {
		defer p.lock.Unlock()

		return p.closeErr
	}

	// Ensure all children are closed
	children := p.children
	p.lock.Unlock()

	for _, child := range children {
		p.closeErr = multierr.Append(p.closeErr, child.Close())
	}

	p.lock.Lock()
	defer p.lock.Unlock()

	p.closed = true

	// If there is a parent pool, return connections to it
	if p.parent != nil {
		p.parent.lock.Lock() // Slow lock
		defer p.parent.lock.Unlock()

		p.parent.all = append(p.parent.all, p.all...)
		p.parent.available = append(p.parent.available, p.available...)
		p.parent.reused = append(p.parent.reused, p.reused...)
		p.parent.maxConns += p.maxConns

		p.all = nil
		p.available = nil
		p.reused = nil

		return p.closeErr
	}

	// If there is no parent pool, close all connections
	for _, conn := range p.all {
		p.closeErr = multierr.Append(p.closeErr, conn.Close())
	}

	return p.closeErr
}

type returnOnClose struct {
	*conn
	once     sync.Once
	closeErr error
	pool     *Pool
}

func (r *returnOnClose) Close() error {
	r.once.Do(func() {
		r.closeErr = r.pool.returnConn(r.conn)
	})

	return r.closeErr
}
