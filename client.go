package iron

import (
	"context"
	"sync"

	"gitea.icts.kuleuven.be/coz/iron/api"
	"go.uber.org/multierr"
)

type Option struct {
	// ClientName is passed to the server as the client type
	ClientName string

	// DeferConnectionToFirstUse will defer the creation of the initial connection to
	// the first use of the Connect() method.
	DeferConnectionToFirstUse bool

	// Maximum number of connections that can be established at any given time.
	MaxConns int

	// AllowConcurrentUse will allow multiple goroutines to use the same connection concurrently,
	// if the maximum number of connections has been reached and no connection is available.
	// Connect() will cycle through the existing connections.
	AllowConcurrentUse bool
}

type Client struct {
	ctx       context.Context //nolint:containedctx
	env       *Env
	option    Option
	available chan *conn
	all       []*conn
	dialErr   error
	lock      sync.Mutex
	*api.API
}

// New creates a new Client instance with the provided environment settings, maximum connections, and options.
// The context and environment settings are used for dialing new connections.
// The maximum number of connections is the maximum number of connections that can be established at any given time.
// The options are used to customize the behavior of the client.
func New(ctx context.Context, env Env, option Option) (*Client, error) {
	env.ApplyDefaults()

	if option.MaxConns <= 0 {
		option.MaxConns = 1
	}

	c := &Client{
		ctx:       ctx,
		env:       &env,
		option:    option,
		available: make(chan *conn, option.MaxConns),
	}

	// Register api
	c.API = api.New(func(ctx context.Context) (api.Conn, error) {
		return c.Connect()
	}, env.DefaultResource)

	// Test first connection unless deferred
	if !option.DeferConnectionToFirstUse {
		conn, err := dial(ctx, env, c.option.ClientName)
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

	if len(c.all) < c.option.MaxConns {
		defer c.lock.Unlock()

		conn, err := c.newConn()
		if err != nil {
			return nil, err
		}

		return &returnOnClose{conn, c}, nil
	}

	if c.option.AllowConcurrentUse {
		defer c.lock.Unlock()

		first := c.all[0]

		// Rotate the connection list
		c.all = append(c.all[1:], first)

		return &dummyCloser{first}, nil
	}

	c.lock.Unlock()

	return &returnOnClose{<-c.available, c}, nil
}

func (c *Client) newConn() (*conn, error) {
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

	conn, err := dial(c.ctx, env, c.option.ClientName)
	if err != nil {
		c.dialErr = err

		return nil, err
	}

	c.all = append(c.all, conn)

	return conn, nil
}

type dummyCloser struct {
	*conn
}

func (*dummyCloser) Close() error {
	return nil
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
