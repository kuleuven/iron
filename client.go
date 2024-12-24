package iron

import (
	"context"
	"sync"

	"gitea.icts.kuleuven.be/coz/iron/api"
	"go.uber.org/multierr"
)

type Client struct {
	env       *Env
	option    string
	available chan *conn
	all       []*conn
	maxConns  int
	dialErr   error
	lock      sync.Mutex
	api.API
}

func New(env Env, option string, maxConns int) (*Client, error) {
	env.ApplyDefaults()

	if maxConns <= 0 {
		maxConns = 1
	}

	c := &Client{
		env:       &env,
		option:    option,
		available: make(chan *conn, maxConns),
		maxConns:  maxConns,
	}

	// Register api
	c.API = api.New(func(ctx context.Context) (api.Conn, error) {
		return c.Connect(ctx)
	}, env.DefaultResource)

	return c, nil
}

func (c *Client) Connect(ctx context.Context) (Conn, error) {
	if len(c.available) > 0 {
		return &returnOnClose{<-c.available, c}, nil
	}

	c.lock.Lock()

	if len(c.all) < c.maxConns {
		defer c.lock.Unlock()

		return c.newConn(ctx)
	}

	c.lock.Unlock()

	return &returnOnClose{<-c.available, c}, nil
}

func (c *Client) newConn(ctx context.Context) (Conn, error) {
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

	conn, err := dial(ctx, env, c.option)
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

func (c *Client) Close() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	var err error

	for _, conn := range c.all {
		err = multierr.Append(err, conn.Close())
	}

	return err
}
