// Package iron provides an interface to IRODS.
package iron

import (
	"context"
	"sync"
	"time"

	"github.com/kuleuven/iron/api"
	"github.com/kuleuven/iron/msg"
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

	// Admin is a flag that indicates whether the client should act in admin mode.
	Admin bool

	// Experimental: UseNativeProtocol will force the use of the native protocol.
	// This is an experimental feature and may be removed in a future version.
	UseNativeProtocol bool

	// EnvCallback is an optional function that returns the environment settings for the connection
	// when a new connection is established. If not provided, the default environment settings are used.
	// This is useful in combination with the DeferConnectionToFirstUse option, to prepare the client
	// before the connection parameters are known. The returned time.Time is the time until which the
	// environment settings are valid, or zero if they are valid indefinitely.
	EnvCallback func() (Env, time.Time, error)

	// AuthenticationPrompt is an optional function that overrides the default pompt function.
	// It is used to prompt the user for information when authenticating with the server,
	// if the authentication scheme is pam_interactive.
	AuthenticationPrompt Prompt

	// ForceReauthentication is a flag that indicates whether the client should discard any previously stored credentials
	// and force reauthentication with the server.
	ForceReauthentication bool

	// DialFunc is an optional function that overrides the default dial function.
	DialFunc DialFunc

	// HandshakeFunc is an optional function that overrides the default handshake function.
	HandshakeFunc HandshakeFunc

	// DiscardConnectionAge is the maximum age of a connection before it is discarded.
	DiscardConnectionAge time.Duration
}

type HandshakeFunc func(ctx context.Context) (Conn, error)

type Client struct {
	env                  *Env
	option               Option
	protocol             msg.Protocol
	nativePassword       string
	envCallbackExpiry    time.Time
	nativePasswordExpiry time.Time
	defaultPool          *Pool
	firstUse             sync.Once
	lock                 sync.Mutex
	*api.API
}

// New creates a new Client instance with the provided environment settings, maximum connections, and options.
// The environment settings are used for dialing new connections.
// The maximum number of connections is the maximum number of connections that can be established at any given time.
// The options are used to customize the behavior of the client.
func New(ctx context.Context, env Env, option Option) (*Client, error) {
	env.ApplyDefaults()

	// Handle force reauthentication by discarding any previously stored credentials.
	// This ensures that the client will prompt for new credentials when connecting to the server.
	if option.ForceReauthentication {
		env.Password = ""
		env.PersistentState = &forceReauthenticationPersistentState{env.PersistentState}
	}

	if option.MaxConns <= 0 {
		option.MaxConns = 1
	}

	c := &Client{
		env:      &env,
		option:   option,
		protocol: msg.XML,
	}

	if option.UseNativeProtocol {
		c.protocol = msg.Native
	}

	// Create default connection pool
	c.defaultPool = newPool(c)

	c.API = c.defaultPool.API

	// Test first connection unless deferred
	if !option.DeferConnectionToFirstUse {
		conn, err := c.defaultPool.newConn(ctx)
		if err != nil {
			return nil, err
		}

		c.defaultPool.available = append(c.defaultPool.available, conn)
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
	if err := c.refreshEnv(); err != nil {
		// We currently ignore the error and return an empty environment.
		return Env{}
	}

	return *c.env
}

func (c *Client) refreshEnv() error {
	if !c.needsEnvCallback() {
		return nil
	}

	env, expiry, err := c.option.EnvCallback()
	if err != nil {
		return err
	}

	c.env = &env
	c.envCallbackExpiry = expiry
	c.Username = env.Username
	c.Zone = env.Zone
	// c.DefaultResource = env.DefaultResource
	c.nativePasswordExpiry = time.Time{}

	if expiry.IsZero() {
		c.option.EnvCallback = nil // No need to call the callback again
	}

	return nil
}

func (c *Client) needsEnvCallback() bool {
	return c.option.EnvCallback != nil && (c.envCallbackExpiry.IsZero() || time.Now().After(c.envCallbackExpiry))
}

// Connect returns a new connection to the iRODS server. It will first try to reuse an available connection.
// If all connections are busy, it will create a new one up to the maximum number of connections.
// If the maximum number of connections has been reached, it will block until a connection becomes available,
// or reuse an existing connection in case AllowConcurrentUse is enabled.
func (c *Client) Connect(ctx context.Context) (Conn, error) {
	return c.defaultPool.Connect(ctx)
}

// ConnectAvailable returns a list of available connections to the iRODS server,
// up to the specified number. If no connections are available, it will return
// an empty list. Retrieved connections must be closed by the caller.
// If n is negative, it will return all available connections.
func (c *Client) ConnectAvailable(ctx context.Context, n int) ([]Conn, error) {
	return c.defaultPool.ConnectAvailable(ctx, n)
}

// Close closes all connections managed by the client, ensuring that any errors
// encountered during the closing process are aggregated and returned. The method
// is safe to call multiple times and locks the client during execution to prevent
// concurrent modifications to the connections.
func (c *Client) Close() error {
	return c.defaultPool.Close()
}

func (c *Client) newConn(ctx context.Context) (Conn, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.option.HandshakeFunc != nil {
		conn, err := c.option.HandshakeFunc(ctx)
		if err != nil {
			return nil, err
		}

		return conn, nil
	}

	// If an EnvCallback is provided, use it to retrieve the environment settings
	if err := c.refreshEnv(); err != nil {
		return nil, err
	}

	env := *c.env

	// Only use pam_password for first connection
	if env.AuthScheme != native && time.Now().Before(c.nativePasswordExpiry) {
		env.AuthScheme = native
		env.Password = c.nativePassword
	}

	// Load persisted password if available.
	if env.AuthScheme != native || env.Password == "" {
		if password, err := env.PersistentState.LoadPassword(); err == nil {
			env.Password = password
			env.AuthScheme = native
		}
	}

	conn, err := dial(ctx, env, c.option.ClientName, c.option.DialFunc, c.option.AuthenticationPrompt, c.protocol)

	// If the authentication scheme was not native, but we tried using a native password that was obtained before,
	// and it expired, we'll retry with the original authentication scheme.
	if code, ok := api.ErrorCode(err); ok &&
		(code == msg.CAT_PASSWORD_EXPIRED || code == msg.CAT_INVALID_AUTHENTICATION) &&
		c.env.AuthScheme != native && env.AuthScheme == native {
		// Retry with the original authentication scheme
		env.AuthScheme = c.env.AuthScheme
		env.Password = ""

		conn, err = dial(ctx, env, c.option.ClientName, c.option.DialFunc, c.option.AuthenticationPrompt, c.protocol)
	}

	// Capture all other errors here
	if err != nil {
		return nil, err
	}

	// Save pam_password for next connection
	if env.AuthScheme != native {
		c.nativePassword = conn.NativePassword()

		// Attempt store in the persistent state, but do not fail the connection if it fails.
		c.env.PersistentState.SavePassword(c.nativePassword) //nolint:errcheck

		// Calculate actual TTL
		actualTTL := min(defaultMinimumTTL, time.Duration(determineTTL(conn.env.GeneratedPasswordTimeout))*time.Hour)

		c.nativePasswordExpiry = conn.connectedAt.Add(actualTTL)
	}

	return conn, nil
}

const defaultMinimumTTL = 121 * time.Second
