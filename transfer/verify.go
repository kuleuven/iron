package transfer

// Verification functions for local and remote files using checksums.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/kuleuven/iron/api"
	"golang.org/x/sync/errgroup"
)

var ErrChecksumMismatch = errors.New("checksum mismatch")

// CompareFunc checks the checksums of the local and remote file and returns nil if they match,
// ErrChecksumMismatch if they don't match, or an error if the verification failed.
// The checksum of the source file is returned along with the checksum of the target file.
type CompareFunc func(ctx context.Context, source, target string, sourceInfo, targetInfo os.FileInfo) ([]byte, []byte, error)

// VerifyLocalToRemote checks the checksum of a local file against the checksum of a remote file.
func VerifyLocalToRemote(a *api.API, progressHandler func(Progress)) CompareFunc {
	return func(ctx context.Context, local, remote string, localInfo, remoteInfo os.FileInfo) ([]byte, []byte, error) {
		sourceChecksum := func(ctx context.Context) ([]byte, error) {
			return Sha256Checksum(ctx, local)
		}

		targetChecksum := func(ctx context.Context) ([]byte, error) {
			return parseOrComputeChecksum(ctx, a, progressHandler, remote, remoteInfo)
		}

		return compareChecksums(ctx, sourceChecksum, targetChecksum)
	}
}

func VerifyRemoteToLocal(a *api.API, progressHandler func(Progress)) CompareFunc {
	return func(ctx context.Context, remote, local string, remoteInfo, localInfo os.FileInfo) ([]byte, []byte, error) {
		sourceChecksum := func(ctx context.Context) ([]byte, error) {
			return parseOrComputeChecksum(ctx, a, progressHandler, local, localInfo)
		}

		targetChecksum := func(ctx context.Context) ([]byte, error) {
			return Sha256Checksum(ctx, remote)
		}

		return compareChecksums(ctx, sourceChecksum, targetChecksum)
	}
}

// VerifyRemote checks the checksum of two remote files
func VerifyRemoteToRemote(a, b *api.API, progressHandler func(Progress)) CompareFunc {
	return func(ctx context.Context, remote1, remote2 string, remote1Info, remote2Info os.FileInfo) ([]byte, []byte, error) {
		return compareChecksums(ctx, func(ctx context.Context) ([]byte, error) {
			return parseOrComputeChecksum(ctx, a, progressHandler, remote1, remote1Info)
		}, func(ctx context.Context) ([]byte, error) {
			return parseOrComputeChecksum(ctx, b, progressHandler, remote2, remote2Info)
		})
	}
}

const shaPrefix = "sha2:"

// parseChecksum attempts to extract the sha256 checksum from the given os.FileInfo.
// It returns the decoded checksum and true if successful, or nil and false otherwise.
func parseChecksum(info os.FileInfo) ([]byte, bool) {
	if info == nil {
		return nil, false
	}

	obj, ok := info.Sys().(*api.DataObject)
	if !ok {
		return nil, false
	}

	for _, replica := range obj.Replicas {
		if replica.Status != "1" {
			continue
		}

		suffix, ok := strings.CutPrefix(replica.Checksum, shaPrefix)
		if !ok {
			continue
		}

		if decoded, err := base64.StdEncoding.DecodeString(suffix); err == nil {
			return decoded, true
		}
	}

	return nil, false
}

func parseOrComputeChecksum(ctx context.Context, a *api.API, progressHandler func(Progress), path string, info os.FileInfo) ([]byte, error) {
	// Try to get checksum from remoteInfo
	if checksum, ok := parseChecksum(info); ok {
		return checksum, nil
	}

	if progressHandler != nil {
		progressHandler(Progress{
			Action: ComputeChecksum,
			Label:  path,
		})
	}

	return a.Checksum(ctx, path, false)
}

// compareChecksums concurrently computes the local and remote checksums using the provided functions and compares them.
// It returns the local and remote checksums if they match, or an error if they differ or if any computation fails.
func compareChecksums(ctx context.Context, sourceChecksum, targetChecksum func(context.Context) ([]byte, error)) ([]byte, []byte, error) {
	g, ctx := errgroup.WithContext(ctx)

	var sourceHash, targetHash []byte

	g.Go(func() error {
		var err error

		sourceHash, err = sourceChecksum(ctx)

		return err
	})

	g.Go(func() error {
		var err error

		targetHash, err = targetChecksum(ctx)

		return err
	})

	if err := g.Wait(); err != nil {
		return nil, nil, err
	}

	if !bytes.Equal(sourceHash, targetHash) {
		return sourceHash, targetHash, fmt.Errorf("%w: source: %s target: %s", ErrChecksumMismatch, base64.StdEncoding.EncodeToString(sourceHash), base64.StdEncoding.EncodeToString(targetHash))
	}

	return sourceHash, targetHash, nil
}

// Sha256Checksum computes the sha256 checksum of a local file in a goroutine, so that it can be canceled with the context.
// The checksum is computed in a goroutine, so that it can be canceled with the context.
// The function returns the checksum as a byte slice, or an error if either the context is canceled or the checksum computation fails.
func Sha256Checksum(ctx context.Context, local string) ([]byte, error) {
	r, err := os.Open(local)
	if err != nil {
		return nil, err
	}

	defer r.Close()

	// Compute sha256 hash
	h := sha256.New()

	done := make(chan error, 1)

	go func() {
		defer close(done)

		_, err = io.Copy(h, r)

		done <- err
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-done:
		if err != nil {
			return nil, err
		}

		localHash := h.Sum(nil)

		return localHash, nil
	}
}
