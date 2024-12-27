package api

import (
	"context"
	"fmt"
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

func (api *api) GetCollection(ctx context.Context, path string) (*Collection, error) {
	var c Collection

	err := api.QueryRow(
		msg.ICAT_COLUMN_COLL_ID,
		msg.ICAT_COLUMN_COLL_NAME,
		msg.ICAT_COLUMN_COLL_OWNER_NAME,
		msg.ICAT_COLUMN_COLL_CREATE_TIME,
		msg.ICAT_COLUMN_COLL_MODIFY_TIME,
	).Where(
		msg.ICAT_COLUMN_COLL_NAME,
		fmt.Sprintf("= '%s'", path),
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
