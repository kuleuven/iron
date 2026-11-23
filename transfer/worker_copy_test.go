package transfer

import (
	"context"
	"errors"
	"testing"

	"github.com/kuleuven/iron/api"
	"github.com/kuleuven/iron/msg"
)

func TestWorkerCopy(t *testing.T) {
	copyErr := errors.New("copy failed")
	touchErr := errors.New("touch failed")

	for _, tc := range []struct {
		name     string
		flags    Flags
		dryRun   bool
		copyErr  error
		touchErr error
		mismatch bool
		wantErr  error
	}{
		{name: "overwrite"},
		{name: "exclusive", flags: Flags{Exclusive: true}},
		{name: "existing target", flags: Flags{Exclusive: true}, copyErr: copyErr, wantErr: copyErr},
		{name: "server-side dry run", flags: Flags{SyncModTime: true, IntegrityChecksums: true}, dryRun: true},
		{name: "verify checksum", flags: Flags{IntegrityChecksums: true}},
		{name: "different checksums", flags: Flags{IntegrityChecksums: true}, mismatch: true, wantErr: ErrChecksumMismatch},
		{name: "sync modification time", flags: Flags{SyncModTime: true}},
		{name: "modification time error", flags: Flags{SyncModTime: true}, touchErr: touchErr, wantErr: touchErr},
		{name: "metadata", flags: Flags{CopyMetadata: true}},
		{name: "ACL", flags: Flags{CopyACL: true}},
		{name: "verify before modification time and ACL", flags: Flags{IntegrityChecksums: true, SyncModTime: true, CopyACL: true}},
		{name: "checksum mismatch skips ACL", flags: Flags{IntegrityChecksums: true, CopyACL: true}, mismatch: true, wantErr: ErrChecksumMismatch},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := &api.MockConn{}
			target.AddResponse(copySourceResponse())

			if !tc.dryRun {
				if tc.flags.CopyACL {
					addACLDialog(target, api.DataObjectType)
					target.AddResponse(copySourceResponse())
					target.AddResponse(msg.QueryResponse{})
					target.AddResponse(msg.QueryResponse{})
				}

				addCopyDialog(target, tc.flags, tc.copyErr, tc.touchErr, tc.mismatch)
			}

			worker := New(nil, mockTransferAPI(target), Options{DryRun: tc.dryRun})
			worker.Copy(t.Context(), testCopySrc, testCopyDest, tc.flags)

			if err := worker.Wait(); !errors.Is(err, tc.wantErr) {
				t.Fatalf("Wait() = %v, want %v", err, tc.wantErr)
			}

			assertTransferDialogs(t, target)
		})
	}
}

func TestWorkerCopySkipsIdenticalACL(t *testing.T) {
	conn := &api.MockConn{}
	conn.AddResponse(copySourceResponse())
	addACLDialog(conn, api.DataObjectType)
	conn.AddResponse(copySourceResponse())
	addACLDialog(conn, api.DataObjectType)
	addCopyDialog(conn, Flags{}, nil, nil, false)

	worker := New(nil, mockTransferAPI(conn), Options{})
	worker.Copy(t.Context(), testCopySrc, testCopyDest, Flags{CopyACL: true})

	if err := worker.Wait(); err != nil {
		t.Fatal(err)
	}

	assertTransferDialogs(t, conn)
}

func TestWorkerCopyMetadataDeleteUsesExplicitChanges(t *testing.T) {
	conn := &api.MockConn{}
	conn.AddResponse(copySourceResponse())
	addMetadataResponse(conn, []api.Metadata{{Name: testMetadataName, Value: testValue}}, msg.ICAT_COLUMN_META_DATA_ATTR_NAME, nil)
	conn.AddResponse(copySourceResponse())
	addMetadataResponse(conn, []api.Metadata{{Name: testTargetOnly, Value: testDifferent}}, msg.ICAT_COLUMN_META_DATA_ATTR_NAME, nil)

	addCopyDialog(conn, Flags{}, nil, nil, false)
	conn.Add(msg.ATOMIC_APPLY_METADATA_OPERATIONS_APN, &msg.AtomicMetadataRequest{
		ItemName: testCopyDest,
		ItemType: api.DataObjectType.String(),
		Operations: []msg.MetadataOperation{
			{Operation: "remove", Name: testTargetOnly, Value: testDifferent},
			{Operation: "add", Name: testMetadataName, Value: testValue},
		},
	}, msg.EmptyResponse{})

	worker := New(nil, mockTransferAPI(conn), Options{})
	worker.Copy(t.Context(), testCopySrc, testCopyDest, Flags{CopyMetadata: true, Delete: true})

	if err := worker.Wait(); err != nil {
		t.Fatal(err)
	}

	assertTransferDialogs(t, conn)
}

func TestWorkerCopyRequiresDataObject(t *testing.T) {
	conn := &api.MockConn{}
	conn.AddResponse(msg.QueryResponse{})

	worker := New(nil, mockTransferAPI(conn), Options{})
	worker.Copy(t.Context(), testCopySrc, testCopyDest, Flags{})

	if err := worker.Wait(); !errors.Is(err, api.ErrNoRowFound) {
		t.Fatalf("Wait() = %v, want %v", err, api.ErrNoRowFound)
	}

	assertTransferDialogs(t, conn)
}

type gatedCopyConn struct {
	*api.MockConn
	started chan<- struct{}
	release <-chan struct{}
}

func (conn *gatedCopyConn) Request(ctx context.Context, number msg.APINumber, request, response any) error {
	if number == msg.DATA_OBJ_COPY_AN {
		conn.started <- struct{}{}

		select {
		case <-conn.release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return conn.MockConn.Request(ctx, number, request, response)
}

func TestWorkerOverlappingCopyFlags(t *testing.T) {
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	conns := make(chan api.Conn, 4)
	flags := Flags{Exclusive: true, SyncModTime: true, CopyMetadata: true}

	first, second := &api.MockConn{}, &api.MockConn{}
	for i, conn := range []*api.MockConn{first, second} {
		conn.AddResponse(copySourceResponse())

		copyFlags := flags
		if i == 1 {
			copyFlags = Flags{}
		}

		addCopyDialog(conn, copyFlags, nil, nil, false)

		gated := &gatedCopyConn{MockConn: conn, started: started, release: release}

		conns <- gated

		conns <- gated
	}

	pool := &api.API{Connect: func(context.Context) (api.Conn, error) { return <-conns, nil }}
	worker := New(nil, pool, Options{})
	worker.Copy(t.Context(), testCopySrc, testCopyDest, flags)

	flags = Flags{}
	worker.Copy(t.Context(), testCopySrc, testCopyDest, flags)

	for range 2 {
		<-started
	}

	close(release)

	if err := worker.Wait(); err != nil {
		t.Fatal(err)
	}

	assertTransferDialogs(t, first, second)
}

func TestWorkerCopySourceError(t *testing.T) {
	sourceErr := errors.New("source lookup failed")
	conn := &api.MockConn{}
	conn.AddResponse(sourceErr)

	worker := New(nil, mockTransferAPI(conn), Options{})
	worker.Copy(t.Context(), testCopySrc, testCopyDest, Flags{})

	if err := worker.Wait(); !errors.Is(err, sourceErr) {
		t.Fatalf("Wait() = %v, want %v", err, sourceErr)
	}

	assertTransferDialogs(t, conn)
}

func copySourceResponse() msg.QueryResponse {
	return msg.QueryResponse{
		RowCount:       1,
		AttributeCount: 14,
		SQLResult: []msg.SQLResult{
			{AttributeIndex: 401, Values: []string{"1"}},
			{AttributeIndex: 500, Values: []string{"1"}},
			{AttributeIndex: 406, Values: []string{"generic"}},
			{AttributeIndex: 404, Values: []string{"0"}},
			{AttributeIndex: 407, Values: []string{"4"}},
			{AttributeIndex: 411, Values: []string{testUser}},
			{AttributeIndex: 412, Values: []string{testZone}},
			{AttributeIndex: 415, Values: []string{testSha}},
			{AttributeIndex: 413, Values: []string{"1"}},
			{AttributeIndex: 409, Values: []string{testDemoResc}},
			{AttributeIndex: 410, Values: []string{testCopySrc}},
			{AttributeIndex: 422, Values: []string{testDemoResc}},
			{AttributeIndex: 419, Values: []string{"10000"}},
			{AttributeIndex: 420, Values: []string{"10000"}},
		},
	}
}

func addCopyDialog(conn *api.MockConn, options Flags, copyErr, touchErr error, mismatch bool) {
	request := msg.DataObjectCopyRequest{
		Paths: []msg.DataObjectRequest{
			{Path: testCopySrc, OperationType: msg.OPER_TYPE_COPY_DATA_OBJ_SRC},
			{Path: testCopyDest, OperationType: msg.OPER_TYPE_COPY_DATA_OBJ_DEST},
		},
	}
	if !options.Exclusive {
		request.Paths[1].KeyVals.Add(msg.FORCE_FLAG_KW, "")
	}

	var response any = msg.EmptyResponse{}

	if copyErr != nil {
		response = copyErr
	}

	conn.Add(msg.DATA_OBJ_COPY_AN, request, response)

	if copyErr != nil {
		return
	}

	if options.IntegrityChecksums {
		if mismatch { // Checksum requests run concurrently, so either path may be requested first.
			conn.AddResponses([]any{msg.String{String: testSha}, transferTestChecksum(testStr, true)})

			return
		}

		conn.AddResponses([]any{msg.String{String: testSha}, msg.String{String: testSha}})
	}

	if options.SyncModTime {
		response = msg.EmptyResponse{}
		if touchErr != nil {
			response = touchErr
		}

		conn.Add(msg.TOUCH_APN, msg.TouchDataObjectReplicaRequest{
			Path: testCopyDest,
			Options: msg.TouchOptions{
				SecondsSinceEpoch: 10000,
				NoCreate:          true,
			},
		}, response)

		if touchErr != nil {
			return
		}
	}

	if options.CopyMetadata {
		conn.AddResponse(msg.EmptyResponse{})
	}

	if options.CopyACL {
		addACLModification(conn)
	}
}

func TestWorkerCopyDirectoryMetadataOnly(t *testing.T) {
	testConn := &api.MockConn{}
	testConn.Add(msg.ATOMIC_APPLY_METADATA_OPERATIONS_APN, &msg.AtomicMetadataRequest{
		ItemName: testCopyDest,
		ItemType: api.DataObjectType.String(),
		Operations: []msg.MetadataOperation{{
			Operation: testMetadataAdd,
			Name:      testMetadataName,
			Value:     testValue,
		}},
	}, msg.EmptyResponse{})
	testConn.Add(msg.MOD_ACCESS_CONTROL_AN, msg.ModifyAccessRequest{
		Path:        testCopyDest,
		UserName:    testUser,
		Zone:        testZone,
		AccessLevel: testOwnPermission,
	}, msg.EmptyResponse{})

	testAPI := mockTransferAPI(testConn)
	worker := New(testAPI, testAPI, Options{MaxQueued: 1})
	task := Task{
		Action: SyncMetadata,
		Source: testCopySrc,
		Target: testCopyDest,
		Attributes: AttributeChanges{
			ItemType:      api.DataObjectType,
			MetadataToAdd: []api.Metadata{{Name: testMetadataName, Value: testValue}},
			AccessToAdd:   []api.Access{{User: api.User{Name: testUser, Zone: testZone}, Permission: testOwnPermission}},
		},
	}

	worker.copyDir(t.Context(), testCopySrc, testCopyDest, testAPI, copyDirCallbacks{
		copyObject:   worker.copyObject,
		syncMetadata: worker.copyObjectMetadata,
		copyCollection: func(context.Context, Task, Flags) error {
			return nil
		},
		compute: func(_ context.Context, _, _ string, queue chan<- Task, _ Flags) error {
			defer close(queue)

			queue <- task

			return nil
		},
	}, Flags{CopyMetadata: true, CopyACL: true})

	if err := worker.Wait(); err != nil {
		t.Fatal(err)
	}

	assertTransferDialogs(t, testConn)
}

func TestWorkerRemoteCopyMetadataOnly(t *testing.T) {
	sourceConn, targetConn := &api.MockConn{}, &api.MockConn{}
	sourceRecord := &testCollectionRecord{
		Collection: &api.Collection{Inheritance: true},
		metadata:   []api.Metadata{{Name: testMetadataName, Value: testValue}},
		access:     []api.Access{{User: api.User{Name: testUser, Zone: testZone}, Permission: testOwnPermission}},
	}
	targetAPI := mockTransferAPI(targetConn)
	inheritance := true

	targetConn.Add(msg.ATOMIC_APPLY_METADATA_OPERATIONS_APN, &msg.AtomicMetadataRequest{
		ItemName: testCopyDest,
		ItemType: api.CollectionType.String(),
		Operations: []msg.MetadataOperation{{
			Operation: testMetadataAdd,
			Name:      testMetadataName,
			Value:     testValue,
		}},
	}, msg.EmptyResponse{})
	targetConn.Add(msg.MOD_ACCESS_CONTROL_AN, msg.ModifyAccessRequest{
		Path:        testCopyDest,
		UserName:    testUser,
		Zone:        testZone,
		AccessLevel: testOwnPermission,
	}, msg.EmptyResponse{})
	targetConn.Add(msg.MOD_ACCESS_CONTROL_AN, msg.ModifyAccessRequest{
		Path:        testCopyDest,
		AccessLevel: testInherit,
	}, msg.EmptyResponse{})

	worker := New(mockTransferAPI(sourceConn), mockTransferAPI(sourceConn), Options{})
	worker.RemoteTransferPool = targetAPI
	worker.remoteCopyObjectMetadata(t.Context(), Task{
		Source: testCopySrc,
		Target: testCopyDest,
		Attributes: AttributeChanges{
			ItemType:              api.CollectionType,
			MetadataToAdd:         sourceRecord.metadata,
			AccessToAdd:           sourceRecord.access,
			CollectionInheritance: &inheritance,
		},
	}, Flags{CopyMetadata: true, CopyACL: true})

	if err := worker.Wait(); err != nil {
		t.Fatal(err)
	}

	assertTransferDialogs(t, sourceConn, targetConn)
}

type testCopyRecord struct {
	*api.DataObject
	metadata []api.Metadata
	access   []api.Access
}

func (record testCopyRecord) Metadata() []api.Metadata {
	return record.metadata
}

func (record testCopyRecord) Access() []api.Access {
	return record.access
}

func (testCopyRecord) Type() api.ObjectType {
	return api.DataObjectType
}
