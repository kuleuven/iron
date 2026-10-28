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
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := &api.MockConn{}
			target.AddResponse(copySourceResponse())

			if !tc.dryRun {
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

	if !options.IntegrityChecksums {
		return
	}

	if mismatch { // Checksum requests run concurrently, so either path may be requested first.
		conn.AddResponses([]any{msg.String{String: testSha}, transferTestChecksum(testStr, true)})

		return
	}

	conn.AddResponses([]any{msg.String{String: testSha}, msg.String{String: testSha}})
}
