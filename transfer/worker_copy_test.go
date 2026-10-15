package transfer

import (
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
		options  Options
		copyErr  error
		touchErr error
		mismatch bool
		wantErr  error
	}{
		{name: "overwrite"},
		{name: "exclusive", options: Options{Exclusive: true}},
		{name: "existing target", options: Options{Exclusive: true}, copyErr: copyErr, wantErr: copyErr},
		{name: "server-side dry run", options: Options{DryRun: true, SyncModTime: true, IntegrityChecksums: true}},
		{name: "verify checksum", options: Options{IntegrityChecksums: true}},
		{name: "different checksums", options: Options{IntegrityChecksums: true}, mismatch: true, wantErr: ErrChecksumMismatch},
		{name: "sync modification time", options: Options{SyncModTime: true}},
		{name: "modification time error", options: Options{SyncModTime: true}, touchErr: touchErr, wantErr: touchErr},
		{name: "metadata", options: Options{CopyMetadata: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := &api.MockConn{}
			target.AddResponse(copySourceResponse())
			addCopyDialog(target, tc.options, tc.copyErr, tc.touchErr, tc.mismatch)

			worker := New(nil, mockTransferAPI(target), tc.options)
			worker.Copy(t.Context(), testCopySrc, testCopyDest)

			if err := worker.Wait(); !errors.Is(err, tc.wantErr) {
				t.Fatalf("Wait() = %v, want %v", err, tc.wantErr)
			}

			assertTransferDialogs(t, target)
		})
	}
}

func TestWorkerCopySourceError(t *testing.T) {
	sourceErr := errors.New("source lookup failed")
	conn := &api.MockConn{}
	conn.AddResponse(sourceErr)

	worker := New(nil, mockTransferAPI(conn), Options{})
	worker.Copy(t.Context(), testCopySrc, testCopyDest)

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

func addCopyDialog(conn *api.MockConn, options Options, copyErr, touchErr error, mismatch bool) {
	if options.DryRun {
		return
	}

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

	if options.IntegrityChecksums {
		if mismatch {
			// Checksum requests run concurrently, so either path may be requested first.
			conn.AddResponse(msg.String{String: testSha})
			conn.AddResponse(transferTestChecksum(testStr, true))
		} else {
			for range 2 {
				conn.AddResponse(msg.String{String: testSha})
			}
		}
	}
}
