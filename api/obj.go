package api

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"gitea.icts.kuleuven.be/coz/iron/msg"
)

var _ os.FileInfo = &Collection{}

type Collection struct {
	ID         int64
	Path       string // Path has an absolute path to the collection
	Owner      string
	CreatedAt  time.Time
	ModifiedAt time.Time
}

func (c *Collection) Identifier() int64 {
	return c.ID
}

func (c *Collection) ObjectType() ObjectType {
	return CollectionType
}

func (c *Collection) IsDir() bool {
	return true
}

func (c *Collection) ModTime() time.Time {
	return c.ModifiedAt
}

func (c *Collection) Mode() os.FileMode {
	return os.FileMode(0o750) | os.ModeDir
}

func (c *Collection) Name() string {
	_, name := Split(c.Path)

	return name
}

func (c *Collection) Size() int64 {
	return 0
}

func (c *Collection) Sys() any {
	return c
}

var _ os.FileInfo = &DataObject{}

type DataObject struct {
	ID           int64
	CollectionID int64
	Path         string
	DataType     string
	Replicas     []Replica
}

type Replica struct {
	Number            int
	Owner             string
	Checksum          string
	Status            string
	Size              int64
	ResourceName      string
	PhysicalPath      string
	ResourceHierarchy string
	CreatedAt         time.Time
	ModifiedAt        time.Time
}

func (d *DataObject) Identifier() int64 {
	return d.ID
}

func (d *DataObject) ObjectType() ObjectType {
	return DataObjectType
}

func (d *DataObject) IsDir() bool {
	return false
}

func (d *DataObject) ModTime() time.Time {
	var t time.Time

	for _, replica := range d.Replicas {
		if t.Before(replica.ModifiedAt) {
			t = replica.ModifiedAt
		}
	}

	return t
}

func (d *DataObject) Mode() os.FileMode {
	return os.FileMode(0o640)
}

func (d *DataObject) Name() string {
	_, name := Split(d.Path)

	return name
}

func (d *DataObject) Size() int64 {
	var size int64

	for _, replica := range d.Replicas {
		if size < replica.Size {
			size = replica.Size
		}
	}

	return size
}

func (d *DataObject) Sys() any {
	return d
}

type Resource struct {
	ID         int64
	Name       string
	Zone       string
	Type       string
	Class      string
	Location   string
	Path       string
	Context    string
	CreatedAt  time.Time
	ModifiedAt time.Time
}

func (r *Resource) Identifier() int64 {
	return r.ID
}

func (r *Resource) ObjectType() ObjectType {
	return ResourceType
}

type User struct {
	ID         int64
	Name       string
	Zone       string
	Type       string
	CreatedAt  time.Time
	ModifiedAt time.Time
}

func (u *User) Identifier() int64 {
	return u.ID
}

func (u *User) ObjectType() ObjectType {
	return UserType
}

const equalTo = "= '%s'"

const equalToInt = "= '%d'"

// GetCollection returns a collection for the path
// Use Query for more complex queries
func (api *API) GetCollection(ctx context.Context, path string) (*Collection, error) {
	c := Collection{
		Path: path,
	}

	err := api.QueryRow(
		msg.ICAT_COLUMN_COLL_ID,
		msg.ICAT_COLUMN_COLL_OWNER_NAME,
		msg.ICAT_COLUMN_COLL_CREATE_TIME,
		msg.ICAT_COLUMN_COLL_MODIFY_TIME,
	).Where(
		msg.ICAT_COLUMN_COLL_NAME,
		fmt.Sprintf(equalTo, path),
	).Execute(ctx).Scan(
		&c.ID,
		&c.Owner,
		&c.CreatedAt,
		&c.ModifiedAt,
	)
	if err != nil {
		return nil, err
	}

	return &c, nil
}

// GetCollectionID returns a collection for the given id
// Use Query for more complex queries
func (api *API) GetCollectionID(ctx context.Context, id int64) (*Collection, error) {
	c := Collection{
		ID: id,
	}

	err := api.QueryRow(
		msg.ICAT_COLUMN_COLL_NAME,
		msg.ICAT_COLUMN_COLL_OWNER_NAME,
		msg.ICAT_COLUMN_COLL_CREATE_TIME,
		msg.ICAT_COLUMN_COLL_MODIFY_TIME,
	).Where(
		msg.ICAT_COLUMN_COLL_ID,
		fmt.Sprintf(equalToInt, id),
	).Execute(ctx).Scan(
		&c.Path,
		&c.Owner,
		&c.CreatedAt,
		&c.ModifiedAt,
	)
	if err != nil {
		return nil, err
	}

	return &c, nil
}

// GetDataObject returns a data object for the path
// Use Query for more complex queries
func (api *API) GetDataObject(ctx context.Context, path string) (*DataObject, error) {
	d := DataObject{
		Path: path,
	}

	coll, name := Split(path)

	results := api.Query(
		msg.ICAT_COLUMN_D_DATA_ID,
		msg.ICAT_COLUMN_COLL_ID,
		msg.ICAT_COLUMN_DATA_TYPE_NAME,
		msg.ICAT_COLUMN_DATA_REPL_NUM,
		msg.ICAT_COLUMN_DATA_SIZE,
		msg.ICAT_COLUMN_D_OWNER_NAME,
		msg.ICAT_COLUMN_D_DATA_CHECKSUM,
		msg.ICAT_COLUMN_D_REPL_STATUS,
		msg.ICAT_COLUMN_D_RESC_NAME,
		msg.ICAT_COLUMN_D_DATA_PATH,
		msg.ICAT_COLUMN_D_RESC_HIER,
		msg.ICAT_COLUMN_D_CREATE_TIME,
		msg.ICAT_COLUMN_D_MODIFY_TIME,
	).Where(
		msg.ICAT_COLUMN_COLL_NAME,
		fmt.Sprintf(equalTo, coll),
	).Where(
		msg.ICAT_COLUMN_DATA_NAME,
		fmt.Sprintf(equalTo, name),
	).Execute(ctx)

	defer results.Close()

	for results.Next() {
		replica := Replica{}

		err := results.Scan(
			&d.ID,
			&d.CollectionID,
			&d.DataType,
			&replica.Number,
			&replica.Size,
			&replica.Owner,
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

		d.Replicas = append(d.Replicas, replica)
	}

	if err := results.Err(); err != nil {
		return nil, err
	}

	return &d, nil
}

// GetDataObjectID returns a data object for the given id
// Use Query for more complex queries
func (api *API) GetDataObjectID(ctx context.Context, id int64) (*DataObject, error) {
	d := DataObject{ID: id}

	var coll, name string

	results := api.Query(
		msg.ICAT_COLUMN_COLL_NAME,
		msg.ICAT_COLUMN_DATA_NAME,
		msg.ICAT_COLUMN_COLL_ID,
		msg.ICAT_COLUMN_DATA_TYPE_NAME,
		msg.ICAT_COLUMN_DATA_REPL_NUM,
		msg.ICAT_COLUMN_DATA_SIZE,
		msg.ICAT_COLUMN_D_OWNER_NAME,
		msg.ICAT_COLUMN_D_DATA_CHECKSUM,
		msg.ICAT_COLUMN_D_REPL_STATUS,
		msg.ICAT_COLUMN_D_RESC_NAME,
		msg.ICAT_COLUMN_D_DATA_PATH,
		msg.ICAT_COLUMN_D_RESC_HIER,
		msg.ICAT_COLUMN_D_CREATE_TIME,
		msg.ICAT_COLUMN_D_MODIFY_TIME,
	).Where(
		msg.ICAT_COLUMN_D_DATA_ID,
		fmt.Sprintf(equalToInt, id),
	).Execute(ctx)

	defer results.Close()

	for results.Next() {
		replica := Replica{}

		err := results.Scan(
			&coll,
			&name,
			&d.CollectionID,
			&d.DataType,
			&replica.Number,
			&replica.Size,
			&replica.Owner,
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

		d.Replicas = append(d.Replicas, replica)
	}

	if err := results.Err(); err != nil {
		return nil, err
	}

	d.Path = coll + "/" + name

	return &d, nil
}

// Split splits the path into dir and file
func Split(path string) (string, string) {
	for i := len(path) - 1; i > 0; i-- {
		if path[i] == '/' {
			return path[:i], path[i+1:]
		}
	}

	if path[0] == '/' {
		return "/", path[1:]
	}

	return "", path
}

// GetResource returns information about a resource, identified by its name
// Use Query for more complex queries
func (api *API) GetResource(ctx context.Context, name string) (*Resource, error) {
	return api.getRescource(ctx, msg.ICAT_COLUMN_R_RESC_NAME, fmt.Sprintf(equalTo, name))
}

// GetResourceID returns information about a resource, identified by its id
// Use Query for more complex queries
func (api *API) GetResourceID(ctx context.Context, id int64) (*Resource, error) {
	return api.getRescource(ctx, msg.ICAT_COLUMN_R_RESC_ID, fmt.Sprintf(equalToInt, id))
}

func (api *API) getRescource(ctx context.Context, column msg.ColumnNumber, value string) (*Resource, error) {
	var r Resource

	err := api.QueryRow(
		msg.ICAT_COLUMN_R_RESC_ID,
		msg.ICAT_COLUMN_R_RESC_NAME,
		msg.ICAT_COLUMN_R_ZONE_NAME,
		msg.ICAT_COLUMN_R_TYPE_NAME,
		msg.ICAT_COLUMN_R_CLASS_NAME,
		msg.ICAT_COLUMN_R_LOC,
		msg.ICAT_COLUMN_R_VAULT_PATH,
		msg.ICAT_COLUMN_R_RESC_CONTEXT,
		msg.ICAT_COLUMN_R_CREATE_TIME,
		msg.ICAT_COLUMN_R_MODIFY_TIME,
	).Where(
		column,
		value,
	).Execute(ctx).Scan(
		&r.ID,
		&r.Name,
		&r.Zone,
		&r.Type,
		&r.Class,
		&r.Location,
		&r.Path,
		&r.Context,
		&r.CreatedAt,
		&r.ModifiedAt,
	)
	if err != nil {
		return nil, err
	}

	return &r, nil
}

// GetUser returns information about a user, identified by its name
// Use Query for more complex queries
func (api *API) GetUser(ctx context.Context, name string) (*User, error) {
	var u User

	zone := api.Zone

	if parts := strings.Split(name, "#"); len(parts) == 2 {
		name = parts[0]
		zone = parts[1]
	}

	err := api.QueryRow(
		msg.ICAT_COLUMN_USER_ID,
		msg.ICAT_COLUMN_USER_NAME,
		msg.ICAT_COLUMN_USER_ZONE,
		msg.ICAT_COLUMN_USER_TYPE,
		msg.ICAT_COLUMN_USER_CREATE_TIME,
		msg.ICAT_COLUMN_USER_MODIFY_TIME,
	).Where(
		msg.ICAT_COLUMN_USER_NAME,
		fmt.Sprintf(equalTo, name),
	).Where(
		msg.ICAT_COLUMN_USER_ZONE,
		fmt.Sprintf(equalTo, zone),
	).Execute(ctx).Scan(
		&u.ID,
		&u.Name,
		&u.Zone,
		&u.Type,
		&u.CreatedAt,
		&u.ModifiedAt,
	)
	if err != nil {
		return nil, err
	}

	return &u, nil
}

// GetUserID returns information about a user, identified by its id
// Use Query for more complex queries
func (api *API) GetUserID(ctx context.Context, id int64) (*User, error) {
	var u User

	err := api.QueryRow(
		msg.ICAT_COLUMN_USER_ID,
		msg.ICAT_COLUMN_USER_NAME,
		msg.ICAT_COLUMN_USER_ZONE,
		msg.ICAT_COLUMN_USER_TYPE,
		msg.ICAT_COLUMN_USER_CREATE_TIME,
		msg.ICAT_COLUMN_USER_MODIFY_TIME,
	).Where(
		msg.ICAT_COLUMN_USER_ID,
		fmt.Sprintf(equalToInt, id),
	).Execute(ctx).Scan(
		&u.ID,
		&u.Name,
		&u.Zone,
		&u.Type,
		&u.CreatedAt,
		&u.ModifiedAt,
	)
	if err != nil {
		return nil, err
	}

	return &u, nil
}

// ListDataObjects returns a list of data objects for the path
// Use Query for more complex queries
func (api *API) ListDataObjects(ctx context.Context, collectionPath string) ([]DataObject, error) { //nolint:funlen
	result := []DataObject{}
	mapping := map[int64]*DataObject{}
	results := api.Query(
		msg.ICAT_COLUMN_D_DATA_ID,
		msg.ICAT_COLUMN_DATA_NAME,
		msg.ICAT_COLUMN_COLL_ID,
		msg.ICAT_COLUMN_DATA_TYPE_NAME,
		msg.ICAT_COLUMN_DATA_REPL_NUM,
		msg.ICAT_COLUMN_DATA_SIZE,
		msg.ICAT_COLUMN_D_OWNER_NAME,
		msg.ICAT_COLUMN_D_DATA_CHECKSUM,
		msg.ICAT_COLUMN_D_REPL_STATUS,
		msg.ICAT_COLUMN_D_RESC_NAME,
		msg.ICAT_COLUMN_D_DATA_PATH,
		msg.ICAT_COLUMN_D_RESC_HIER,
		msg.ICAT_COLUMN_D_CREATE_TIME,
		msg.ICAT_COLUMN_D_MODIFY_TIME,
	).Where(
		msg.ICAT_COLUMN_COLL_NAME,
		fmt.Sprintf(equalTo, collectionPath),
	).Execute(ctx)

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

		object.Path = collectionPath + "/" + name

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

// ListSubCollections returns a list of collections for the path
// Use Query for more complex queries
func (api *API) ListSubCollections(ctx context.Context, collectionPath string) ([]Collection, error) {
	var out []Collection

	results := api.Query(
		msg.ICAT_COLUMN_COLL_ID,
		msg.ICAT_COLUMN_COLL_NAME,
		msg.ICAT_COLUMN_COLL_OWNER_NAME,
		msg.ICAT_COLUMN_COLL_CREATE_TIME,
		msg.ICAT_COLUMN_COLL_MODIFY_TIME,
	).Where(
		msg.ICAT_COLUMN_COLL_PARENT_NAME,
		fmt.Sprintf(equalTo, collectionPath),
	).Execute(ctx)

	defer results.Close()

	for results.Next() {
		var c Collection

		if err := results.Scan(
			&c.ID,
			&c.Path,
			&c.Owner,
			&c.CreatedAt,
			&c.ModifiedAt,
		); err != nil {
			return nil, err
		}

		out = append(out, c)
	}

	return out, results.Err()
}

// ListMetadata returns a list of metadata for the path
// Use Query for more complex queries
func (api *API) ListMetadata(ctx context.Context, name string, itemType ObjectType) ([]Metadata, error) {
	var query PreparedQuery

	switch itemType {
	case DataObjectType:
		coll, name := Split(name)

		query = api.Query(
			msg.ICAT_COLUMN_META_DATA_ATTR_NAME,
			msg.ICAT_COLUMN_META_DATA_ATTR_VALUE,
			msg.ICAT_COLUMN_META_DATA_ATTR_UNITS,
		).Where(
			msg.ICAT_COLUMN_COLL_NAME,
			fmt.Sprintf(equalTo, coll),
		).Where(
			msg.ICAT_COLUMN_DATA_NAME,
			fmt.Sprintf(equalTo, name),
		)
	case CollectionType:
		query = api.Query(
			msg.ICAT_COLUMN_META_COLL_ATTR_NAME,
			msg.ICAT_COLUMN_META_COLL_ATTR_VALUE,
			msg.ICAT_COLUMN_META_COLL_ATTR_UNITS,
		).Where(
			msg.ICAT_COLUMN_COLL_NAME,
			fmt.Sprintf(equalTo, name),
		)
	case ResourceType:
		query = api.Query(
			msg.ICAT_COLUMN_META_RESC_ATTR_NAME,
			msg.ICAT_COLUMN_META_RESC_ATTR_VALUE,
			msg.ICAT_COLUMN_META_RESC_ATTR_UNITS,
		).Where(
			msg.ICAT_COLUMN_R_RESC_NAME,
			fmt.Sprintf(equalTo, name),
		)
	case UserType:
		zone := api.Zone

		if parts := strings.Split(name, "#"); len(parts) == 2 {
			name = parts[0]
			zone = parts[1]
		}

		query = api.Query(
			msg.ICAT_COLUMN_META_USER_ATTR_NAME,
			msg.ICAT_COLUMN_META_USER_ATTR_VALUE,
			msg.ICAT_COLUMN_META_USER_ATTR_UNITS,
		).Where(
			msg.ICAT_COLUMN_USER_NAME,
			fmt.Sprintf(equalTo, name),
		).Where(
			msg.ICAT_COLUMN_USER_ZONE,
			fmt.Sprintf(equalTo, zone),
		)
	default:
		return nil, ErrInvalidItemType
	}

	return api.executeMetadataQuery(ctx, query)
}

// ListMetadataID returns a list of metadata for the object id
// Use Query for more complex queries
func (api *API) ListMetadataID(ctx context.Context, id int64, itemType ObjectType) ([]Metadata, error) {
	var query PreparedQuery

	switch itemType {
	case DataObjectType:
		query = api.Query(
			msg.ICAT_COLUMN_META_DATA_ATTR_NAME,
			msg.ICAT_COLUMN_META_DATA_ATTR_VALUE,
			msg.ICAT_COLUMN_META_DATA_ATTR_UNITS,
		).Where(
			msg.ICAT_COLUMN_D_DATA_ID,
			fmt.Sprintf(equalToInt, id),
		)
	case CollectionType:
		query = api.Query(
			msg.ICAT_COLUMN_META_COLL_ATTR_NAME,
			msg.ICAT_COLUMN_META_COLL_ATTR_VALUE,
			msg.ICAT_COLUMN_META_COLL_ATTR_UNITS,
		).Where(
			msg.ICAT_COLUMN_COLL_ID,
			fmt.Sprintf(equalToInt, id),
		)
	case ResourceType:
		query = api.Query(
			msg.ICAT_COLUMN_META_RESC_ATTR_NAME,
			msg.ICAT_COLUMN_META_RESC_ATTR_VALUE,
			msg.ICAT_COLUMN_META_RESC_ATTR_UNITS,
		).Where(
			msg.ICAT_COLUMN_R_RESC_ID,
			fmt.Sprintf(equalToInt, id),
		)
	case UserType:
		query = api.Query(
			msg.ICAT_COLUMN_META_USER_ATTR_NAME,
			msg.ICAT_COLUMN_META_USER_ATTR_VALUE,
			msg.ICAT_COLUMN_META_USER_ATTR_UNITS,
		).Where(
			msg.ICAT_COLUMN_USER_ID,
			fmt.Sprintf(equalToInt, id),
		)
	default:
		return nil, ErrInvalidItemType
	}

	return api.executeMetadataQuery(ctx, query)
}

func (api *API) executeMetadataQuery(ctx context.Context, query PreparedQuery) ([]Metadata, error) {
	var out []Metadata

	results := query.Execute(ctx)

	defer results.Close()

	for results.Next() {
		var m Metadata

		if err := results.Scan(
			&m.Name,
			&m.Value,
			&m.Units,
		); err != nil {
			return nil, err
		}

		out = append(out, m)
	}

	return out, results.Err()
}

type Access struct {
	UserID     int64
	Permission string
}

const equalAccessType = "= 'access_type'"

var ErrInvalidItemType = errors.New("invalid item type")

func (api *API) ListAccess(ctx context.Context, path string, itemType ObjectType) ([]Access, error) {
	var query PreparedQuery

	switch itemType { //nolint:exhaustive
	case DataObjectType:
		coll, name := Split(path)

		query = api.Query(
			msg.ICAT_COLUMN_DATA_ACCESS_NAME,
			msg.ICAT_COLUMN_DATA_ACCESS_USER_ID,
		).Where(
			msg.ICAT_COLUMN_COLL_NAME, fmt.Sprintf(equalTo, coll),
		).Where(
			msg.ICAT_COLUMN_DATA_NAME, fmt.Sprintf(equalTo, name),
		).Where(
			msg.ICAT_COLUMN_DATA_TOKEN_NAMESPACE, equalAccessType,
		)
	case CollectionType:
		query = api.Query(
			msg.ICAT_COLUMN_COLL_ACCESS_NAME,
			msg.ICAT_COLUMN_COLL_ACCESS_USER_ID,
		).Where(
			msg.ICAT_COLUMN_COLL_NAME, fmt.Sprintf(equalTo, path),
		).Where(
			msg.ICAT_COLUMN_COLL_TOKEN_NAMESPACE, equalAccessType,
		)
	default:
		return nil, ErrInvalidItemType
	}

	return api.executeAccessQuery(ctx, query)
}

func (api *API) ListAccessID(ctx context.Context, id int64, itemType ObjectType) ([]Access, error) {
	var query PreparedQuery

	switch itemType { //nolint:exhaustive
	case DataObjectType:
		query = api.Query(
			msg.ICAT_COLUMN_DATA_ACCESS_NAME,
			msg.ICAT_COLUMN_DATA_ACCESS_USER_ID,
		).Where(
			msg.ICAT_COLUMN_D_DATA_ID, fmt.Sprintf(equalToInt, id),
		).Where(
			msg.ICAT_COLUMN_DATA_TOKEN_NAMESPACE, equalAccessType,
		)
	case CollectionType:
		query = api.Query(
			msg.ICAT_COLUMN_COLL_ACCESS_NAME,
			msg.ICAT_COLUMN_COLL_ACCESS_USER_ID,
		).Where(
			msg.ICAT_COLUMN_COLL_ID, fmt.Sprintf(equalToInt, id),
		).Where(
			msg.ICAT_COLUMN_COLL_TOKEN_NAMESPACE, equalAccessType,
		)
	default:
		return nil, ErrInvalidItemType
	}

	return api.executeAccessQuery(ctx, query)
}

func (api *API) executeAccessQuery(ctx context.Context, query PreparedQuery) ([]Access, error) {
	var out []Access

	results := query.Execute(ctx)

	defer results.Close()

	for results.Next() {
		var a Access

		if err := results.Scan(
			&a.Permission,
			&a.UserID,
		); err != nil {
			return nil, err
		}

		out = append(out, a)
	}

	return out, results.Err()
}
