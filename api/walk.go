package api

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gitea.icts.kuleuven.be/coz/iron/msg"
)

type Record interface {
	os.FileInfo
	Metadata() []Metadata
	Access() []Access
}

type record struct {
	os.FileInfo
	metadata []Metadata
	access   []Access
}

func (r *record) Metadata() []Metadata {
	return r.metadata
}

func (r *record) Access() []Access {
	return r.access
}

type WalkFunc func(path string, record Record, err error) error

var (
	SkipAll     = filepath.SkipAll                                                //nolint:errname
	SkipDir     = filepath.SkipDir                                                //nolint:errname
	SkipSubDirs = errors.New("skip children of subdirectories of this directory") //nolint:staticcheck,errname
)

type WalkOption int

const (
	FetchAccess WalkOption = 1 << iota
	FetchMetadata
)

// Walk traverses the iRODS hierarchy rooted at the given path, calling the
// given function for each encountered file or directory. The function is
// called with the path relative to the root of the traversal, the
// corresponding Record, and any error encountered while walking. If the
// function returns an error or SkipAll, the traversal is stopped. If the
// function returns SkipDir for a collection, the children of the collection
// are not visited. If the function returns SkipSubDirs for a collection, the
// children of subcollections are not visited, but the subcollections are
// visited as apparent being empty. This avoids querying all children of the
// subcollections; otherwise the walk function on a collection is called after
// retrieving all children in memory.
// The order in which the collections are visited is not specified. The only
// guarantees are that parent collections are visited before their children;
// and that data objects within a collection are visited in lexographical
// order.
func (api *API) Walk(ctx context.Context, path string, walkFn WalkFunc, opts ...WalkOption) error {
	collection, err := api.GetCollection(ctx, path)
	if err != nil {
		return walkFn(path, nil, err)
	}

	return api.walkLevel(ctx, walkFn, []Collection{*collection}, opts...)
}

const maxBatchLength = 14000

// walkLevel traverses a single level of the iRODS hierarchy, by expanding the children
// of the given list of parent collections. It splits the traversal into batches to avoid
// exceeding the maximum IN condition length.
func (api *API) walkLevel(ctx context.Context, fn WalkFunc, parents []Collection, opts ...WalkOption) error {
	if len(parents) == 0 {
		return nil
	}

	var (
		batch []Collection
		n     int
	)

	for _, item := range parents {
		if strings.Contains(item.Path, "'") {
			// The irods IN condition used in walkLevelBatch cannot cope with single quotes
			// Do a batch with a single item instead, so the = condition can be used
			if err := api.walkLevelBatch(ctx, fn, []Collection{item}); err != nil {
				return err
			}

			continue
		}

		if n+len(item.Path)+4 > maxBatchLength {
			if err := api.walkLevelBatch(ctx, fn, batch, opts...); err != nil {
				return err
			}

			batch = nil
			n = 0
		}

		batch = append(batch, item)
		n += len(item.Path) + 4
	}

	if len(batch) > 0 {
		return api.walkLevelBatch(ctx, fn, batch, opts...)
	}

	return nil
}

// walkLevelBatch traverses a single batch level of the iRODS hierarchy in batches.
func (api *API) walkLevelBatch(ctx context.Context, fn WalkFunc, parents []Collection, opts ...WalkOption) error { //nolint:funlen
	ids := collectionIDs(parents)
	names := collectionPaths(parents)
	pmap := collectionIDPathMap(parents)

	// Find all subcollections
	subcollections, err := api.ListCollections(ctx, In(msg.ICAT_COLUMN_COLL_PARENT_NAME, names), NotEqual(msg.ICAT_COLUMN_COLL_NAME, "/"))
	if err != nil {
		return api.handleWalkError(fn, names, err)
	}

	// Find all objects
	objects, err := api.walkListDataObjects(ctx, ids, pmap)
	if err != nil {
		return api.handleWalkError(fn, names, err)
	}

	bulk := bulk{}

	// Find attributes of the parents
	if err := bulk.PrefetchCollections(ctx, api, ids, opts...); err != nil {
		return api.handleWalkError(fn, names, err)
	}

	var skipcolls []Collection

	// Call walk function for parents
	for _, coll := range parents {
		err := fn(coll.Path, bulk.Record(&coll), nil)
		switch err {
		case SkipAll:
			return nil
		case SkipDir:
			// Need to remove all subcollections and objects within this directory
			subcollections = slices.DeleteFunc(subcollections, func(c Collection) bool {
				return strings.HasPrefix(c.Path, skipPrefix(coll.Path))
			})

			objects = slices.DeleteFunc(objects, func(o DataObject) bool {
				return strings.HasPrefix(o.Path, skipPrefix(coll.Path))
			})
		case SkipSubDirs:
			// Subcollections within this directory should be added as objects
			skipcolls = append(skipcolls, slices.DeleteFunc(slices.Clone(subcollections), func(c Collection) bool {
				return !strings.HasPrefix(c.Path, skipPrefix(coll.Path))
			})...)

			subcollections = slices.DeleteFunc(subcollections, func(c Collection) bool {
				return strings.HasPrefix(c.Path, skipPrefix(coll.Path))
			})

			continue
		case nil:
			continue
		default:
			return err
		}
	}

	// Find attributes of the objects
	attrErr := bulk.PrefetchDataObjectsInCollections(ctx, api, ids, opts...)

	// Iterate over objects
	for _, o := range objects {
		err := fn(o.Path, bulk.Record(&o), attrErr)
		if err == filepath.SkipAll {
			return nil
		} else if err != nil {
			return err
		}
	}

	// Find attributes of the skipped subcollections
	attrErr = bulk.PrefetchCollections(ctx, api, collectionIDs(skipcolls), opts...)

	// Iterate over skipped subcollections
	for _, c := range skipcolls {
		err := fn(c.Path, bulk.Record(&c), attrErr)
		if err == filepath.SkipAll {
			return nil
		} else if err != nil && err != SkipDir && err != SkipSubDirs {
			return err
		}
	}

	// Iterate over subcollections
	return api.walkLevel(ctx, fn, subcollections, opts...)
}

func skipPrefix(colPath string) string {
	if colPath == "/" {
		return "/"
	}

	return colPath + "/"
}

// walkListDataObjects is an optimized version of ListDataObjects that avoids the extra join with R_COLL_MAIN
func (api *API) walkListDataObjects(ctx context.Context, ids []int64, pmap map[int64]string) ([]DataObject, error) { //nolint:funlen
	result := []DataObject{}
	mapping := map[int64]*DataObject{}
	results := api.Query(
		msg.ICAT_COLUMN_D_DATA_ID,
		msg.ICAT_COLUMN_DATA_NAME,
		msg.ICAT_COLUMN_D_COLL_ID,
		msg.ICAT_COLUMN_DATA_TYPE_NAME,
		msg.ICAT_COLUMN_DATA_REPL_NUM,
		msg.ICAT_COLUMN_DATA_SIZE,
		msg.ICAT_COLUMN_D_OWNER_NAME,
		msg.ICAT_COLUMN_D_OWNER_ZONE,
		msg.ICAT_COLUMN_D_DATA_CHECKSUM,
		msg.ICAT_COLUMN_D_REPL_STATUS,
		msg.ICAT_COLUMN_D_RESC_NAME,
		msg.ICAT_COLUMN_D_DATA_PATH,
		msg.ICAT_COLUMN_D_RESC_HIER,
		msg.ICAT_COLUMN_D_CREATE_TIME,
		msg.ICAT_COLUMN_D_MODIFY_TIME,
	).With(In(msg.ICAT_COLUMN_D_COLL_ID, ids)).Execute(ctx)

	defer results.Close()

	for results.Next() {
		var (
			object  DataObject
			replica Replica
			name    string
		)

		err := results.Scan(
			&object.ID,
			&name,
			&object.CollectionID,
			&object.DataType,
			&replica.Number,
			&replica.Size,
			&replica.Owner,
			&replica.OwnerZone,
			&replica.Checksum,
			&replica.Status,
			&replica.ResourceName,
			&replica.PhysicalPath,
			&replica.ResourceHierarchy,
			&replica.CreatedAt,
			&replica.ModifiedAt,
		)
		if err != nil {
			return nil, err
		}

		coll := pmap[object.CollectionID]

		object.Path = coll + "/" + name

		if prev, ok := mapping[object.ID]; ok {
			prev.Replicas = append(prev.Replicas, replica)

			continue
		}

		object.Replicas = append(object.Replicas, replica)
		result = append(result, object)
		mapping[object.ID] = &result[len(result)-1]
	}

	return result, results.Err()
}

func (api *API) handleWalkError(fn WalkFunc, names []string, err error) error {
	for _, name := range names {
		if err1 := fn(name, nil, err); err1 != nil {
			return err1
		}
	}

	return nil
}

func collectionIDs(collections []Collection) []int64 {
	ids := make([]int64, len(collections))

	for i, p := range collections {
		ids[i] = p.ID
	}

	return ids
}

func collectionPaths(collections []Collection) []string {
	paths := make([]string, len(collections))

	for i, p := range collections {
		paths[i] = p.Path
	}

	return paths
}

func collectionIDPathMap(collections []Collection) map[int64]string {
	paths := map[int64]string{}

	for _, p := range collections {
		paths[p.ID] = p.Path
	}

	return paths
}
