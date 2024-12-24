package api

import (
	"context"
	"os"

	"gitea.icts.kuleuven.be/coz/iron/msg"
)

func (api *api) CreateCollection(ctx context.Context, name string) error {
	request := msg.CreateCollectionRequest{
		Name: name,
	}

	api.SetFlags(&request.KeyVals)

	return api.Request(ctx, msg.COLL_CREATE_AN, request, &msg.EmptyResponse{})
}

func (api *api) CreateCollectionAll(ctx context.Context, name string) error {
	request := msg.CreateCollectionRequest{
		Name: name,
	}

	request.KeyVals.Add(msg.RECURSIVE_OPR_KW, "")

	api.SetFlags(&request.KeyVals)

	return api.Request(ctx, msg.COLL_CREATE_AN, request, &msg.EmptyResponse{})
}

func (api *api) DeleteCollection(ctx context.Context, name string, force bool) error {
	request := msg.CreateCollectionRequest{
		Name: name,
	}

	if force {
		request.KeyVals.Add(msg.FORCE_FLAG_KW, "")
	}

	api.SetFlags(&request.KeyVals)

	return api.Request(ctx, msg.RM_COLL_AN, request, &msg.CollectionOperationStat{})
}

func (api *api) DeleteCollectionAll(ctx context.Context, name string, force bool) error {
	request := msg.CreateCollectionRequest{
		Name: name,
	}

	request.KeyVals.Add(msg.RECURSIVE_OPR_KW, "")

	if force {
		request.KeyVals.Add(msg.FORCE_FLAG_KW, "")
	}

	api.SetFlags(&request.KeyVals)

	return api.Request(ctx, msg.RM_COLL_AN, request, &msg.CollectionOperationStat{})
}

func (api *api) RenameCollection(ctx context.Context, oldName, newName string) error {
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

	api.SetFlags(&request.Paths[0].KeyVals)
	api.SetFlags(&request.Paths[1].KeyVals)

	return api.Request(ctx, msg.DATA_OBJ_RENAME_AN, request, &msg.EmptyResponse{})
}

func (api *api) DeleteDataObject(ctx context.Context, path string, force bool) error {
	request := msg.DataObjectRequest{
		Path: path,
	}

	if force {
		request.KeyVals.Add(msg.FORCE_FLAG_KW, "")
	}

	api.SetFlags(&request.KeyVals)

	return api.Request(ctx, msg.DATA_OBJ_UNLINK_AN, request, &msg.EmptyResponse{})
}

func (api *api) RenameDataObject(ctx context.Context, oldPath, newPath string) error {
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

	api.SetFlags(&request.Paths[0].KeyVals)
	api.SetFlags(&request.Paths[1].KeyVals)

	return api.Request(ctx, msg.DATA_OBJ_RENAME_AN, request, &msg.EmptyResponse{})
}

func (api *api) CopyDataObject(ctx context.Context, oldPath, newPath string) error {
	request := msg.DataObjectCopyRequest{
		Paths: []msg.DataObjectRequest{
			{
				Path:          oldPath,
				OperationType: msg.OPER_TYPE_COPY_DATA_OBJ_SRC,
			},
			{
				Path:          newPath,
				OperationType: msg.OPER_TYPE_COPY_DATA_OBJ_DEST,
			},
		},
	}

	api.SetFlags(&request.Paths[0].KeyVals)
	api.SetFlags(&request.Paths[1].KeyVals)

	return api.Request(ctx, msg.DATA_OBJ_COPY_AN, request, &msg.EmptyResponse{})
}

const (
	O_RDONLY = os.O_RDONLY //nolint:stylecheck
	O_WRONLY = os.O_WRONLY //nolint:stylecheck
	O_RDWR   = os.O_RDWR   //nolint:stylecheck
	O_CREAT  = os.O_CREATE //nolint:stylecheck
	O_EXCL   = os.O_EXCL   //nolint:stylecheck
	O_TRUNC  = os.O_TRUNC  //nolint:stylecheck
	O_APPEND = os.O_APPEND // Irods does not support O_APPEND, we need to seek to the end //nolint:stylecheck
)

func (api *api) CreateDataObject(ctx context.Context, path string, mode int) (File, error) {
	request := msg.DataObjectRequest{
		Path:       path,
		CreateMode: 0o644,
		OpenFlags:  (mode &^ O_APPEND) | O_CREAT,
	}

	request.KeyVals.Add(msg.DATA_TYPE_KW, "generic")

	if api.resource != "" {
		request.KeyVals.Add(msg.DEST_RESC_NAME_KW, api.resource)
	}

	api.SetFlags(&request.KeyVals)

	h := handle{
		api: api,
		ctx: ctx,
	}

	err := api.Request(ctx, msg.DATA_OBJ_CREATE_AN, request, &h.FileDescriptor)

	return &h, err
}

func (api *api) OpenDataObject(ctx context.Context, path string, mode int) (File, error) {
	request := msg.DataObjectRequest{
		Path:       path,
		CreateMode: 0o644,
		OpenFlags:  mode &^ O_APPEND,
	}

	request.KeyVals.Add(msg.DATA_TYPE_KW, "generic")

	if api.resource != "" {
		request.KeyVals.Add(msg.DEST_RESC_NAME_KW, api.resource)
	}

	api.SetFlags(&request.KeyVals)

	h := handle{
		api: api,
		ctx: ctx,
	}

	err := api.Request(ctx, msg.DATA_OBJ_OPEN_AN, request, &h.FileDescriptor)
	if err == nil && mode&O_APPEND != 0 {
		// Irods does not support O_APPEND, we need to seek to the end
		_, err = h.Seek(0, 2)
	}

	return &h, err
}
