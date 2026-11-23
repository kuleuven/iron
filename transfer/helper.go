package transfer

import (
	"context"
	"errors"
	"fmt"

	"github.com/kuleuven/iron/api"
)

// ConnectionAsAPI returns a new api.API instance that uses the given connection as each connection.
// The returned api.API instance will always use the provided connection for its Connect method,
// ignoring any connection pooling or other connection logic in the parent API.
// Note that the resulting *ConnAPI must be closed to release the underlying connection.
func ConnectionAsAPI(parent *api.API, conn api.Conn) *connAPI {
	child := *parent

	// Build channel with single connection
	ch := make(chan api.Conn, 1)
	ch <- conn

	// Override the Connect method to always return the single connection from the channel
	child.Connect = func(ctx context.Context) (api.Conn, error) {
		conn := <-ch

		return &returnOnClose{
			Conn: conn,
			ch:   ch,
		}, nil
	}

	return &connAPI{
		ch:  ch,
		API: &child,
	}
}

type connAPI struct {
	ch chan api.Conn
	*api.API
}

func (c *connAPI) Close() error {
	close(c.ch)

	for conn := range c.ch {
		conn.Close()
	}

	return nil
}

type returnOnClose struct {
	api.Conn
	ch chan<- api.Conn
}

func (r *returnOnClose) Close() error {
	r.ch <- r.Conn

	return nil
}

// RemoteCopyMetadata adds source metadata missing from the target, preserving target-only metadata.
func RemoteCopyMetadata(ctx context.Context, a, b *api.API, source string, sourceType api.ObjectType, target string, targetType api.ObjectType) error {
	sourceMetadata, err := a.ListMetadata(ctx, source, sourceType)
	if err != nil {
		return err
	}

	targetMetadata, err := b.ListMetadata(ctx, target, targetType)
	if err != nil {
		return err
	}

	return remoteCopyMetadata(ctx, b, target, targetType, sourceMetadata, targetMetadata)
}

func remoteCopyMetadata(ctx context.Context, b *api.API, target string, targetType api.ObjectType, sourceMetadata, targetMetadata []api.Metadata) error {
	return (AttributeChanges{
		ItemType:      targetType,
		MetadataToAdd: metadataToAdd(sourceMetadata, targetMetadata),
	}).Apply(ctx, b, target)
}

func metadataToAdd(source, target []api.Metadata) []api.Metadata {
	existing := make(map[api.Metadata]struct{}, len(target))
	for _, metadata := range target {
		existing[metadata] = struct{}{}
	}

	var add []api.Metadata

	for _, metadata := range source {
		if _, ok := existing[metadata]; ok {
			continue
		}

		add = append(add, metadata)
		existing[metadata] = struct{}{}
	}

	return add
}

func accessToAdd(source, target []api.Access) []api.Access {
	type principal struct {
		name string
		zone string
	}

	existing := make(map[principal]string, len(target))
	for _, access := range target {
		existing[principal{access.User.Name, access.User.Zone}] = access.Permission
	}

	var add []api.Access

	for _, access := range source {
		key := principal{access.User.Name, access.User.Zone}
		if permission, ok := existing[key]; ok && permission == access.Permission {
			continue
		}

		add = append(add, access)
		existing[key] = access.Permission
	}

	return add
}

func metadataToRemove(source, target []api.Metadata) []api.Metadata {
	existing := make(map[api.Metadata]struct{}, len(source))
	for _, metadata := range source {
		existing[metadata] = struct{}{}
	}

	var remove []api.Metadata

	removed := make(map[api.Metadata]struct{}, len(target))

	for _, metadata := range target {
		if _, ok := existing[metadata]; ok {
			continue
		}

		if _, ok := removed[metadata]; ok {
			continue
		}

		remove = append(remove, metadata)
		removed[metadata] = struct{}{}
	}

	return remove
}

func accessToRemove(source, target []api.Access) []api.Access {
	type principal struct {
		name string
		zone string
	}

	existing := make(map[principal]struct{}, len(source))
	for _, access := range source {
		existing[principal{access.User.Name, access.User.Zone}] = struct{}{}
	}

	var remove []api.Access

	removed := make(map[principal]struct{}, len(target))

	for _, access := range target {
		key := principal{access.User.Name, access.User.Zone}
		if _, ok := existing[key]; ok {
			continue
		}

		if _, ok := removed[key]; ok {
			continue
		}

		remove = append(remove, access)
		removed[key] = struct{}{}
	}

	return remove
}

const removeAccessPermission = "null"

// AttributeChanges contains precomputed metadata and ACL changes for a target object.
type AttributeChanges struct {
	MetadataToCopyFrom    string
	ItemType              api.ObjectType
	MetadataToAdd         []api.Metadata
	MetadataToRemove      []api.Metadata
	AccessToAdd           []api.Access
	AccessToRemove        []api.Access
	CollectionInheritance *bool
}

func ComputeAttributeChanges(source, target api.Record, flags Flags, sameZone bool) (AttributeChanges, error) {
	if !flags.CopyMetadata && !flags.CopyACL {
		return AttributeChanges{}, nil
	}

	if source == nil {
		return AttributeChanges{}, fmt.Errorf("source record is required to copy metadata or ACLs")
	}

	changes := AttributeChanges{ItemType: source.Type()}

	if flags.CopyMetadata {
		if err := setMetadataChanges(&changes, source, target, sameZone, flags.Delete); err != nil {
			return AttributeChanges{}, err
		}
	}

	if flags.CopyACL {
		if err := setACLChanges(&changes, source, target, flags.Delete); err != nil {
			return AttributeChanges{}, err
		}
	}

	return changes, nil
}

func setMetadataChanges(changes *AttributeChanges, source, target api.Record, sameZone, deleteMissing bool) error {
	var targetMetadata []api.Metadata

	if target != nil {
		targetMetadata = target.Metadata()
	}

	sourceMetadata := source.Metadata()

	if deleteMissing {
		changes.MetadataToRemove = metadataToRemove(sourceMetadata, targetMetadata)
	}

	add := metadataToAdd(sourceMetadata, targetMetadata)

	if len(add) == 0 {
		return nil
	}

	if !sameZone || deleteMissing {
		changes.MetadataToAdd = add

		return nil
	}

	sourcePath, err := recordPath(source)
	if err != nil {
		return err
	}

	changes.MetadataToCopyFrom = sourcePath

	return nil
}

func setACLChanges(changes *AttributeChanges, source, target api.Record, deleteMissing bool) error {
	var targetAccess []api.Access

	if target != nil {
		targetAccess = target.Access()
	}

	sourceAccess := source.Access()
	changes.AccessToAdd = accessToAdd(sourceAccess, targetAccess)

	if deleteMissing {
		changes.AccessToRemove = accessToRemove(sourceAccess, targetAccess)
	}

	if source.Type() != api.CollectionType {
		return nil
	}

	sourceCollection, ok := source.Sys().(*api.Collection)
	if !ok {
		return fmt.Errorf("collection ACL record has unexpected type %T", source.Sys())
	}

	if target != nil && target.Type() == api.CollectionType {
		targetCollection, ok := target.Sys().(*api.Collection)
		if !ok {
			return fmt.Errorf("target collection ACL record has unexpected type %T", target.Sys())
		}

		if sourceCollection.Inheritance == targetCollection.Inheritance {
			return nil
		}
	}

	inheritance := sourceCollection.Inheritance
	changes.CollectionInheritance = &inheritance

	return nil
}

func recordPath(record api.Record) (string, error) {
	switch object := record.Sys().(type) {
	case *api.Collection:
		return object.Path, nil
	case *api.DataObject:
		return object.Path, nil
	default:
		return "", fmt.Errorf("record has unexpected object type %T", record.Sys())
	}
}

func (changes AttributeChanges) HasChanges() bool {
	return changes.MetadataToCopyFrom != "" || len(changes.MetadataToAdd) > 0 ||
		len(changes.MetadataToRemove) > 0 || len(changes.AccessToAdd) > 0 ||
		len(changes.AccessToRemove) > 0 || changes.CollectionInheritance != nil
}

// Apply applies the precomputed metadata and ACL changes to the target object.
func (changes AttributeChanges) Apply(ctx context.Context, targetAPI *api.API, target string) error {
	var err error

	if changes.MetadataToCopyFrom != "" && len(changes.MetadataToRemove) == 0 {
		err = errors.Join(err, targetAPI.CopyMetadata(
			ctx,
			changes.MetadataToCopyFrom,
			changes.ItemType,
			target,
			changes.ItemType,
		))
	} else if len(changes.MetadataToAdd) > 0 || len(changes.MetadataToRemove) > 0 {
		err = errors.Join(err, targetAPI.ModifyMetadata(
			ctx, target, changes.ItemType, changes.MetadataToAdd, changes.MetadataToRemove,
		))
	}

	for _, entry := range changes.AccessToAdd {
		user := entry.User.Name
		if entry.User.Zone != "" {
			user += "#" + entry.User.Zone
		}

		err = errors.Join(err, targetAPI.ModifyAccess(ctx, target, user, entry.Permission, false))
	}

	for _, entry := range changes.AccessToRemove {
		user := entry.User.Name
		if entry.User.Zone != "" {
			user += "#" + entry.User.Zone
		}

		err = errors.Join(err, targetAPI.ModifyAccess(ctx, target, user, removeAccessPermission, false))
	}

	if changes.CollectionInheritance != nil {
		err = errors.Join(err, targetAPI.SetCollectionInheritance(ctx, target, *changes.CollectionInheritance, false))
	}

	return err
}

// RemoteCopyACL copies source access permissions onto the target.
func RemoteCopyACL(ctx context.Context, a, b *api.API, source string, itemType api.ObjectType, target string) error {
	record, err := a.GetRecord(ctx, source, api.FetchAccess)
	if err != nil {
		return err
	}

	if record.Type() != itemType {
		return api.ErrInvalidItemType
	}

	changes, err := ComputeAttributeChanges(record, nil, Flags{CopyACL: true}, true)
	if err != nil {
		return err
	}

	return changes.Apply(ctx, b, target)
}
