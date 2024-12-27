package iron

import (
	"context"
	"sync"

	"gitea.icts.kuleuven.be/coz/iron/api"
	"go.uber.org/multierr"
)

type Client struct {
	ctx       context.Context //nolint:containedctx
	env       *Env
	option    Option
	available chan *conn
	all       []*conn
	maxConns  int
	dialErr   error
	lock      sync.Mutex
	api.API
}

// New creates a new Client instance with the provided environment settings, maximum connections, and options.
// The context and environment settings are used for dialing new connections.
// The maximum number of connections is the maximum number of connections that can be established at any given time.
// The options are used to customize the behavior of the client.
func New(ctx context.Context, env Env, maxConns int, option Option) (*Client, error) {
	env.ApplyDefaults()

	if maxConns <= 0 {
		maxConns = 1
	}

	c := &Client{
		ctx:       ctx,
		env:       &env,
		option:    option,
		available: make(chan *conn, maxConns),
		maxConns:  maxConns,
	}

	// Register api
	c.API = api.New(func(ctx context.Context) (api.Conn, error) {
		return c.Connect()
	}, env.DefaultResource)

	// Test first connection unless deferred
	if !option.ConnectAtFirstUse {
		conn, err := dial(ctx, env, c.option)
		if err != nil {
			return nil, err
		}

		c.all = append(c.all, conn)

		c.available <- conn
	}

	return c, nil
}

// Connect returns a new connection to the iRODS server. It will first try to reuse an available connection.
// If all connections are busy, it will create a new one up to the maximum number of connections.
// If the maximum number of connections has been reached, it will block until a connection becomes available.
func (c *Client) Connect() (Conn, error) {
	if len(c.available) > 0 {
		return &returnOnClose{<-c.available, c}, nil
	}

	c.lock.Lock()

	if len(c.all) < c.maxConns {
		defer c.lock.Unlock()

		return c.newConn()
	}

	c.lock.Unlock()

	return &returnOnClose{<-c.available, c}, nil
}

func (c *Client) newConn() (Conn, error) {
	if c.dialErr != nil {
		// Dial has already failed, return the same error without retrying
		return nil, c.dialErr
	}

	env := *c.env

	// Only use pam_password for first connection
	if len(c.all) > 0 && env.AuthScheme != native {
		env.AuthScheme = native
		env.Password = c.all[0].NativePassword
	}

	conn, err := dial(c.ctx, env, c.option)
	if err != nil {
		c.dialErr = err

		return nil, err
	}

	c.all = append(c.all, conn)

	return &returnOnClose{conn, c}, nil
}

type returnOnClose struct {
	*conn
	client *Client
}

func (r *returnOnClose) Close() error {
	r.client.available <- r.conn

	return nil
}

// Close closes all connections managed by the client, ensuring that any errors
// encountered during the closing process are aggregated and returned. The method
// is safe to call multiple times and locks the client during execution to prevent
// concurrent modifications to the connections.
func (c *Client) Close() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	var err error

	for _, conn := range c.all {
		err = multierr.Append(err, conn.Close())
	}

	return err
}

// Context returns the context used by the client for all of its operations.
func (c *Client) Context() context.Context {
	return c.ctx
}
