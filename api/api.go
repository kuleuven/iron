package api

import (
	"context"
	"time"

	"gitea.icts.kuleuven.be/coz/iron/msg"

	"github.com/sirupsen/logrus"
)

type API struct {
	Username, Zone  string
	Connect         func(context.Context) (Conn, error) // Handler to obtain a connection to perform requests on
	Admin           bool                                // Whether to act as admin by sending the admin keyword
	DefaultResource string                              // Default resource to use when creating data objects
}

type Conn interface {
	// Request sends an API request for the given API number and expects a response.
	// Both request and response should represent a type such as in `msg/types.go`.
	// The request and response will be marshaled and unmarshaled automatically.
	// If a negative IntInfo is returned, an appropriate error will be returned.
	Request(ctx context.Context, apiNumber msg.APINumber, request, response any) error

	// RequestWithBuffers behaves as Request, with provided buffers for the request
	// and response binary data. Both requestBuf and responseBuf could be nil.
	RequestWithBuffers(ctx context.Context, apiNumber msg.APINumber, request, response any, requestBuf, responseBuf []byte) error

	// Close closes the connection or releases it back to the pool.
	Close() error
}

type ObjectType string

const (
	UserType       ObjectType = "u"
	CollectionType ObjectType = "C"
	DataObjectType ObjectType = "d"
	ResourceType   ObjectType = "R"
)

type Metadata struct {
	Name  string
	Value string
	Units string
}

type File interface {
	// Name returns the name of the file as passed to OpenDataObject or CreateDataObject.
	Name() string

	// Close closes the file.
	// If the file was reopened, Close() will block until the additional handles are closed.
	Close() error

	// Seek moves file pointer of a data object, returns offset
	Seek(offset int64, whence int) (int64, error)

	// Read reads data from the file
	Read(b []byte) (int, error)

	// Write writes data to the file
	Write(b []byte) (int, error)

	// Truncate truncates the file
	// In our implementation, the file seems to be truncated in further read/write operations
	// on this handle or on reopened handles, but the file is not truncated on the server
	// until Close() is called.
	// Truncate requires retrieving file descriptor information, and this does not support
	// the admin keyword.
	Truncate(size int64) error

	// Touch changes the modification time of the file
	// A zero value for mtime means the current time. The file is not touched on the server
	// until Close() is called.
	// Touch does not support the admin keyword.
	Touch(mtime time.Time) error

	// Reopen reopens the file using another connection.
	// When called using iron.Client, nil can be passed instead of a connection,
	// and another connection from the pool will be used and blocked until the
	// file is closed. When called using iron.Conn directly, the caller is
	// responsible for providing a valid connection.
	// Reopen takes ownership of the connection, and closes it when done.
	// A reopened file must be closed before the original handle is closed.
	Reopen(conn Conn, mode int) (File, error)
}

// WithAdmin returns a new API with the admin keyword set
func (api API) WithAdmin() *API {
	api.Admin = true

	return &api
}

// WithDefaultResource returns a new API with the default resource set
func (api API) WithDefaultResource(resource string) *API {
	api.DefaultResource = resource

	return &api
}

func (api *API) setFlags(ptr *msg.SSKeyVal) {
	if api.Admin {
		ptr.Add(msg.ADMIN_KW, "true")
	}
}

func (api *API) Request(ctx context.Context, apiNumber msg.APINumber, request, response any) error {
	return api.RequestWithBuffers(ctx, apiNumber, request, response, nil, nil)
}

func (api *API) RequestWithBuffers(ctx context.Context, apiNumber msg.APINumber, request, response any, requestBuf, responseBuf []byte) error {
	conn, err := api.Connect(ctx)
	if err != nil {
		return err
	}

	defer conn.Close()

	return conn.RequestWithBuffers(ctx, apiNumber, request, response, requestBuf, responseBuf)
}

// ElevateRequest is a wrapper around Request, that elevates permissions on the given path if the request
// fails with CAT_NO_ACCESS_PERMISSION, if the admin flag is set; for operations that ignore the admin
// keyword. If giving permissions fails with CAT_NO_ROWS_FOUND, it will try to elevate permissions
// on the parent directory.
func (api *API) ElevateRequest(ctx context.Context, apiNumber msg.APINumber, request, response any, path string) error {
	err := api.Request(ctx, apiNumber, request, response)
	if err == nil || !api.Admin {
		return err
	}

	rodsErr, ok := err.(*msg.IRODSError)
	if !ok || rodsErr.Code != -818000 { // CAT_NO_ACCESS_PERMISSION
		return err
	}

	for path != "/" {
		err1 := api.ModifyAccess(ctx, path, api.Username, "own", false)
		if err1 == nil {
			logrus.Infof("Admin keyword not supported. Elevated permissions on directory: %s", path)

			return api.Request(ctx, apiNumber, request, response)
		}

		rodsErr, ok = err1.(*msg.IRODSError)
		if !ok || rodsErr.Code != -808000 { // CAT_NO_ROWS_FOUND
			return err
		}

		path, _ = Split(path)
	}

	return err
}
