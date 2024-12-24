package api

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"gitea.icts.kuleuven.be/coz/iron/msg"
)

// mock an API for testing purposes
var testAPI = New(func(context.Context) (Conn, error) {
	return testConn, nil
}, "demoResc")

var testConn = &mockConn{}

type mockConn struct {
	NextResponse  any
	NextResponses []any
	LastRequest   any
}

func (c *mockConn) Request(ctx context.Context, apiNumber msg.APINumber, request, response any) error {
	return c.RequestWithBuffers(ctx, apiNumber, request, response, nil, nil)
}

func (c *mockConn) RequestWithBuffers(ctx context.Context, apiNumber msg.APINumber, request, response any, _, _ []byte) error {
	val := reflect.ValueOf(response)

	// Marshal argument must be a pointer
	if val.Kind() != reflect.Ptr {
		return fmt.Errorf("%w: expected ptr, got %T", msg.ErrUnrecognizedType, response)
	}

	// Respond the value of NextResponse
	switch {
	case c.NextResponse != nil:
		val.Elem().Set(reflect.ValueOf(c.NextResponse))

		c.NextResponse = nil
	case len(c.NextResponses) > 0:
		val.Elem().Set(reflect.ValueOf(c.NextResponses[0]))

		c.NextResponses = c.NextResponses[1:]
	default:
		return errors.New("no response found")
	}

	// Save the last request
	c.LastRequest = request

	return nil
}

func (c *mockConn) Close() error {
	return nil
}
