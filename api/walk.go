package api

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kuleuven/iron/msg"
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
	// FetchAccess prefetches ACL information for all records. If not given,
	// the Access() method will be empty on all records.
	FetchAccess WalkOption = 1 << iota

	// FetchMetadata prefetches metadata for all records. If not given,
	// the Metadata() method will be empty on all records.
	FetchMetadata

	// If the option LexographicalOrder is given, the order is guaranteed to be
	// lexographical, but the irods queries will be significantly more expensive,
	// unless the option NoSkip is also given.
	LexographicalOrder

	// If the option NoSkip is given, the walk function may not return SkipSubDirs
	// or SkipDir, but it may return SkipAll. This is useful in combination with
	// LexographicalOrder to speed up the irods queries.
	NoSkip

	// If the option BreadthFirst is given, a single level of collections is handled
	// first, before moving to the next level: collections at level N are handled first,
	// followed by data objects at level N+1, collections at level N+1 for which children
	// are skipped (see SkipSubDirs option) and finally collections at level N + 1.
	// Note that this option caches at level N, a list of subcollections of level N + 1,
	// this might require a significant amount of memory for large collections.
	BreadthFirst
)

var ErrSkipNotAllowed = errors.New("skip not allowed")

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
// The order in which the collections are visited is not specified in general.
// The only guarantees are that parent collections are visited before their
// children.
func (api *API) Walk(ctx context.Context, path string, walkFn WalkFunc, opts ...WalkOption) error {
	collection, err := api.GetCollection(ctx, path)

	switch {
	case err != nil:
		// See below
	case slices.Contains(opts, BreadthFirst):
		return api.walkBreadthFirst(ctx, walkFn, []Collection{*collection}, opts...)
	case slices.Contains(opts, LexographicalOrder) && slices.Contains(opts, NoSkip):
		return api.walkLexographicalNoSkip(ctx, walkFn, []Collection{*collection}, opts...)
	case slices.Contains(opts, LexographicalOrder):
		return api.walkLexographical(ctx, walkFn, *collection, opts...)
	default:
		return api.walkBatches(ctx, walkFn, []Collection{*collection}, opts...)
	}

	// If the collection does not exist, check if it is a data object
	if code, ok := ErrorCode(err); ok && code == msg.CAT_NO_ROWS_FOUND {
		if record, err := api.GetRecord(ctx, path, opts...); err == nil && !record.IsDir() {
			err = walkFn(path, record, nil)
			if err == SkipAll {
				return nil
			}

			return err
		}
	}

	return walkFn(path, nil, err)
}

const maxBatchLength = 14000

// walkBatches traverses a single level of the iRODS hierarchy, by expanding the children
// of the given list of parent collections. It splits the traversal into batches to avoid
// exceeding the maximum IN condition length. It recurses to the subcollections of each batch,
// before moving to the next batch.
func (api *API) walkBatches(ctx context.Context, fn WalkFunc, parents []Collection, opts ...WalkOption) error {
	return callBatches(api.walkBatch, ctx, fn, parents, opts...)
}

func callBatches(callback func(ctx context.Context, fn WalkFunc, parents []Collection, opts ...WalkOption) error, ctx context.Context, fn WalkFunc, parents []Collection, opts ...WalkOption) error {
	if len(parents) == 0 {
		return nil
	}

	var (
		batch []Collection
		n     int
	)

	for _, item := range parents {
		if strings.Contains(item.Path, "'") {
			// The irods IN condition used in callback cannot cope with single quotes
			// Do a batch with a single item instead, so the = condition can be used
			if err := callback(ctx, fn, []Collection{item}, opts...); err != nil {
				return err
			}

			continue
		}

		if n+len(item.Path)+4 > maxBatchLength {
			if err := callback(ctx, fn, batch, opts...); err != nil {
				return err
			}

			batch = nil
			n = 0
		}

		batch = append(batch, item)
		n += len(item.Path) + 4
	}

	if len(batch) > 0 {
		if err := callback(ctx, fn, batch, opts...); err != nil {
			return err
		}
	}

	return nil
}

func (api *API) walkBatch(ctx context.Context, fn WalkFunc, parents []Collection, opts ...WalkOption) error {
	subcols, err := api.walkCollections(ctx, fn, parents, opts...)
	if err != nil {
		return err
	}

	return api.walkBatches(ctx, fn, subcols, opts...)
}

// walkBreadthFirst traverses a single level of the iRODS hierarchy, by expanding the children
// of the given list of parent collections. It splits the traversal into batches to avoid
// exceeding the maximum IN condition length. It then recurses to the next level.
func (api *API) walkBreadthFirst(ctx context.Context, fn WalkFunc, parents []Collection, opts ...WalkOption) error {
	if len(parents) == 0 {
		return nil
	}

	var (
		batch          []Collection
		subcollections []Collection
		n              int
	)

	for _, item := range parents {
		if strings.Contains(item.Path, "'") {
			// The irods IN condition used in walkLevelBatch cannot cope with single quotes
			// Do a batch with a single item instead, so the = condition can be used
			subcols, err := api.walkCollections(ctx, fn, []Collection{item}, opts...)
			if err != nil {
				return err
			}

			subcollections = append(subcollections, subcols...)

			continue
		}

		if n+len(item.Path)+4 > maxBatchLength {
			subcols, err := api.walkCollections(ctx, fn, batch, opts...)
			if err != nil {
				return err
			}

			subcollections = append(subcollections, subcols...)

			batch = nil
			n = 0
		}

		batch = append(batch, item)
		n += len(item.Path) + 4
	}

	if len(batch) > 0 {
		subcols, err := api.walkCollections(ctx, fn, batch, opts...)
		if err != nil {
			return err
		}

		subcollections = append(subcollections, subcols...)
	}

	return api.walkBreadthFirst(ctx, fn, subcollections, opts...)
}

type result struct {
	path   string
	record Record
	err    error
}

func (api *API) walkLexographical(ctx context.Context, fn WalkFunc, parent Collection, opts ...WalkOption) error {
	queue, subcols, err := api.listCollection(ctx, fn, parent, opts...)
	if err != nil {
		return err
	}

	// Iterate over queue and subcols
	for _, item := range queue {
		for len(subcols) > 0 && subcols[0].Path < item.path {
			err = api.walkLexographical(ctx, fn, subcols[0], opts...)
			if err != nil {
				return err
			}

			subcols = subcols[1:]
		}

		err := fn(item.path, item.record, item.err)
		if err != nil && err != SkipDir {
			return err
		}
	}

	for _, subcol := range subcols {
		err = api.walkLexographical(ctx, fn, subcol, opts...)
		if err != nil {
			return err
		}
	}

	return nil
}

func (api *API) listCollection(ctx context.Context, fn WalkFunc, parent Collection, opts ...WalkOption) ([]result, []Collection, error) {
	var (
		first   = true
		results []result
	)

	subcols, err := api.walkCollections(ctx, func(path string, record Record, err error) error {
		if first {
			first = false

			return fn(path, record, err)
		}

		results = append(results, result{path, record, err})

		return nil
	}, []Collection{parent}, opts...)
	if err != nil {
		return nil, nil, err
	}

	slices.SortFunc(results, func(a, b result) int {
		return strings.Compare(a.path, b.path)
	})

	slices.SortFunc(subcols, func(a, b Collection) int {
		return strings.Compare(a.Path, b.Path)
	})

	return results, subcols, nil
}

func (api *API) walkLexographicalNoSkip(ctx context.Context, fn WalkFunc, parents []Collection, opts ...WalkOption) error {
	return callBatches(api.walkLexographicalNoSkipBatch, ctx, fn, parents, opts...)
}

func (api *API) walkLexographicalNoSkipBatch(ctx context.Context, fn WalkFunc, parents []Collection, opts ...WalkOption) error { //nolint:funlen
	type result struct {
		path   string
		record Record
		err    error
	}

	var (
		first = true
		queue []result
	)

	subcols, err := api.walkCollections(ctx, func(path string, record Record, err error) error {
		if first {
			first = false

			return fn(path, record, err)
		}

		queue = append(queue, result{path, record, err})

		return nil
	}, parents, opts...)
	if err != nil {
		return err
	}

	slices.SortFunc(queue, func(a, b result) int {
		return strings.Compare(a.path, b.path)
	})

	var skipAll bool

	err = api.walkLexographicalNoSkip(ctx, func(path string, record Record, err error) error {
		for len(queue) > 0 && queue[0].path < path {
			switch err := fn(queue[0].path, queue[0].record, queue[0].err); err {
			case SkipAll:
				skipAll = true

				return SkipAll
			case SkipDir, SkipSubDirs:
				return ErrSkipNotAllowed
			case nil:
				queue = queue[1:]
			default:
				return err
			}
		}

		return fn(path, record, err)
	}, subcols, opts...)
	if err != nil || skipAll {
		return err
	}

	for _, item := range queue {
		switch err := fn(item.path, item.record, item.err); err {
		case SkipAll:
			skipAll = true

			return nil
		case SkipDir, SkipSubDirs:
			return ErrSkipNotAllowed
		case nil:
			continue
		default:
			return err
		}
	}

	return nil
}

// walkCollections runs the walk function on each given collection, its containing
// data objects, and if SkipSubDirs was returned, its subcollections.
// The function returns a list of subcollections of the given collections for which
// SkipSubDirs was not returned. The caller is expected to run walkCollections on
// this list for further traversal.
func (api *API) walkCollections(ctx context.Context, fn WalkFunc, parents []Collection, opts ...WalkOption) ([]Collection, error) { //nolint:funlen
	ids := collectionIDs(parents)
	names := collectionPaths(parents)
	pmap := collectionIDPathMap(parents)

	// Find all subcollections
	subcollections, err := api.ListCollections(ctx, In(msg.ICAT_COLUMN_COLL_PARENT_NAME, names), NotEqual(msg.ICAT_COLUMN_COLL_NAME, "/"))
	if err != nil {
		return nil, api.handleWalkError(fn, names, err)
	}

	// Find all objects
	objects, err := api.walkListDataObjects(ctx, ids, pmap)
	if err != nil {
		return nil, api.handleWalkError(fn, names, err)
	}

	bulk := bulk{}

	// Find attributes of the parents
	if err := bulk.PrefetchCollections(ctx, api, ids, opts...); err != nil {
		return nil, api.handleWalkError(fn, names, err)
	}

	var skipcolls []Collection

	// Call walk function for parents
	for _, coll := range parents {
		err := fn(coll.Path, bulk.Record(&coll), nil)
		switch err {
		case SkipAll:
			return nil, nil
		case SkipDir:
			// Need to remove all subcollections and objects within this directory
			ids = slices.DeleteFunc(ids, func(id int64) bool {
				return id == coll.ID
			})

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
		case nil:
			continue
		default:
			return nil, err
		}
	}

	// Find attributes of the objects
	attrErr := bulk.PrefetchDataObjectsInCollections(ctx, api, ids, opts...)

	// Iterate over objects
	for _, o := range objects {
		err := fn(o.Path, bulk.Record(&o), attrErr)
		if err == filepath.SkipAll {
			return nil, nil
		} else if err != nil {
			return nil, err
		}
	}

	// Find attributes of the skipped subcollections
	attrErr = bulk.PrefetchCollections(ctx, api, collectionIDs(skipcolls), opts...)

	// Iterate over skipped subcollections
	for _, c := range skipcolls {
		err := fn(c.Path, bulk.Record(&c), attrErr)
		if err == filepath.SkipAll {
			return nil, nil
		} else if err != nil && err != SkipDir && err != SkipSubDirs {
			return nil, err
		}
	}

	// Return all unvisited subcollections
	return subcollections, nil
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

// GetRecord retrieves a Record for the given path. The Record is a combination of
// os.FileInfo and iRODS metadata. The metadata is only retrieved if the
// FetchMetadata or FetchAccess WalkOptions are given.
func (api *API) GetRecord(ctx context.Context, path string, options ...WalkOption) (Record, error) {
	var (
		fi  os.FileInfo
		err error
	)

	objectType := DataObjectType

	fi, err = api.GetDataObject(ctx, path)
	if errors.Is(err, ErrNoRowFound) {
		fi, err = api.GetCollection(ctx, path)

		objectType = CollectionType
	}

	if err != nil {
		return nil, err
	}

	r := &record{
		FileInfo: fi,
	}

	if slices.Contains(options, FetchMetadata) {
		r.metadata, err = api.ListMetadata(ctx, path, objectType)
		if err != nil {
			return nil, err
		}
	}

	if slices.Contains(options, FetchAccess) {
		r.access, err = api.ListAccess(ctx, path, objectType)
		if err != nil {
			return nil, err
		}
	}

	return r, nil
}
