package cli

import (
	"context"
	"io"
	"strings"

	"github.com/kuleuven/iron/api"
)

func (a *App) Path(path string) string {
	return a.PathIn(path, a.Workdir)
}

func (a *App) PathIn(path, workdir string) string {
	if path == "" || path == "." {
		return workdir
	}

	if !strings.HasPrefix(path, "/") {
		path = "/" + path

		if workdir != "/" {
			path = workdir + "/" + path
		}
	}

	path = strings.TrimSuffix(path, "/")

	if path == "" {
		return "/"
	}

	parts := strings.Split(path, "/")
	kept := make([]string, 0, len(parts))

	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}

		if part == ".." {
			if len(kept) > 0 {
				kept = kept[:len(kept)-1]
			}

			continue
		}

		kept = append(kept, part)
	}

	return "/" + strings.Join(kept, "/")
}

func Name(path string) string {
	_, name := api.Split(path)

	return name
}

// CopyBuffer reads from src into buf and writes to dst.
// Implementation taken from io.CopyBuffer, but removed the ReadFrom and WriteTo checks
func CopyBuffer(ctx context.Context, dst io.Writer, src io.Reader, buf []byte) (int64, error) {
	var written int64

	for {
		if ctx.Err() != nil {
			return written, ctx.Err()
		}

		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])

			written += int64(nw)
			if ew != nil {
				return written, ew
			}

			if nr != nw {
				return written, io.ErrShortWrite
			}
		}

		if er == io.EOF {
			return written, nil
		}

		if er != nil {
			return written, er
		}
	}
}
