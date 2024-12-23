package api

import (
	"context"

	"gitea.icts.kuleuven.be/coz/iron/msg"
)

type Conn interface {
	Request(ctx context.Context, apiNumber msg.APINumber, request, response any) error
	Close() error
}

func New(connect func(context.Context) (Conn, error)) API {
	return &api{
		Connect: connect,
	}
}

type API interface {
	Admin() API
	Query(columns ...msg.ColumnNumber) PreparedQuery
	CreateCollection(ctx context.Context, name string) error
	CreateCollectionAll(ctx context.Context, name string) error
	DeleteCollection(ctx context.Context, name string, force bool) error
	DeleteCollectionAll(ctx context.Context, name string, force bool) error
	RenameCollection(ctx context.Context, oldName, newName string) error
}

type api struct {
	Connect func(context.Context) (Conn, error)
	admin   bool
}

func (api api) Admin() API {
	api.admin = true

	return &api
}

func (api *api) SetFlags(ptr *msg.SSKeyVal) {
	if api.admin {
		ptr.Add(msg.ADMIN_KW, "true")
	}
}

func (api *api) Request(ctx context.Context, apiNumber msg.APINumber, request, response any) error {
	conn, err := api.Connect(ctx)
	if err != nil {
		return err
	}

	defer conn.Close()

	return conn.Request(ctx, apiNumber, request, response)
}
