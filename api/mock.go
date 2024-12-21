package api

import (
	"context"
	"fmt"
	"reflect"

	"gitea.icts.kuleuven.be/coz/iron/msg"
)

// mock an API for testing purposes
var testAPI = New(func(context.Context) (Conn, error) {
	return testConn, nil
})

var testConn = &mockConn{}

type mockConn struct {
	NextResponse any
	LastRequest  any
}

func (c *mockConn) Request(ctx context.Context, apiNumber int32, request, response any) error {
	val := reflect.ValueOf(response)

	// Marshal argument must be a pointer
	if val.Kind() != reflect.Ptr {
		return fmt.Errorf("%w: expected ptr, got %T", msg.ErrUnrecognizedType, response)
	}

	// Respond the value of NextResponse
	val.Elem().Set(reflect.ValueOf(c.NextResponse))

	// Save the last request
	c.LastRequest = request

	return nil
}

func (c *mockConn) Close() error {
	return nil
}
