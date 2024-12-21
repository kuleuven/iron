package api

import (
	"context"

	"gitea.icts.kuleuven.be/coz/iron/msg"
)

type Conn interface {
	Request(ctx context.Context, apiNumber int32, request, response any) error
	Close() error
}

func New(connect func(context.Context) (Conn, error)) API {
	return &api{
		Connect: connect,
	}
}

type API interface {
	Query(columns ...msg.ColumnNumber) PreparedQuery
}
