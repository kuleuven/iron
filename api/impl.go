//nolint:staticcheck
package api

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/kuleuven/iron/msg"
	"github.com/sirupsen/logrus"
	"go.uber.org/multierr"
)

// CreateCollection creates a collection.
// If the collection already exists, an error is returned.
func (api *API) CreateCollection(ctx context.Context, name string) error {
	request := msg.CreateCollectionRequest{
		Name: name,
	}

	api.setFlags(&request.KeyVals)

	parent, _ := Split(name)

	return api.ElevateRequest(ctx, msg.COLL_CREATE_AN, request, &msg.EmptyResponse{}, parent)
}

// CreateCollectionAll creates a collection and its parents recursively.
// If the collection already exists, nothing happens.
func (api *API) CreateCollectionAll(ctx context.Context, name string) error {
	request := msg.CreateCollectionRequest{
		Name: name,
	}

	request.KeyVals.Add(msg.RECURSIVE_OPR_KW, "")

	api.setFlags(&request.KeyVals)

	return api.ElevateRequest(ctx, msg.COLL_CREATE_AN, request, &msg.EmptyResponse{}, name)
}

// DeleteCollection deletes a collection.
// If the collection is not empty, an error is returned.
// If force is true, the collection is not moved to the trash.
func (api *API) DeleteCollection(ctx context.Context, name string, skipTrash bool) error {
	request := msg.CreateCollectionRequest{
		Name: name,
	}

	if skipTrash {
		request.KeyVals.Add(msg.FORCE_FLAG_KW, "")
	}

	api.setFlags(&request.KeyVals)

	return api.Request(ctx, msg.RM_COLL_AN, request, &msg.CollectionOperationStat{})
}

// DeleteCollectionAll deletes a collection and its children recursively.
// If force is true, the collection is not moved to the trash.
func (api *API) DeleteCollectionAll(ctx context.Context, name string, skipTrash bool) error {
	request := msg.CreateCollectionRequest{
		Name: name,
	}

	request.KeyVals.Add(msg.RECURSIVE_OPR_KW, "")

	if skipTrash {
		request.KeyVals.Add(msg.FORCE_FLAG_KW, "")
	}

	api.setFlags(&request.KeyVals)

	parent, _ := Split(name)

	return api.ElevateRequest(ctx, msg.RM_COLL_AN, request, &msg.CollectionOperationStat{}, name+"/", parent)
}

// RenameCollection renames a collection.
// It will fail if the target already exists.
func (api *API) RenameCollection(ctx context.Context, oldName, newName string) error {
	request := msg.DataObjectCopyRequest{
		Paths: []msg.DataObjectRequest{
			{
				Path:          oldName,
				OperationType: msg.OPER_TYPE_RENAME_COLL,
			},
			{
				Path:          newName,
				OperationType: msg.OPER_TYPE_RENAME_COLL,
			},
		},
	}

	api.setFlags(&request.Paths[0].KeyVals)
	api.setFlags(&request.Paths[1].KeyVals)

	parentOld, _ := Split(oldName)
	parentNew, _ := Split(newName)

	return api.ElevateRequest(ctx, msg.DATA_OBJ_RENAME_AN, request, &msg.EmptyResponse{}, parentNew, parentOld, oldName)
}

// DeleteDataObject deletes a data object.
// If force is true, the data object is not moved to the trash.
func (api *API) DeleteDataObject(ctx context.Context, path string, skipTrash bool) error {
	request := msg.DataObjectRequest{
		Path: path,
	}

	if skipTrash {
		request.KeyVals.Add(msg.FORCE_FLAG_KW, "")
	}

	api.setFlags(&request.KeyVals)

	return api.ElevateRequest(ctx, msg.DATA_OBJ_UNLINK_AN, request, &msg.EmptyResponse{}, path)
}

// ReplicateDataObject replicates a data object to the specified resource.
func (api *API) ReplicateDataObject(ctx context.Context, path, resource string) error {
	request := msg.DataObjectRequest{
		Path:          path,
		OperationType: msg.OPER_TYPE_REPLICATE_DATA_OBJ,
		Threads:       api.NumThreads,
	}

	if resource != "" {
		request.KeyVals.Add(msg.DEST_RESC_NAME_KW, resource)
	}

	api.setFlags(&request.KeyVals)

	return api.ElevateRequest(ctx, msg.DATA_OBJ_REPL_AN, request, &msg.EmptyResponse{}, path)
}

// TrimDataObject removes a data object from the specified resource.
func (api *API) TrimDataObject(ctx context.Context, path, resource string) error {
	request := msg.DataObjectRequest{
		Path: path,
	}

	request.KeyVals.Add(msg.DEST_RESC_NAME_KW, resource)

	// Despite being deprecated, the irods server won't trim anything if this is not specified,
	// and only two replicas exists.
	request.KeyVals.Add(msg.COPIES_KW, "1")

	api.setFlags(&request.KeyVals)

	return api.ElevateRequest(ctx, msg.DATA_OBJ_TRIM_AN, request, &msg.EmptyResponse{}, path)
}

// TrimDataObjectReplica removes a specific replica of a data object.
func (api *API) TrimDataObjectReplica(ctx context.Context, path string, replicaNumber int) error {
	request := msg.DataObjectRequest{
		Path: path,
	}

	request.KeyVals.Add(msg.REPL_NUM_KW, strconv.Itoa(replicaNumber))

	// Despite being deprecated, the irods server won't trim anything if this is not specified,
	// and only two replicas exists.
	request.KeyVals.Add(msg.COPIES_KW, "1")

	api.setFlags(&request.KeyVals)

	return api.ElevateRequest(ctx, msg.DATA_OBJ_TRIM_AN, request, &msg.EmptyResponse{}, path)
}

// RenameDataObject renames a data object.
// It will fail if the target already exists.
func (api *API) RenameDataObject(ctx context.Context, oldPath, newPath string) error {
	request := msg.DataObjectCopyRequest{
		Paths: []msg.DataObjectRequest{
			{
				Path:          oldPath,
				OperationType: msg.OPER_TYPE_RENAME_DATA_OBJ,
			},
			{
				Path:          newPath,
				OperationType: msg.OPER_TYPE_RENAME_DATA_OBJ,
			},
		},
	}

	api.setFlags(&request.Paths[0].KeyVals)
	api.setFlags(&request.Paths[1].KeyVals)

	parentNew, _ := Split(newPath)

	return api.ElevateRequest(ctx, msg.DATA_OBJ_RENAME_AN, request, &msg.EmptyResponse{}, oldPath, parentNew)
}

// CopyDataObject copies a data object.
// A target resource can be specified with WithDefaultResource() first if needed.
// It will fail if the target already exists.
func (api *API) CopyDataObject(ctx context.Context, oldPath, newPath string) error {
	request := msg.DataObjectCopyRequest{
		Paths: []msg.DataObjectRequest{
			{
				Path:          oldPath,
				OperationType: msg.OPER_TYPE_COPY_DATA_OBJ_SRC,
				Threads:       api.NumThreads,
			},
			{
				Path:          newPath,
				OperationType: msg.OPER_TYPE_COPY_DATA_OBJ_DEST,
				Threads:       api.NumThreads,
			},
		},
	}

	api.setFlags(&request.Paths[0].KeyVals)
	api.setFlags(&request.Paths[1].KeyVals)

	// Add the default resource if needed
	if api.DefaultResource != "" {
		request.Paths[1].KeyVals.Add(msg.DEST_RESC_NAME_KW, api.DefaultResource)
	}

	parentNew, _ := Split(newPath)

	return api.ElevateRequest(ctx, msg.DATA_OBJ_COPY_AN, request, &msg.EmptyResponse{}, oldPath, parentNew)
}

const (
	O_RDONLY = os.O_RDONLY
	O_WRONLY = os.O_WRONLY
	O_RDWR   = os.O_RDWR
	O_CREAT  = os.O_CREATE
	O_EXCL   = os.O_EXCL
	O_TRUNC  = os.O_TRUNC
	O_APPEND = os.O_APPEND // Irods does not support O_APPEND, we need to seek to the end //nolint:staticcheck
)

// CreateDataObject creates a data object.
// A target resource can be specified with WithDefaultResource() first if needed.
// This method blocks an irods connection until the file has been closed.
// If the context is canceled, Seek, Read, Write, Truncate, Touch and Reopen will fail.
func (api *API) CreateDataObject(ctx context.Context, path string, mode int) (File, error) {
	request := msg.DataObjectRequest{
		Path:       path,
		CreateMode: 0o644,
		OpenFlags:  (mode &^ O_APPEND) | O_CREAT | O_EXCL,
	}

	request.KeyVals.Add(msg.DATA_TYPE_KW, "generic")

	if mode&O_EXCL == 0 {
		request.KeyVals.Add(msg.FORCE_FLAG_KW, "")
	}

	if api.DefaultResource != "" {
		request.KeyVals.Add(msg.DEST_RESC_NAME_KW, api.DefaultResource)
	}

	api.setFlags(&request.KeyVals)

	conn, err := api.Connect(ctx)
	if err != nil {
		return nil, err
	}

	h := handle{
		object: &object{
			api:          api,
			ctx:          ctx,
			path:         path,
			truncateSize: -1,
		},
		conn: conn,
	}

	err = api.connElevateRequest(ctx, conn, msg.DATA_OBJ_CREATE_AN, request, &h.fileDescriptor, path)
	if err != nil {
		err = multierr.Append(err, conn.Close())

		return nil, err
	}

	h.unregisterEmergencyCloser = conn.RegisterCloseHandler(func() error { //nolint:contextcheck
		logrus.Warnf("Emergency close of %s", path)

		return h.Close()
	})

	return &h, err
}

// OpenDataObject opens a data object.
// A target resource can be specified with WithDefaultResource() first if needed.
// A replica number can be specified with WithReplicaNumber() first if needed.
// This method blocks an irods connection until the file has been closed.
// If the context is canceled, Seek, Read, Write, Truncate, Touch and Reopen will fail.
func (api *API) OpenDataObject(ctx context.Context, path string, mode int) (File, error) {
	request := msg.DataObjectRequest{
		Path:       path,
		CreateMode: 0o644,
		OpenFlags:  mode &^ O_APPEND,
	}

	request.KeyVals.Add(msg.DATA_TYPE_KW, "generic")

	if api.DefaultResource != "" {
		request.KeyVals.Add(msg.DEST_RESC_NAME_KW, api.DefaultResource)
	}

	if api.ReplicaNumber != nil {
		request.KeyVals.Add(msg.REPL_NUM_KW, strconv.Itoa(*api.ReplicaNumber))
	}

	api.setFlags(&request.KeyVals)

	conn, err := api.Connect(ctx)
	if err != nil {
		return nil, err
	}

	h := handle{
		object: &object{
			api:          api,
			ctx:          ctx,
			path:         path,
			actualSize:   -1,
			truncateSize: -1,
		},
		conn: conn,
	}

	err = api.connElevateRequest(ctx, conn, msg.DATA_OBJ_OPEN_AN, request, &h.fileDescriptor, path)
	if err == nil && mode&O_TRUNC == 0 && mode&O_EXCL == 0 && mode&O_APPEND != 0 {
		// Irods does not support O_APPEND, we need to seek to the end
		_, err = h.Seek(0, 2)
	}

	if mode&O_TRUNC != 0 || mode&O_EXCL != 0 {
		// File is truncated or created, so the actual size is 0
		h.object.actualSize = 0
	}

	if err != nil {
		err = multierr.Append(err, conn.Close())

		return nil, err
	}

	h.unregisterEmergencyCloser = conn.RegisterCloseHandler(func() error { //nolint:contextcheck
		logrus.Warnf("Emergency close of %s", path)

		return h.Close()
	})

	return &h, err
}

const shaPrefix = "sha2:"

var ErrChecksumNotFound = errors.New("checksum not found")

// Checksum returns the sha256 checksum of a data object as stored in the catalog.
// If no checksum is present, it is calculated on the fly.
// A target resource can be specified with WithDefaultResource() first if needed.
// A replica number can be specified with WithReplicaNumber() first if needed.
// The force flag is used to recompute any saved checksums.
func (api *API) Checksum(ctx context.Context, path string, force bool) ([]byte, error) {
	request := msg.DataObjectRequest{
		Path: path,
	}

	if api.DefaultResource != "" {
		request.KeyVals.Add(msg.DEST_RESC_NAME_KW, api.DefaultResource)
	}

	if api.ReplicaNumber != nil {
		request.KeyVals.Add(msg.REPL_NUM_KW, strconv.Itoa(*api.ReplicaNumber))
	}

	if force {
		request.KeyVals.Add(msg.FORCE_CHKSUM_KW, "")
	}

	api.setFlags(&request.KeyVals)

	conn, err := api.Connect(ctx)
	if err != nil {
		return nil, err
	}

	var checksum msg.Checksum

	err = api.connElevateRequest(ctx, conn, msg.DATA_OBJ_CHKSUM_AN, request, &checksum, path)
	if err != nil {
		return nil, err
	}

	if !strings.HasPrefix(checksum.Checksum, shaPrefix) {
		return nil, fmt.Errorf("%w: prefix %s missing", ErrChecksumNotFound, shaPrefix)
	}

	return base64.StdEncoding.DecodeString(strings.TrimPrefix(checksum.Checksum, shaPrefix))
}

// ModifyAccess modifies the access level of a data object or collection.
// For users of federated zones, specify <name>#<zone> as user.
func (api *API) ModifyAccess(ctx context.Context, path, user, accessLevel string, recursive bool) error {
	if api.Admin {
		accessLevel = fmt.Sprintf("admin:%s", accessLevel)
	}

	var zone string

	if parts := strings.SplitN(user, "#", 2); len(parts) == 2 {
		user = parts[0]
		zone = parts[1]
	}

	request := msg.ModifyAccessRequest{
		Path:        path,
		UserName:    user,
		Zone:        zone,
		AccessLevel: accessLevel,
	}

	if recursive {
		request.RecursiveFlag = 1
	}

	return api.Request(ctx, msg.MOD_ACCESS_CONTROL_AN, request, &msg.EmptyResponse{})
}

// SetCollectionInheritance sets the inheritance of a collection.
func (api *API) SetCollectionInheritance(ctx context.Context, path string, inherit, recursive bool) error {
	inheritStr := "inherit"

	if !inherit {
		inheritStr = "noinherit"
	}

	return api.ModifyAccess(ctx, path, "", inheritStr, recursive)
}

// AddMetadata adds a single metadata value of a data object, collection, user or resource.
func (api *API) AddMetadata(ctx context.Context, name string, itemType ObjectType, value Metadata) error {
	request := &msg.ModifyMetadataRequest{
		Operation: "add",
		ItemType:  fmt.Sprintf("-%s", string(itemType)),
		ItemName:  name,
		AttrName:  value.Name,
		AttrValue: value.Value,
		AttrUnits: value.Units,
	}

	api.setFlags(&request.KeyVals)

	return api.Request(ctx, msg.MOD_AVU_METADATA_AN, request, &msg.EmptyResponse{})
}

// RemoveMetadata removes a single metadata value of a data object, collection, user or resource.
func (api *API) RemoveMetadata(ctx context.Context, name string, itemType ObjectType, value Metadata) error {
	request := &msg.ModifyMetadataRequest{
		Operation: "rm",
		ItemType:  fmt.Sprintf("-%s", string(itemType)),
		ItemName:  name,
		AttrName:  value.Name,
		AttrValue: value.Value,
		AttrUnits: value.Units,
	}

	api.setFlags(&request.KeyVals)

	return api.Request(ctx, msg.MOD_AVU_METADATA_AN, request, &msg.EmptyResponse{})
}

// SetMetadata add a single metadata value for the given key and removes old metadata values with the same key.
func (api *API) SetMetadata(ctx context.Context, name string, itemType ObjectType, value Metadata) error {
	request := &msg.ModifyMetadataRequest{
		Operation: "set",
		ItemType:  fmt.Sprintf("-%s", string(itemType)),
		ItemName:  name,
		AttrName:  value.Name,
		AttrValue: value.Value,
		AttrUnits: value.Units,
	}

	api.setFlags(&request.KeyVals)

	return api.Request(ctx, msg.MOD_AVU_METADATA_AN, request, &msg.EmptyResponse{})
}

// ModifyMetadata does a bulk update of metadata, removing and adding the given values.
func (api *API) ModifyMetadata(ctx context.Context, name string, itemType ObjectType, add, remove []Metadata) error {
	request := &msg.AtomicMetadataRequest{
		AdminMode: api.Admin,
		ItemName:  name,
		ItemType:  itemType.String(),
	}

	for _, value := range remove {
		request.Operations = append(request.Operations, msg.MetadataOperation{
			Operation: "remove",
			Name:      value.Name,
			Value:     value.Value,
			Units:     value.Units,
		})
	}

	for _, value := range add {
		request.Operations = append(request.Operations, msg.MetadataOperation{
			Operation: "add",
			Name:      value.Name,
			Value:     value.Value,
			Units:     value.Units,
		})
	}

	if len(request.Operations) == 0 {
		return nil
	}

	return api.Request(ctx, msg.ATOMIC_APPLY_METADATA_OPERATIONS_APN, request, &msg.EmptyResponse{})
}
