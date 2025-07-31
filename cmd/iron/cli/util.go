package cli

import (
	"strings"

	"github.com/kuleuven/iron/api"
)

func (a *App) Path(path string) string {
	if path == "" || path == "." {
		return a.Workdir
	}

	if !strings.HasPrefix(path, "/") {
		path = "/" + path

		if a.Workdir != "/" {
			path = a.Workdir + "/" + path
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
