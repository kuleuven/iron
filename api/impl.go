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

/*
func (api *api) DeleteCollection(ctx context.Context, name string) error {
	return nil
}
*/
