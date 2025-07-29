package api

import (
	"context"
	"strings"
	"time"

	"github.com/kuleuven/iron/msg"

	"github.com/sirupsen/logrus"
)

// API provides the interface to IRODS using the provided connection function.
// Each time an API method is called, the Connect function is called to obtain
// a connection, and it is closed afterwards. If used together with an instance
// of iron.Client, the Connect will take a connection from the pool, and when
// closed it will be returned to the pool.
// The Username and Zone must match the username and zone of the connection.
// If Admin is true, the API will send the admin keyword with each request.
// The DefaultResource is the resource to use when creating data objects.
type API struct {
	Username, Zone  string
	Connect         func(context.Context) (Conn, error) // Handler to obtain a connection to perform requests on
	Admin           bool                                // Whether to act as admin by sending the admin keyword
	DefaultResource string                              // Default resource to use when creating data objects
	ReplicaNumber   *int                                // Replica number to use for open/checksum operations
	NumThreads      int                                 // Number of threads to use for server-side copies
}

// Conn is a limited interface to an iRODS connection to avoid dependency cycles.
// Use iron.Conn instead.
type Conn interface {
	// ClientSignature returns the client signature
	ClientSignature() string

	// NativePassword returns the native password
	// In case of PAM authentication, this is the generated password
	NativePassword() string

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

	// RegisterCloseHandler registers a function to be called when the connection is
	// about to closed. It is used to ensure opened files are closed.
	RegisterCloseHandler(handler func() error) context.CancelFunc
}

type ObjectType string

const (
	UserType       ObjectType = "u"
	CollectionType ObjectType = "C"
	DataObjectType ObjectType = "d"
	ResourceType   ObjectType = "R"
)

func (o ObjectType) String() string {
	switch o {
	case UserType:
		return "user"
	case CollectionType:
		return "collection"
	case ResourceType:
		return "resource"
	case DataObjectType:
		return "data_object"
	default:
		return string(o)
	}
}

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

	// Size returns the size of the file
	Size() (int64, error)

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
	// Reopen takes ownership of the connection, and closes it when done,
	// also if an error is returned.
	// A reopened file must be closed before the original handle is closed.
	Reopen(conn Conn, mode int) (File, error)
}

// AsAdmin returns a new API with the admin keyword set
func (api API) AsAdmin() *API {
	api.Admin = true

	return &api
}

// WithDefaultResource returns a new API with the default resource set
func (api API) WithDefaultResource(resource string) *API {
	api.DefaultResource = resource

	return &api
}

// WithNumThreads returns a new API with the number of threads set
func (api API) WithNumThreads(n int) *API {
	api.NumThreads = n

	return &api
}

// WithReplicaNumber returns a new API with the replica number set
func (api API) WithReplicaNumber(n int) *API {
	api.ReplicaNumber = &n

	return &api
}

func (api *API) setFlags(ptr *msg.SSKeyVal) {
	if api.Admin {
		ptr.Add(msg.ADMIN_KW, "")
	}
}

// Request is a wrapper function for using api.Connect to obtain a connection,
// use conn.Request on the connection, and Close the connection.
func (api *API) Request(ctx context.Context, apiNumber msg.APINumber, request, response any) error {
	return api.RequestWithBuffers(ctx, apiNumber, request, response, nil, nil)
}

// RequestWithBuffers is a wrapper function for using api.Connect to obtain a connection,
// use conn.RequestWithBuffers on the connection, and Close the connection.
func (api *API) RequestWithBuffers(ctx context.Context, apiNumber msg.APINumber, request, response any, requestBuf, responseBuf []byte) error {
	conn, err := api.Connect(ctx)
	if err != nil {
		return err
	}

	defer conn.Close()

	return conn.RequestWithBuffers(ctx, apiNumber, request, response, requestBuf, responseBuf)
}

// ElevateRequest is a wrapper around api.Request, that elevates permissions on the given path if the request
// fails with CAT_NO_ACCESS_PERMISSION, if the admin flag is set; for operations that ignore the admin
// keyword. If giving permissions fails with CAT_NO_ROWS_FOUND, it will try to elevate permissions
// on the parent directory.
func (api *API) ElevateRequest(ctx context.Context, apiNumber msg.APINumber, request, response any, paths ...string) error {
	conn, err := api.Connect(ctx)
	if err != nil {
		return err
	}

	defer conn.Close()

	return api.connElevateRequest(ctx, conn, apiNumber, request, response, paths...)
}

// connElevateRequest is a wrapper around conn.Request, that elevates permissions on the given path if the request
// fails with CAT_NO_ACCESS_PERMISSION, if the admin flag is set; for operations that ignore the admin
// keyword. If giving permissions fails with CAT_NO_ROWS_FOUND, it will try to elevate permissions
// on the parent directory.
func (api *API) connElevateRequest(ctx context.Context, conn Conn, apiNumber msg.APINumber, request, response any, paths ...string) error {
	err := conn.Request(ctx, apiNumber, request, response)
	if !Is(err, msg.CAT_NO_ACCESS_PERMISSION) || !api.Admin {
		return err
	}

	for _, path := range paths {
		if err1 := api.gainAccess(ctx, conn, path); err1 != nil {
			return err
		}
	}

	return conn.Request(ctx, apiNumber, request, response)
}

func (api *API) gainAccess(ctx context.Context, conn Conn, path string) error {
	request := msg.ModifyAccessRequest{
		Path:        strings.TrimSuffix(path, "/"),
		UserName:    api.Username,
		Zone:        api.Zone,
		AccessLevel: "admin:own",
	}

	if strings.HasSuffix(path, "/") {
		request.RecursiveFlag = 1
	}

	err := conn.Request(ctx, msg.MOD_ACCESS_CONTROL_AN, request, &msg.EmptyResponse{})
	if err == nil {
		logrus.Infof("Admin keyword not supported. Elevated permissions on directory %s", path)

		return nil
	}

	if !Is(err, msg.CAT_NO_ROWS_FOUND) && !Is(err, msg.INVALID_OBJECT_TYPE) {
		logrus.Warnf("Admin keyword not supported. Failed to elevate permissions on directory %s: %s", path, err)

		return err
	}

	path, _ = Split(strings.TrimSuffix(path, "/"))

	if path == "/" {
		return err
	}

	return api.gainAccess(ctx, conn, path)
}
