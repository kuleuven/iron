package api

import (
	"context"
	"os"
	"slices"

	"github.com/kuleuven/iron/msg"
)

type bulk map[int64]*Attributes

type Attributes struct {
	User     *User
	Metadata []Metadata
	Access   []struct {
		UserID     int64
		Permission string
	}
}

type pathObject interface {
	Object
	os.FileInfo
}

func (b bulk) Record(object pathObject) Record {
	return &record{
		FileInfo: object,
		metadata: b.Metadata(object.Identifier()),
		access:   b.Access(object.Identifier()),
	}
}

func (b bulk) Access(key int64) []Access {
	cached, ok := b[key]
	if !ok {
		return nil
	}

	out := make([]Access, 0, len(cached.Access))

	for _, access := range cached.Access {
		u, ok := b[access.UserID]
		if !ok || u.User == nil {
			continue
		}

		out = append(out, Access{
			User:       *u.User,
			Permission: access.Permission,
		})
	}

	return out
}

func (b bulk) Metadata(key int64) []Metadata {
	cached, ok := b[key]
	if !ok {
		return nil
	}

	return cached.Metadata
}

func (b bulk) PrefetchCollections(ctx context.Context, api *API, keys []int64, opts ...WalkOption) error {
	if len(keys) == 0 {
		return nil
	}

	if slices.Contains(opts, FetchAccess) {
		if err := b.prefetchACLForCollections(ctx, api, keys...); err != nil {
			return err
		}

		if err := b.resolveUsers(ctx, api); err != nil {
			return err
		}
	}

	if !slices.Contains(opts, FetchMetadata) {
		return nil
	}

	return b.prefetchMetadataForCollections(ctx, api, keys...)
}

// PrefetchDataObjectsInCollections fetches attributes for data objects that are in one of the given collections
func (b bulk) PrefetchDataObjectsInCollections(ctx context.Context, api *API, keys []int64, opts ...WalkOption) error {
	if len(keys) == 0 {
		return nil
	}

	if slices.Contains(opts, FetchAccess) {
		if err := b.prefetchACLForDataObjectsInCollections(ctx, api, keys...); err != nil {
			return err
		}

		if err := b.resolveUsers(ctx, api); err != nil {
			return err
		}
	}

	if !slices.Contains(opts, FetchMetadata) {
		return nil
	}

	return b.prefetchMetadataForDataObjectsInCollections(ctx, api, keys...)
}

func (b bulk) prefetchACLForCollections(ctx context.Context, api *API, keys ...int64) error {
	for _, batch := range makeBatches(keys) {
		result := api.Query(
			msg.ICAT_COLUMN_COLL_ACCESS_NAME,
			msg.ICAT_COLUMN_COLL_ACCESS_USER_ID,
			msg.ICAT_COLUMN_COLL_ID,
		).Where(
			msg.ICAT_COLUMN_COLL_TOKEN_NAMESPACE, equalAccessType,
		).With(In(
			msg.ICAT_COLUMN_COLL_ID, batch,
		)).Execute(ctx)

		if err := b.collectACLs(result); err != nil {
			return err
		}
	}

	return nil
}

func (b bulk) prefetchACLForDataObjectsInCollections(ctx context.Context, conn *API, keys ...int64) error {
	for _, batch := range makeBatches(keys) {
		result := conn.Query(
			msg.ICAT_COLUMN_DATA_ACCESS_NAME,
			msg.ICAT_COLUMN_DATA_ACCESS_USER_ID,
			msg.ICAT_COLUMN_D_DATA_ID,
		).Where(
			msg.ICAT_COLUMN_DATA_TOKEN_NAMESPACE, equalAccessType,
		).With(In(
			msg.ICAT_COLUMN_D_COLL_ID, batch,
		)).Execute(ctx)

		if err := b.collectACLs(result); err != nil {
			return err
		}
	}

	return nil
}

func (b bulk) collectACLs(result *Result) error {
	defer result.Close()

	for result.Next() {
		var (
			accessName string
			userID     int64
			objectID   int64
		)

		err := result.Scan(&accessName, &userID, &objectID)
		if err != nil {
			return err
		}

		if _, ok := b[objectID]; !ok {
			b[objectID] = &Attributes{}
		}

		b[objectID].Access = append(b[objectID].Access, struct {
			UserID     int64
			Permission string
		}{
			UserID:     userID,
			Permission: accessName,
		})
	}

	return result.Err()
}

func (b bulk) resolveUsers(ctx context.Context, api *API) error {
	var userID []int64

	for _, attrs := range b {
		for _, access := range attrs.Access {
			if v, ok := b[access.UserID]; ok && v.User != nil {
				continue
			}

			userID = append(userID, access.UserID)
		}
	}

	for _, batch := range makeBatches(userID) {
		users, err := api.ListUsers(ctx, In(msg.ICAT_COLUMN_USER_ID, batch))
		if err != nil {
			return err
		}

		for i, user := range users {
			b[user.ID] = &Attributes{
				User: &users[i],
			}
		}
	}

	return nil
}

func (b bulk) prefetchMetadataForCollections(ctx context.Context, conn *API, keys ...int64) error {
	for _, batch := range makeBatches(keys) {
		result := conn.Query(
			msg.ICAT_COLUMN_META_COLL_ATTR_NAME,
			msg.ICAT_COLUMN_META_COLL_ATTR_VALUE,
			msg.ICAT_COLUMN_META_COLL_ATTR_UNITS,
			msg.ICAT_COLUMN_COLL_ID,
		).With(In(
			msg.ICAT_COLUMN_COLL_ID, batch,
		)).Execute(ctx)

		if err := b.collectMetadata(result); err != nil {
			return err
		}
	}

	return nil
}

func (b bulk) prefetchMetadataForDataObjectsInCollections(ctx context.Context, conn *API, keys ...int64) error {
	for _, batch := range makeBatches(keys) {
		result := conn.Query(
			msg.ICAT_COLUMN_META_DATA_ATTR_NAME,
			msg.ICAT_COLUMN_META_DATA_ATTR_VALUE,
			msg.ICAT_COLUMN_META_DATA_ATTR_UNITS,
			msg.ICAT_COLUMN_D_DATA_ID,
		).With(In(
			msg.ICAT_COLUMN_D_COLL_ID, batch,
		)).Execute(ctx)

		if err := b.collectMetadata(result); err != nil {
			return err
		}
	}

	return nil
}

func (b bulk) collectMetadata(result *Result) error {
	defer result.Close()

	for result.Next() {
		var (
			meta     Metadata
			objectID int64
		)

		if err := result.Scan(&meta.Name, &meta.Value, &meta.Units, &objectID); err != nil {
			return err
		}

		if _, ok := b[objectID]; !ok {
			b[objectID] = &Attributes{}
		}

		b[objectID].Metadata = append(b[objectID].Metadata, meta)
	}

	return result.Err()
}

const batchSize = 100

func makeBatches(keys []int64) [][]int64 {
	batches := make([][]int64, 0, len(keys)/batchSize+1)

	for i := 0; i < len(keys); i += batchSize {
		batches = append(batches, keys[i:min(i+batchSize, len(keys))])
	}

	return batches
}
