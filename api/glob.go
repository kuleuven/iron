package api

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/kuleuven/iron/msg"
)

// Glob finds records matching the given glob pattern, and calls the given function for each match.
// The pattern is matched component-by-component using filepath.Match against directory entries
// obtained via ListSubCollections and ListDataObjectsInCollection.
//
// If the pattern is absolute, absolute paths are passed to walkFn.
// If the pattern is relative, the search is within root and relative paths are passed to walkFn.
// If the function returns an error or SkipAll, the traversal is stopped.
func (api *API) Glob(ctx context.Context, root, pattern string, walkFn WalkFunc) error {
	abs := strings.HasPrefix(pattern, "/")

	var absPattern string
	if abs {
		absPattern = pattern
	} else {
		absPattern = root + "/" + pattern
	}

	dir, parts := splitGlobPrefix(absPattern)

	if len(parts) == 0 {
		// No wildcards - exact path lookup
		p := globPath(root, dir, abs)

		rec, err := api.GetRecord(ctx, dir)
		if err != nil {
			err = walkFn(p, nil, err)
		} else {
			err = walkFn(p, rec, nil)
		}

		if err == SkipAll {
			return nil
		}

		return err
	}

	err := api.globMatch(ctx, root, dir, parts, abs, walkFn)
	if err == SkipAll {
		return nil
	}

	return err
}

func (api *API) globMatch(ctx context.Context, root, dir string, parts []string, abs bool, walkFn WalkFunc) error {
	pattern := parts[0]
	remaining := parts[1:]

	// Static path component — descend without querying
	if !hasMeta(pattern) && len(remaining) > 0 {
		return api.globMatch(ctx, root, dir+"/"+pattern, remaining, abs, walkFn)
	}

	likePattern := globToLike(pattern)

	subcols, err := api.ListCollections(ctx,
		Equal(msg.ICAT_COLUMN_COLL_PARENT_NAME, dir),
		Like(msg.ICAT_COLUMN_COLL_NAME, dir+"/"+likePattern),
	)
	if err != nil {
		return err
	}

	for i := range subcols {
		if matched, _ := filepath.Match(pattern, subcols[i].Name()); !matched { //nolint:errcheck
			continue
		}

		if len(remaining) > 0 {
			err = api.globMatch(ctx, root, subcols[i].Path, remaining, abs, walkFn)
		} else {
			err = walkFn(globPath(root, subcols[i].Path, abs), &record{FileInfo: &subcols[i]}, nil)
		}

		if err != nil {
			return err
		}
	}

	if len(remaining) > 0 {
		return nil
	}

	objects, err := api.ListDataObjects(ctx,
		Equal(msg.ICAT_COLUMN_COLL_NAME, dir),
		Like(msg.ICAT_COLUMN_DATA_NAME, likePattern),
	)
	if err != nil {
		return err
	}

	for i := range objects {
		if matched, _ := filepath.Match(pattern, objects[i].Name()); !matched { //nolint:errcheck
			continue
		}

		if err := walkFn(globPath(root, objects[i].Path, abs), &record{FileInfo: &objects[i]}, nil); err != nil {
			return err
		}
	}

	return nil
}

// hasMeta reports whether the given string contains any glob meta characters.
func hasMeta(s string) bool {
	return strings.ContainsAny(s, `*?[\`)
}

// globToLike converts a glob pattern to a SQL LIKE pattern.
// Glob metacharacters are translated as follows:
//   - * → %
//   - ? → _
//   - [...] → % (character classes cannot be expressed in LIKE; filepath.Match refines the result)
//   - Literal % and _ in the glob are escaped with a backslash.
func globToLike(pattern string) string {
	var b strings.Builder

	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*':
			b.WriteByte('%')
		case '?':
			b.WriteByte('_')
		case '[':
			// Skip entire character class; replace with % wildcard
			b.WriteByte('%')

			for i++; i < len(pattern) && pattern[i] != ']'; i++ {
			}
		case '\\':
			// Escaped character in glob — emit literally, but escape if it is a LIKE metachar
			if i+1 < len(pattern) {
				i++

				writeLikeLiteral(&b, pattern[i])
			}
		case '%', '_':
			b.WriteByte('\\')
			b.WriteByte(pattern[i])
		default:
			b.WriteByte(pattern[i])
		}
	}

	return b.String()
}

func writeLikeLiteral(b *strings.Builder, ch byte) {
	if ch == '%' || ch == '_' {
		b.WriteByte('\\')
	}

	b.WriteByte(ch)
}

// splitGlobPrefix splits an absolute glob pattern into a static directory prefix
// (containing no glob meta characters) and the remaining pattern parts.
func splitGlobPrefix(absPattern string) (string, []string) {
	components := strings.Split(absPattern, "/")

	i := 0
	for i < len(components) {
		if hasMeta(components[i]) {
			break
		}

		i++
	}

	dir := strings.Join(components[:i], "/")
	if dir == "" {
		dir = "/"
	}

	return dir, components[i:]
}

// globPath returns the path to pass to walkFn: absolute if the pattern was
// absolute, or relative to root otherwise.
func globPath(root, absPath string, abs bool) string {
	if abs {
		return absPath
	}

	rel := strings.TrimPrefix(absPath, root)
	rel = strings.TrimPrefix(rel, "/")

	if rel == "" {
		return "."
	}

	return rel
}
