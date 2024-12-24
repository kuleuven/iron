package api

import (
	"context"

	"gitea.icts.kuleuven.be/coz/iron/msg"
)

type Conn interface {
	Request(ctx context.Context, apiNumber msg.APINumber, request, response any) error
	RequestWithBuffers(ctx context.Context, apiNumber msg.APINumber, request, response any, requestBuf, responseBuf []byte) error
	Close() error
}

func New(connect func(context.Context) (Conn, error), resource string) API {
	return &api{
		Connect:  connect,
		resource: resource,
	}
}

type API interface {
	Admin() API
	WithDefaultResource(resource string) API
	Query(columns ...msg.ColumnNumber) PreparedQuery
	CreateCollection(ctx context.Context, name string) error
	CreateCollectionAll(ctx context.Context, name string) error
	DeleteCollection(ctx context.Context, name string, force bool) error
	DeleteCollectionAll(ctx context.Context, name string, force bool) error
	RenameCollection(ctx context.Context, oldName, newName string) error
	DeleteDataObject(ctx context.Context, path string, force bool) error
	RenameDataObject(ctx context.Context, oldPath, newPath string) error
	CopyDataObject(ctx context.Context, oldPath, newPath string) error
	CreateDataObject(ctx context.Context, path string, mode int) (File, error)
	OpenDataObject(ctx context.Context, path string, mode int) (File, error)
}

type File interface {
	Close() error
	Seek(offset int64, whence int) (int64, error)
	Read(b []byte) (int, error)
	Write(b []byte) (int, error)
}

type api struct {
	Connect  func(context.Context) (Conn, error)
	admin    bool
	resource string
}

func (api api) Admin() API {
	api.admin = true

	return &api
}

func (api api) WithDefaultResource(resource string) API {
	api.resource = resource

	return &api
}

func (api *api) SetFlags(ptr *msg.SSKeyVal) {
	if api.admin {
		ptr.Add(msg.ADMIN_KW, "true")
	}
}

func (api *api) Request(ctx context.Context, apiNumber msg.APINumber, request, response any) error {
	return api.RequestWithBuffers(ctx, apiNumber, request, response, nil, nil)
}

func (api *api) RequestWithBuffers(ctx context.Context, apiNumber msg.APINumber, request, response any, requestBuf, responseBuf []byte) error {
	conn, err := api.Connect(ctx)
	if err != nil {
		return err
	}

	defer conn.Close()

	return conn.RequestWithBuffers(ctx, apiNumber, request, response, requestBuf, responseBuf)
}
