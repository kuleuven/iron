package api

import (
	"context"
	"time"

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

	// Query prepares a query to read from the irods catalog.
	Query(columns ...msg.ColumnNumber) PreparedQuery

	// QueryRow prepares a query to read a single row from the irods catalog.
	QueryRow(columns ...msg.ColumnNumber) PreparedSingleRowQuery

	// CreateCollection creates a collection.
	// If the collection already exists, an error is returned.
	CreateCollection(ctx context.Context, name string) error

	// CreateCollectionAll creates a collection and its parents recursively.
	// If the collection already exists, nothing happens.
	CreateCollectionAll(ctx context.Context, name string) error

	// DeleteCollection deletes a collection.
	// If the collection is not empty, an error is returned.
	// If force is true, the collection is not moved to the trash.
	DeleteCollection(ctx context.Context, name string, force bool) error

	// DeleteCollectionAll deletes a collection and its children recursively.
	// If force is true, the collection is not moved to the trash.
	DeleteCollectionAll(ctx context.Context, name string, force bool) error

	// RenameCollection renames a collection.
	RenameCollection(ctx context.Context, oldName, newName string) error

	// DeleteDataObject deletes a data object.
	// If force is true, the data object is not moved to the trash.
	DeleteDataObject(ctx context.Context, path string, force bool) error

	// RenameDataObject renames a data object.
	RenameDataObject(ctx context.Context, oldPath, newPath string) error

	// CopyDataObject copies a data object.
	// A target resource can be specified with WithDefaultResource() first if needed.
	CopyDataObject(ctx context.Context, oldPath, newPath string) error

	// CreateDataObject creates a data object.
	// A target resource can be specified with WithDefaultResource() first if needed.
	// When called using iron.Client, this method blocks an irods connection
	// until the file has been closed.
	CreateDataObject(ctx context.Context, path string, mode int) (File, error)

	// OpenDataObject opens a data object.
	// A target resource can be specified with WithDefaultResource() first if needed.
	// When called using iron.Client, this method blocks an irods connection
	// until the file has been closed.
	OpenDataObject(ctx context.Context, path string, mode int) (File, error)

	// ModifyAccess modifies the access level of a data object or collection.
	// For users of federated zones, specify <name>#<zone> as user.
	ModifyAccess(ctx context.Context, path string, user string, accessLevel string, recursive bool) error

	// SetCollectionInheritance sets the inheritance of a collection.
	SetCollectionInheritance(ctx context.Context, path string, inherit bool, recursive bool) error

	// AddMetadata adds a single metadata value of a data object, collection, user or resource.
	AddMetadata(ctx context.Context, path string, objectType ObjectType, metadata Metadata) error

	// RemoveMetadata removes a single metadata value of a data object, collection, user or resource.
	RemoveMetadata(ctx context.Context, path string, objectType ObjectType, metadata Metadata) error

	// SetMetadata add a single metadata value for the given key and removes old metadata values with the same key.
	SetMetadata(ctx context.Context, path string, objectType ObjectType, metadata Metadata) error

	// ModifyMetadata does a bulk update of metadata, removing and adding the given values.
	ModifyMetadata(ctx context.Context, path string, objectType ObjectType, add, remove []Metadata) error
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
