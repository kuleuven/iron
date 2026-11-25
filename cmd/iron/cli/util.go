package cli

import (
	"strings"

	"github.com/kuleuven/iron/api"
)

func (a *App) Path(path string) string {
	return a.PathIn(path, a.Workdir)
}

func (a *App) PathIn(path, workdir string) string {
	path = a.Unescape(path)

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

	if a.Literal {
		return path
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
