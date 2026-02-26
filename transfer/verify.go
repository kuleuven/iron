package transfer

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

// Verify checks the checksum of a local file against the checksum of a remote file
func VerifyLocalToRemote(a *api.API, progressHandler func(Progress)) func(ctx context.Context, local, remote string, localInfo, remoteInfo os.FileInfo) ([]byte, []byte, error) {
	return func(ctx context.Context, local, remote string, localInfo, remoteInfo os.FileInfo) ([]byte, []byte, error) {
		g, ctx := errgroup.WithContext(ctx)

		var localHash, remoteHash []byte

		g.Go(func() error {
			var err error

			localHash, err = Sha256Checksum(ctx, local)

			return err
		})

		g.Go(func() error {
			// Try to get checksum from remoteInfo
			if checksum, ok := parseChecksum(remoteInfo); ok {
				remoteHash = checksum

				return nil
			}

			if progressHandler != nil {
				progressHandler(Progress{
					Action: ComputeChecksum,
					Label:  local,
				})
			}

			var err error

			remoteHash, err = a.Checksum(ctx, remote, false)

			return err
		})

		if err := g.Wait(); err != nil {
			return nil, nil, err
		}

		if !bytes.Equal(localHash, remoteHash) {
			return localHash, remoteHash, fmt.Errorf("%w: local: %s remote: %s", ErrChecksumMismatch, base64.StdEncoding.EncodeToString(localHash), base64.StdEncoding.EncodeToString(remoteHash))
		}

		return localHash, remoteHash, nil
	}
}

func VerifyRemoteToLocal(a *api.API, progressHandler func(Progress)) func(ctx context.Context, local, remote string, localInfo, remoteInfo os.FileInfo) ([]byte, []byte, error) {
	return func(ctx context.Context, local, remote string, localInfo, remoteInfo os.FileInfo) ([]byte, []byte, error) {
		l, r, err := VerifyLocalToRemote(a, progressHandler)(ctx, local, remote, localInfo, remoteInfo)

		return r, l, err
	}
}

// VerifyRemote checks the checksum of two remote files
func VerifyRemoteToRemote(a *api.API, progressHandler func(Progress)) func(ctx context.Context, remote1, remote2 string, remote1Info, remote2Info os.FileInfo) ([]byte, []byte, error) {
	return func(ctx context.Context, remote1, remote2 string, remote1Info, remote2Info os.FileInfo) ([]byte, []byte, error) {
		g, ctx := errgroup.WithContext(ctx)

		var remote1Hash, remote2Hash []byte

		g.Go(func() error {
			// Try to get checksum from remoteInfo
			if checksum, ok := parseChecksum(remote1Info); ok {
				remote1Hash = checksum

				return nil
			}

			if progressHandler != nil {
				progressHandler(Progress{
					Action: ComputeChecksum,
					Label:  remote1,
				})
			}

			var err error

			remote1Hash, err = a.Checksum(ctx, remote1, false)

			return err
		})

		g.Go(func() error {
			// Try to get checksum from remoteInfo
			if checksum, ok := parseChecksum(remote2Info); ok {
				remote2Hash = checksum

				return nil
			}

			if progressHandler != nil {
				progressHandler(Progress{
					Action: ComputeChecksum,
					Label:  remote2,
				})
			}

			var err error

			remote2Hash, err = a.Checksum(ctx, remote2, false)

			return err
		})

		if err := g.Wait(); err != nil {
			return nil, nil, err
		}

		if !bytes.Equal(remote1Hash, remote2Hash) {
			return remote1Hash, remote2Hash, fmt.Errorf("%w: remote1: %s remote2: %s", ErrChecksumMismatch, base64.StdEncoding.EncodeToString(remote1Hash), base64.StdEncoding.EncodeToString(remote2Hash))
		}

		return remote1Hash, remote2Hash, nil
	}
}

const shaPrefix = "sha2:"

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
