package api

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gitea.icts.kuleuven.be/coz/iron/msg"
)

type Collection struct {
	ID         int64
	Path       string // Path has an absolute path to the collection
	Owner      string
	CreatedAt  time.Time
	ModifiedAt time.Time
}

type DataObject struct {
	ID           int64
	CollectionID int64
	Path         string
	Size         int64
	DataType     string
	Replicas     []Replica
}

type Replica struct {
	Number            int
	Owner             string
	Checksum          string
	Status            string
	ResourceName      string
	PhysicalPath      string
	ResourceHierarchy string
	CreatedAt         time.Time
	ModifiedAt        time.Time
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

type User struct {
	ID         int64
	Name       string
	Zone       string
	Type       string
	CreatedAt  time.Time
	ModifiedAt time.Time
}

const equalTo = "= '%s'"

// GetCollection returns a collection for the path
// Use Query for more complex queries
func (api *API) GetCollection(ctx context.Context, path string) (*Collection, error) {
	var c Collection

	err := api.QueryRow(
		msg.ICAT_COLUMN_COLL_ID,
		msg.ICAT_COLUMN_COLL_NAME,
		msg.ICAT_COLUMN_COLL_OWNER_NAME,
		msg.ICAT_COLUMN_COLL_CREATE_TIME,
		msg.ICAT_COLUMN_COLL_MODIFY_TIME,
	).Where(
		msg.ICAT_COLUMN_COLL_NAME,
		fmt.Sprintf(equalTo, path),
	).Execute(ctx).Scan(
		&c.ID,
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
		msg.ICAT_COLUMN_DATA_SIZE,
		msg.ICAT_COLUMN_DATA_TYPE_NAME,
		msg.ICAT_COLUMN_DATA_REPL_NUM,
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
			&d.Size,
			&d.DataType,
			&replica.Number,
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

// Clean makes sure that the path is clean
func Clean(path string) string {
	if path == "/" {
		return path
	}

	return strings.TrimSuffix(path, "/")
}

// GetResource returns information about a resource, identified by its name
// Use Query for more complex queries
func (api *API) GetResource(ctx context.Context, name string) (*Resource, error) {
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
		msg.ICAT_COLUMN_R_RESC_NAME,
		fmt.Sprintf(equalTo, name),
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

// ListDataObjects returns a list of data objects for the path
// Use Query for more complex queries
func (api *API) ListDataObjects(ctx context.Context, collectionPath string) ([]DataObject, error) { //nolint:funlen
	result := []DataObject{}
	mapping := map[int64]*DataObject{}
	results := api.Query(
		msg.ICAT_COLUMN_D_DATA_ID,
		msg.ICAT_COLUMN_DATA_NAME,
		msg.ICAT_COLUMN_COLL_ID,
		msg.ICAT_COLUMN_DATA_SIZE,
		msg.ICAT_COLUMN_DATA_TYPE_NAME,
		msg.ICAT_COLUMN_DATA_REPL_NUM,
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
			&object.Size,
			&object.DataType,
			&replica.Number,
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
