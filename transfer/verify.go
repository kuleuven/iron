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

	"github.com/kuleuven/iron/api"
	"golang.org/x/sync/errgroup"
)

var ErrChecksumMismatch = errors.New("checksum mismatch")

// Verify checks the checksum of a local file against the checksum of a remote file
func Verify(ctx context.Context, a *api.API, local, remote string) error {
	g, ctx := errgroup.WithContext(ctx)

	var localHash, remoteHash []byte

	g.Go(func() error {
		var err error

		localHash, err = Sha256Checksum(ctx, local)

		return err
	})

	g.Go(func() error {
		var err error

		remoteHash, err = a.Checksum(ctx, remote, false)

		return err
	})

	if err := g.Wait(); err != nil {
		return err
	}

	if !bytes.Equal(localHash, remoteHash) {
		return fmt.Errorf("%w: local: %s remote: %s", ErrChecksumMismatch, base64.StdEncoding.EncodeToString(localHash), base64.StdEncoding.EncodeToString(remoteHash))
	}

	return nil
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
