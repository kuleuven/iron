package transfer_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kuleuven/iron"
	"github.com/kuleuven/iron/tests"
	"github.com/kuleuven/iron/transfer"
)

// TestIntegrationWorkerBasicUploadStream tests uploading data via stream.
// This verifies that the worker can transfer data from a reader to remote storage.
func TestIntegrationWorkerBasicUploadStream(t *testing.T) {
	tests.WithClients(t, func(t *testing.T, client *iron.Client, workdir string) {
		testCollection := tests.CreateTestCollection(t, client.API, workdir)
		t.Cleanup(func() {
			tests.RemoveTestCollection(t, client.API, testCollection)
		})

		// Test upload via stream
		testData := "Hello from integration test"

		err := client.RunWorker(transfer.Options{}, func(worker *transfer.Worker) {
			remotePath := testCollection + "/test_file.txt"
			worker.FromStream(t.Context(), "test_file.txt", strings.NewReader(testData), remotePath, false,
				transfer.Flags{})

			if err := worker.Wait(); err != nil {
				t.Errorf("failed to upload: %v", err)
			}
		})
		if err != nil {
			t.Fatalf("failed to run worker: %v", err)
		}
	})
}

// TestIntegrationWorkerRemoteCopy tests copying a file between two remote locations.
// This verifies that the worker can perform remote-to-remote transfers.
func TestIntegrationWorkerRemoteCopy(t *testing.T) {
	tests.WithClients(t, func(t *testing.T, client *iron.Client, workdir string) {
		sourceCollection := tests.CreateTestCollection(t, client.API, workdir)
		destCollection := tests.CreateTestCollection(t, client.API, workdir)

		t.Cleanup(func() {
			tests.RemoveTestCollection(t, client.API, sourceCollection)
			tests.RemoveTestCollection(t, client.API, destCollection)
		})

		// Create a test file in source collection
		testData := "Test data for remote copy"

		err := client.RunWorker(transfer.Options{
			MaxThreads: 1,
		}, func(worker *transfer.Worker) {
			sourcePath := sourceCollection + "/source_file.txt"
			worker.FromStream(t.Context(), "source_file.txt", strings.NewReader(testData), sourcePath, false,
				transfer.Flags{})

			if err := worker.Wait(); err != nil {
				t.Errorf("failed to upload source file: %v", err)
			}
		})
		if err != nil {
			t.Fatalf("failed to run worker for upload: %v", err)
		}

		// Copy the file to destination collection
		err = client.RunWorker(transfer.Options{
			MaxThreads: 2,
		}, func(worker *transfer.Worker) {
			worker.RemoteIndexPool = worker.IndexPool
			worker.RemoteTransferPool = worker.TransferPool

			sourcePath := sourceCollection + "/source_file.txt"
			destPath := destCollection + "/copied_file.txt"
			worker.RemoteCopy(t.Context(), sourcePath, destPath, transfer.Flags{})

			if err := worker.Wait(); err != nil {
				t.Errorf("failed to copy remotely: %v", err)
			}
		})
		if err != nil {
			t.Fatalf("failed to run worker for copy: %v", err)
		}
	})
}

// TestIntegrationUploadDir tests uploading a directory to remote storage with various checksum options.
// It verifies:
// 1. Directory upload with integrity checksums enabled
// 2. Reupload with CompareChecksums flag that skips files already present with matching checksums
func TestIntegrationUploadDir(t *testing.T) { //nolint:funlen
	tests.WithClients(t, func(t *testing.T, client *iron.Client, workdir string) {
		testCollection := tests.CreateTestCollection(t, client.API, workdir)
		t.Cleanup(func() {
			tests.RemoveTestCollection(t, client.API, testCollection)
		})

		// Create temporary local directory with test files
		localDir := t.TempDir()
		testFile1 := filepath.Join(localDir, "file1.txt")
		testFile2 := filepath.Join(localDir, "file2.txt")
		testSubDir := filepath.Join(localDir, "subdir")

		if err := os.Mkdir(testSubDir, 0o755); err != nil {
			t.Fatalf("failed to create subdirectory: %v", err)
		}

		testFile3 := filepath.Join(testSubDir, "file3.txt")

		if err := os.WriteFile(testFile1, []byte("Content of file 1"), 0o644); err != nil {
			t.Fatalf("failed to write test file 1: %v", err)
		}

		if err := os.WriteFile(testFile2, []byte("Content of file 2"), 0o644); err != nil {
			t.Fatalf("failed to write test file 2: %v", err)
		}

		if err := os.WriteFile(testFile3, []byte("Content of file 3 in subdir"), 0o644); err != nil {
			t.Fatalf("failed to write test file 3: %v", err)
		}

		remoteDir := testCollection + "/uploaded_dir"

		// First upload: transfer all files with integrity checksums enabled
		err := client.RunWorker(transfer.Options{
			MaxThreads: 2,
		}, func(worker *transfer.Worker) {
			flags := transfer.Flags{
				IntegrityChecksums: true,
			}
			worker.UploadDir(t.Context(), localDir, remoteDir, flags)

			if err := worker.Wait(); err != nil {
				t.Errorf("failed to upload directory: %v", err)
			}
		})
		if err != nil {
			t.Fatalf("failed to run worker for first upload: %v", err)
		}

		// Second upload: with CompareChecksums flag, should skip all files
		// (all files already exist remotely with matching checksums)
		err = client.RunWorker(transfer.Options{
			MaxThreads: 2,
		}, func(worker *transfer.Worker) {
			flags := transfer.Flags{
				CompareChecksums: true,
			}
			worker.UploadDir(t.Context(), localDir, remoteDir, flags)

			if err := worker.Wait(); err != nil {
				t.Errorf("failed to upload directory with CompareChecksums: %v", err)
			}
		})
		if err != nil {
			t.Fatalf("failed to run worker for second upload: %v", err)
		}
	})
}

// TestIntegrationDownloadDir tests downloading a directory from remote storage with various checksum options.
// It verifies:
// 1. Directory download with integrity checksums enabled
// 2. Redownload with CompareChecksums flag that skips files already present with matching checksums
func TestIntegrationDownloadDir(t *testing.T) { //nolint:funlen
	tests.WithClients(t, func(t *testing.T, client *iron.Client, workdir string) {
		testCollection := tests.CreateTestCollection(t, client.API, workdir)
		t.Cleanup(func() {
			tests.RemoveTestCollection(t, client.API, testCollection)
		})

		// First, upload some test files to remote
		testData1 := "Remote file content 1"
		testData2 := "Remote file content 2"

		err := client.RunWorker(transfer.Options{
			MaxThreads: 2,
		}, func(worker *transfer.Worker) {
			remotePath1 := testCollection + "/remote_file1.txt"
			remotePath2 := testCollection + "/remote_file2.txt"

			worker.FromStream(t.Context(), "remote_file1.txt", strings.NewReader(testData1), remotePath1, false, transfer.Flags{})
			worker.FromStream(t.Context(), "remote_file2.txt", strings.NewReader(testData2), remotePath2, false, transfer.Flags{})

			if err := worker.Wait(); err != nil {
				t.Errorf("failed to upload test files: %v", err)
			}
		})
		if err != nil {
			t.Fatalf("failed to run worker for upload: %v", err)
		}

		// Create download directory
		localDir := t.TempDir()

		// First download: transfer all files with integrity checksums enabled
		err = client.RunWorker(transfer.Options{
			MaxThreads: 2,
		}, func(worker *transfer.Worker) {
			flags := transfer.Flags{
				IntegrityChecksums: true,
			}
			worker.DownloadDir(t.Context(), localDir, testCollection, flags)

			if err := worker.Wait(); err != nil {
				t.Errorf("failed to download directory: %v", err)
			}
		})
		if err != nil {
			t.Fatalf("failed to run worker for first download: %v", err)
		}

		// Verify the downloaded files exist
		file1Path := filepath.Join(localDir, "remote_file1.txt")
		file2Path := filepath.Join(localDir, "remote_file2.txt")

		if _, err := os.Stat(file1Path); err != nil {
			t.Errorf("downloaded file 1 not found: %v", err)
		}

		if _, err := os.Stat(file2Path); err != nil {
			t.Errorf("downloaded file 2 not found: %v", err)
		}

		// Second download: with CompareChecksums flag, should skip all files
		// (all files already exist locally with matching checksums)
		err = client.RunWorker(transfer.Options{
			MaxThreads: 2,
		}, func(worker *transfer.Worker) {
			flags := transfer.Flags{
				CompareChecksums: true,
			}
			worker.DownloadDir(t.Context(), localDir, testCollection, flags)

			if err := worker.Wait(); err != nil {
				t.Errorf("failed to download directory with CompareChecksums: %v", err)
			}
		})
		if err != nil {
			t.Fatalf("failed to run worker for second download: %v", err)
		}
	})
}
