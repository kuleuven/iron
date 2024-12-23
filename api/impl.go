package api

import (
	"context"

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
