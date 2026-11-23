package transfer

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kuleuven/iron/api"
	"github.com/kuleuven/iron/msg"
)

func TestDownloadDirCompareChecksumPaths(t *testing.T) {
	t.Chdir(t.TempDir())
	writeIdenticalLocalFile(t)

	conn := &api.MockConn{}
	conn.AddResponses(responses)
	conn.Add(msg.DATA_OBJ_CHKSUM_AN, msg.DataObjectRequest{Path: testFile1}, transferTestChecksum(testStr, false))

	var output bytes.Buffer

	worker := New(mockTransferAPI(conn), nil, Options{
		DryRun:          true,
		Output:          &output,
		ProgressHandler: func(context.Context, Progress) {},
	})
	worker.DownloadDir(t.Context(), "./", "/test", Flags{CompareChecksums: true})

	if err := worker.Wait(); err != nil {
		t.Fatal(err)
	}

	if output.Len() != 0 {
		t.Fatalf("identical checksums scheduled a download: %q", output.String())
	}

	assertTransferDialogs(t, conn)
}

func TestCopyWalkOptionsFetchRequestedAttributes(t *testing.T) {
	options := copyWalkOptions(Flags{CopyMetadata: true, CopyACL: true})

	var metadata, access bool

	for _, option := range options {
		metadata = metadata || option == api.FetchMetadata
		access = access || option == api.FetchAccess
	}

	if !metadata || !access {
		t.Errorf("copy walk options = %v, want FetchMetadata and FetchAccess", options)
	}
}

type synchronizeMetadataCase struct {
	name                 string
	sourceModTime        time.Time
	targetModTime        time.Time
	compareChecksums     bool
	syncModificationTime bool
	wantModificationTime bool
}

func TestSynchronizeMetadataForIdenticalObjects(t *testing.T) {
	modTime := time.Unix(10000, 0)
	tests := []synchronizeMetadataCase{
		{name: "same modification time", sourceModTime: modTime, targetModTime: modTime},
		{
			name:                 "identical checksums",
			sourceModTime:        modTime.Add(time.Second),
			targetModTime:        modTime,
			compareChecksums:     true,
			syncModificationTime: true,
			wantModificationTime: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			testSynchronizeMetadataCase(t, tc)
		})
	}
}

func TestSynchronizeDoesNotQueueUnselectedMetadata(t *testing.T) {
	record := func(path string) *testCopyRecord {
		return &testCopyRecord{
			DataObject: &api.DataObject{
				Path: path,
				Replicas: []api.Replica{{
					Size:       int64(len(testStr)),
					ModifiedAt: time.Unix(10000, 0),
				}},
			},
		}
	}

	sourceRecord, targetRecord := record(testCopySrc), record(testCopyDest)
	sourceRecord.metadata = []api.Metadata{{Name: testMetadataName, Value: testValue}}
	sourceRecord.access = []api.Access{{User: api.User{Name: testUser, Zone: testZone}, Permission: testOwnPermission}}

	left := make(chan *object, 1)
	left <- &object{source: testCopySrc, target: testCopyDest, info: sourceRecord}

	close(left)

	right := make(chan *object, 1)
	right <- &object{source: testCopyDest, target: testCopyDest, info: targetRecord}

	close(right)

	queue := make(chan Task, 1)
	if err := Synchronize(t.Context(), left, right, queue, SynchronizeOptions{
		ResyncMetadata: true,
	}); err != nil {
		t.Fatal(err)
	}

	if task, ok := <-queue; ok {
		t.Fatalf("queued attributes without a narrow option selected: %+v", task)
	}
}

func TestSynchronizeMetadataForExistingCollections(t *testing.T) {
	sourceRecord := &testCollectionRecord{
		Collection: &api.Collection{Path: testCopySrc, Inheritance: true},
		metadata:   []api.Metadata{{Name: testMetadataName, Value: testValue}},
		access:     []api.Access{{User: api.User{Name: testUser, Zone: testZone}, Permission: testOwnPermission}},
	}
	targetRecord := &testCollectionRecord{Collection: &api.Collection{Path: testCopyDest}}

	left := make(chan *object, 1)
	left <- &object{source: testCopySrc, target: testCopyDest, info: sourceRecord}

	close(left)

	right := make(chan *object, 1)
	right <- &object{source: testCopyDest, target: testCopyDest, info: targetRecord}

	close(right)

	queue := make(chan Task, 1)
	if err := Synchronize(t.Context(), left, right, queue, SynchronizeOptions{
		ResyncMetadata: true,
		CopyMetadata:   true,
		CopyACL:        true,
	}); err != nil {
		t.Fatal(err)
	}

	task, ok := <-queue
	if !ok {
		t.Fatal("existing collection did not schedule metadata and ACL synchronization")
	}

	if task.Action != SyncMetadata {
		t.Fatalf("task action = %v, want %v", task.Action, SyncMetadata)
	}

	if task.Attributes.ItemType != api.CollectionType {
		t.Errorf("metadata task item type = %v, want %v", task.Attributes.ItemType, api.CollectionType)
	}

	if len(task.Attributes.MetadataToAdd) != 1 || task.Attributes.MetadataToAdd[0] != sourceRecord.metadata[0] {
		t.Errorf("metadata to add = %v, want %v", task.Attributes.MetadataToAdd, sourceRecord.metadata)
	}

	if len(task.Attributes.AccessToAdd) != 1 || task.Attributes.AccessToAdd[0] != sourceRecord.access[0] {
		t.Errorf("access to add = %v, want %v", task.Attributes.AccessToAdd, sourceRecord.access)
	}

	if task.Attributes.CollectionInheritance == nil || !*task.Attributes.CollectionInheritance {
		t.Errorf("collection inheritance = %v, want true", task.Attributes.CollectionInheritance)
	}

	if _, ok := <-queue; ok {
		t.Fatal("existing collection scheduled unexpected additional tasks")
	}
}

func testSynchronizeMetadataCase(t *testing.T, tc synchronizeMetadataCase) {
	t.Helper()

	sourceRecord := &testCopyRecord{
		DataObject: &api.DataObject{
			Path: testCopySrc,
			Replicas: []api.Replica{{
				Size:       int64(len(testStr)),
				ModifiedAt: tc.sourceModTime,
			}},
		},
		metadata: []api.Metadata{{Name: testMetadataName, Value: testValue}},
		access: []api.Access{
			{User: api.User{Name: testUser, Zone: testZone}, Permission: testOwnPermission},
		},
	}

	left := make(chan *object, 1)
	left <- &object{source: testCopySrc, target: testCopyDest, info: sourceRecord}

	close(left)

	targetRecord := &testCopyRecord{
		DataObject: &api.DataObject{
			Path: testCopyDest,
			Replicas: []api.Replica{{
				Size:       int64(len(testStr)),
				ModifiedAt: tc.targetModTime,
			}},
		},
	}

	right := make(chan *object, 1)
	right <- &object{source: testCopyDest, target: testCopyDest, info: targetRecord}

	close(right)

	queue := make(chan Task, 2)

	err := Synchronize(t.Context(), left, right, queue, SynchronizeOptions{
		CompareChecksums:                 tc.compareChecksums,
		SyncModTimeForIdenticalChecksums: tc.syncModificationTime,
		ResyncMetadata:                   true,
		CopyMetadata:                     true,
		CopyACL:                          true,
		ChecksumVerify: func(context.Context, string, string, os.FileInfo, os.FileInfo) ([]byte, []byte, error) {
			return []byte(testSha), []byte(testSha), nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	assertSynchronizedMetadataTasks(t, queue, tc)
}

func assertSynchronizedMetadataTasks(t *testing.T, queue <-chan Task, tc synchronizeMetadataCase) {
	t.Helper()

	var metadata, modificationTime, foundMetadata, foundACL bool

	for task := range queue {
		metadata = metadata || task.Action == SyncMetadata
		modificationTime = modificationTime || task.Action == SetModificationTime
		foundMetadata = foundMetadata || task.Action == SyncMetadata && len(task.Attributes.MetadataToAdd) == 1
		foundACL = foundACL || task.Action == SyncMetadata && len(task.Attributes.AccessToAdd) == 1

		if task.Action != SyncMetadata && task.Action != SetModificationTime {
			t.Fatalf("unexpected task for identical objects: %+v", task)
		}
	}

	if !metadata {
		t.Error("identical object did not schedule metadata and ACL synchronization")
	}

	if !foundMetadata || !foundACL {
		t.Error("metadata synchronization task did not include the attribute changes")
	}

	if modificationTime != tc.wantModificationTime {
		t.Errorf("modification-time task = %t, want %t", modificationTime, tc.wantModificationTime)
	}
}

func TestDownloadDirDryRunExistingTarget(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	for _, local := range []string{".", "./", dir, dir + "/"} {
		t.Run(local, func(t *testing.T) {
			writeIdenticalLocalFile(t)

			conn := &api.MockConn{}
			conn.AddResponses(responses)

			var output bytes.Buffer

			worker := New(mockTransferAPI(conn), nil, Options{
				DryRun:          true,
				Output:          &output,
				ProgressHandler: func(context.Context, Progress) {},
			})
			worker.DownloadDir(t.Context(), local, "/test", Flags{})

			if err := worker.Wait(); err != nil {
				t.Fatal(err)
			}

			if output.Len() != 0 {
				t.Fatalf("unchanged directory has dry-run actions: %q", output.String())
			}
		})
	}
}

func TestComputeLocalDirPaths(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	if err := os.Mkdir("sub", 0o700); err != nil {
		t.Fatal(err)
	}

	for _, local := range []string{".", "./", dir, dir + "/", "./sub/.."} {
		for _, tc := range []struct {
			name     string
			existing bool
			upload   bool
		}{
			{name: "download missing"},
			{name: "download existing", existing: true},
			{name: "upload existing", existing: true, upload: true},
		} {
			t.Run(local+"/"+tc.name, func(t *testing.T) {
				testComputeLocalDirPaths(t, local, tc.existing, tc.upload)
			})
		}
	}
}

func testComputeLocalDirPaths(t *testing.T, local string, existing, upload bool) {
	t.Helper()

	if existing {
		writeIdenticalLocalFile(t)
	}

	conn := &api.MockConn{}
	conn.AddResponses(responses)

	worker := &Worker{
		IndexPool: mockTransferAPI(conn),
		options: Options{
			ErrorHandler: func(_ context.Context, _, _ string, err error) error {
				return err
			},
		},
	}
	queue := make(chan Task, 16)
	flags := Flags{Delete: true, IgnorePatterns: []string{"sub"}}

	var err error

	if upload {
		err = worker.ComputeUploadDir(t.Context(), local, "/test", queue, flags)
	} else {
		err = worker.ComputeDownloadDir(t.Context(), "/test", local, queue, flags)
	}

	if err != nil {
		t.Fatal(err)
	}

	if existing {
		if len(queue) != 0 {
			t.Fatalf("identical file scheduled for modification: %+v", <-queue)
		}

		return
	}

	if len(queue) != 1 {
		t.Fatalf("got %d tasks, want one download", len(queue))
	}

	task := <-queue
	if task.Action != TransferFile || task.Source != testFile1 ||
		task.Target != filepath.Join(local, "file1") || task.Size != int64(len(testStr)) {
		t.Errorf("unexpected download task: %+v", task)
	}
}

func writeIdenticalLocalFile(t *testing.T) {
	t.Helper()

	if err := os.WriteFile("file1", []byte(testStr), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := os.Remove("file1"); err != nil {
			t.Error(err)
		}
	})

	modTime := time.Unix(10000, 0)
	if err := os.Chtimes("file1", modTime, modTime); err != nil {
		t.Fatal(err)
	}
}
