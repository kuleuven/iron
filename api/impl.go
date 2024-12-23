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

	return api.Request(ctx, msg.COLL_CREATE_AN, request, &msg.CreateCollectionResponse{})
}

func (api *api) CreateCollectionAll(ctx context.Context, name string) error {
	request := msg.CreateCollectionRequest{
		Name: name,
	}

	request.KeyVals.Add(msg.RECURSIVE_OPR_KW, "")

	api.SetFlags(&request.KeyVals)

	return api.Request(ctx, msg.COLL_CREATE_AN, request, &msg.CreateCollectionResponse{})
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
