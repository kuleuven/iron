package api

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/kuleuven/iron/msg"
)

// mock an API for testing purposes
var testAPI = &API{
	Username: "testuser",
	Zone:     "testzone",
	Connect: func(context.Context) (Conn, error) {
		return testConn, nil
	},
	DefaultResource: "demoResc",
}

var testConn = &mockConn{}

type mockConn struct {
	NextResponse  any
	NextResponses []any
	NextBin       []byte
	LastRequest   any
}

func (c *mockConn) ClientSignature() string {
	return "testsignature"
}

func (c *mockConn) NativePassword() string {
	return "testpassword"
}

func (c *mockConn) Request(ctx context.Context, apiNumber msg.APINumber, request, response any) error {
	return c.RequestWithBuffers(ctx, apiNumber, request, response, nil, nil)
}

func (c *mockConn) RequestWithBuffers(ctx context.Context, apiNumber msg.APINumber, request, response any, _, responseBuf []byte) error {
	val := reflect.ValueOf(response)

	// Marshal argument must be a pointer
	if val.Kind() != reflect.Ptr {
		return fmt.Errorf("%w: expected ptr, got %T", msg.ErrUnrecognizedType, response)
	}

	// Save the last request
	c.LastRequest = request

	// Respond the value of NextResponse
	switch {
	case c.NextResponse != nil:
		if err, ok := c.NextResponse.(error); ok {
			c.NextResponse = nil

			return err
		}

		val.Elem().Set(reflect.ValueOf(c.NextResponse))

		c.NextResponse = nil
	case len(c.NextResponses) > 0:
		if err, ok := c.NextResponses[0].(error); ok {
			c.NextResponses = c.NextResponses[1:]

			return err
		}

		val.Elem().Set(reflect.ValueOf(c.NextResponses[0]))

		c.NextResponses = c.NextResponses[1:]
	default:
		return errors.New("no response found")
	}

	n := copy(responseBuf, c.NextBin)
	c.NextBin = c.NextBin[n:]

	return nil
}

func (c *mockConn) Close() error {
	return nil
}

func (c *mockConn) RegisterCloseHandler(handler func() error) context.CancelFunc {
	return func() {
		// do nothing
	}
}
