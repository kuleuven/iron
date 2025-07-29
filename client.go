// Package iron provides an interface to IRODS.
package iron

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/kuleuven/iron/api"
	"github.com/kuleuven/iron/msg"
	"go.uber.org/multierr"
)

type Option struct {
	// ClientName is passed to the server as the client type
	ClientName string

	// DeferConnectionToFirstUse will defer the creation of the initial connection to
	// the first use of the Connect() method.
	DeferConnectionToFirstUse bool

	// AtFirstUse is an optional function that is called when the first connection is established,
	// before the connection is returned to the caller of Connect().
	AtFirstUse func(*api.API)

	// Maximum number of connections that can be established at any given time.
	MaxConns int

	// AllowConcurrentUse will allow multiple goroutines to use the same connection concurrently,
	// if the maximum number of connections has been reached and no connection is available.
	// Connect() will cycle through the existing connections.
	AllowConcurrentUse bool

	// EnvCallback is an optional function that returns the environment settings for the connection
	// when a new connection is established. If not provided, the default environment settings are used.
	// This is useful in combination with the DeferConnectionToFirstUse option, to prepare the client
	// before the connection parameters are known. The returned time.Time is the time until which the
	// environment settings are valid, or zero if they are valid indefinitely.
	EnvCallback func() (Env, time.Time, error)

	// Admin is a flag that indicates whether the client should act in admin mode.
	Admin bool

	// Experimental: UseNativeProtocol will force the use of the native protocol.
	// This is an experimental feature and may be removed in a future version.
	UseNativeProtocol bool

	// GeneratedNativePasswordAge is the maximum age of a generated native password before it is discarded.
	// In case pam authentication is used, this should be put to a value lower than the PAM timeout which is set on the server.
	GeneratedNativePasswordAge time.Duration

	// DiscardConnectionAge is the maximum age of a connection before it is discarded.
	DiscardConnectionAge time.Duration

	// DialFunc is an optional function that overrides the default dial function.
	DialFunc DialFunc
}

type Client struct {
	ctx                    context.Context //nolint:containedctx
	env                    *Env
	option                 Option
	protocol               msg.Protocol
	nativePassword         string
	envCallbackExpiry      time.Time
	nativePasswordExpiry   time.Time
	available, all, reused []*conn
	waiting                int
	ready                  chan *conn
	dialErr                error
	firstUse               sync.Once
	lock                   sync.Mutex
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
		ctx:      ctx,
		env:      &env,
		option:   option,
		protocol: msg.XML,
		ready:    make(chan *conn, option.MaxConns),
	}

	if option.UseNativeProtocol {
		c.protocol = msg.Native
	}

	// Register api
	c.API = &api.API{
		Username: env.Username,
		Zone:     env.Zone,
		Connect: func(ctx context.Context) (api.Conn, error) {
			return c.Connect()
		},
		DefaultResource: env.DefaultResource,
	}

	if option.Admin {
		c.Admin = true
	}

	// Test first connection unless deferred
	if !option.DeferConnectionToFirstUse {
		conn, err := c.newConn()
		if err != nil {
			return nil, err
		}

		c.available = append(c.available, conn)
	}

	if option.DiscardConnectionAge > 0 {
		go c.discardOldConnectionsLoop()
	}

	return c, nil
}

// Option returns the client options.
func (c *Client) Option() Option {
	return c.option
}

// Env returns the client environment.
func (c *Client) Env() Env {
	c.lock.Lock()

	defer c.lock.Unlock()

	// If an EnvCallback is provided, use it to retrieve the environment settings
	if c.needsEnvCallback() {
		if c.dialErr != nil {
			return Env{}
		}

		env, expiry, err := c.option.EnvCallback()
		if err != nil {
			c.dialErr = err

			return Env{}
		}

		c.env = &env
		c.envCallbackExpiry = expiry
		c.Username = env.Username
		c.Zone = env.Zone
		c.DefaultResource = env.DefaultResource
		c.nativePasswordExpiry = time.Time{}

		if expiry.IsZero() {
			c.option.EnvCallback = nil // No need to call the callback again
		}
	}

	return *c.env
}

func (c *Client) needsEnvCallback() bool {
	return c.option.EnvCallback != nil && (c.envCallbackExpiry.IsZero() || time.Now().After(c.envCallbackExpiry))
}

// TryConnect tries to connect to the iRODS server,
// by either returning an available connection or creating a new one.
// If the maximum number of connections has been reached, it will return ErrNoConnectionsAvailable.
func (c *Client) TryConnect() (Conn, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.tryConnect()
}

var ErrNoConnectionsAvailable = errors.New("no connections available")

func (c *Client) tryConnect() (Conn, error) {
	c.discardOldConnections()

	if len(c.available) > 0 {
		conn := c.available[0]
		c.available = c.available[1:]

		return &returnOnClose{conn: conn, client: c}, nil
	}

	if len(c.all) < c.option.MaxConns {
		conn, err := c.newConn()
		if err != nil {
			return nil, err
		}

		c.firstUse.Do(func() {
			if c.option.AtFirstUse != nil {
				c.option.AtFirstUse(conn.API())
			}
		})

		return &returnOnClose{conn: conn, client: c}, nil
	}

	return nil, ErrNoConnectionsAvailable
}

// Connect returns a new connection to the iRODS server. It will first try to reuse an available connection.
// If all connections are busy, it will create a new one up to the maximum number of connections.
// If the maximum number of connections has been reached, it will block until a connection becomes available,
// or reuse an existing connection in case AllowConcurrentUse is enabled.
func (c *Client) Connect() (Conn, error) {
	c.lock.Lock()

	if conn, err := c.tryConnect(); err != ErrNoConnectionsAvailable {
		defer c.lock.Unlock()

		return conn, err
	}

	if c.option.AllowConcurrentUse {
		defer c.lock.Unlock()

		first := c.all[0]

		// Rotate the connection list
		c.all = append(c.all[1:], first)

		// Mark the connection as reused
		c.reused = append(c.reused, first)

		return &returnOnClose{conn: first, client: c}, nil
	}

	// None available, block until one becomes available
	c.waiting++
	c.lock.Unlock()

	if conn := <-c.ready; conn != nil {
		return &returnOnClose{conn: conn, client: c}, nil
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	// We received a token from a returned connection that was closed
	// In this case we are allowed to create a new connection
	conn, err := c.newConn()
	if err != nil {
		return nil, err
	}

	return &returnOnClose{conn: conn, client: c}, nil
}

func (c *Client) newConn() (*conn, error) {
	if c.dialErr != nil {
		// Dial has already failed, return the same error without retrying
		return nil, c.dialErr
	}

	// If an EnvCallback is provided, use it to retrieve the environment settings
	if c.needsEnvCallback() {
		env, expiry, err := c.option.EnvCallback()
		if err != nil {
			c.dialErr = err

			return nil, err
		}

		c.env = &env
		c.envCallbackExpiry = expiry
		c.Username = env.Username
		c.Zone = env.Zone
		c.DefaultResource = env.DefaultResource
		c.nativePasswordExpiry = time.Time{}

		if expiry.IsZero() {
			c.option.EnvCallback = nil // No need to call the callback again
		}
	}

	env := *c.env

	// Only use pam_password for first connection
	if env.AuthScheme != native && time.Now().Before(c.nativePasswordExpiry) {
		env.AuthScheme = native
		env.Password = c.nativePassword
	}

	conn, err := dial(c.ctx, env, c.option.ClientName, c.option.DialFunc, c.protocol)
	if err != nil {
		c.dialErr = err

		return nil, err
	}

	// Save pam_password for next connection
	if env.AuthScheme != native {
		c.nativePassword = conn.NativePassword()
		c.nativePasswordExpiry = conn.connectedAt.Add(c.option.GeneratedNativePasswordAge)
	}

	c.all = append(c.all, conn)

	return conn, nil
}

func (c *Client) returnConn(conn *conn) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	// If the connection is reused, remove it from the reused list and return
	for i := range c.reused {
		if c.reused[i] != conn {
			continue
		}

		c.reused = append(c.reused[:i], c.reused[i+1:]...)

		return nil
	}

	if conn.transportErrors > 0 || conn.sqlErrors > 0 || c.option.DiscardConnectionAge > 0 && time.Since(conn.connectedAt) > c.option.DiscardConnectionAge {
		for i := range c.all {
			if c.all[i] != conn {
				continue
			}

			c.all = append(c.all[:i], c.all[i+1:]...)

			// If someone is waiting for a connection, we must inform them
			// that it is allowed to call newConn()
			if c.waiting > 0 {
				c.waiting--
				c.ready <- nil
			}

			return conn.Close()
		}
	}

	if c.waiting > 0 {
		c.waiting--
		c.ready <- conn

		return nil
	}

	c.available = append(c.available, conn)

	return nil
}

func (c *Client) discardOldConnectionsLoop() {
	ticker := time.NewTicker(c.option.DiscardConnectionAge / 2)

	for range ticker.C {
		c.lock.Lock()
		c.discardOldConnections()
		c.lock.Unlock()
	}
}

func (c *Client) discardOldConnections() {
	if c.option.DiscardConnectionAge <= 0 {
		return
	}

	now := time.Now()

	for i, conn := range c.available {
		if now.Sub(conn.connectedAt) <= c.option.DiscardConnectionAge {
			continue
		}

		j := slices.Index(c.all, conn)

		if j == -1 {
			continue
		}

		c.available = append(c.available[:i], c.available[i+1:]...)
		c.all = append(c.all[:j], c.all[j+1:]...)

		conn.Close()
	}
}

type dummyCloser struct {
	*conn
}

func (*dummyCloser) Close() error {
	return nil
}

type returnOnClose struct {
	*conn
	once     sync.Once
	closeErr error
	client   *Client
}

func (r *returnOnClose) Close() error {
	r.once.Do(func() {
		r.closeErr = r.client.returnConn(r.conn)
	})

	return r.closeErr
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
