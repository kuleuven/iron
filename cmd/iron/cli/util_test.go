package cli

import (
	"testing"
)

//nolint:funlen,goconst
func TestPathWithEscapes(t *testing.T) {
	tests := []struct {
		name     string
		literal  bool
		path     string
		workdir  string
		expected string
	}{
		// Basic paths without escapes
		{
			name:     "simple path",
			literal:  false,
			path:     "/zone/home/user/file",
			workdir:  "/zone/home/user",
			expected: "/zone/home/user/file",
		},
		// Paths with escape sequences - should be unescaped
		{
			name:     "carriage return in name",
			literal:  false,
			path:     "/zone/home/user/file\\rname",
			workdir:  "/zone/home/user",
			expected: "/zone/home/user/file\rname",
		},
		{
			name:     "hex escape in name",
			literal:  false,
			path:     "/zone/home/user/file\\x0dname",
			workdir:  "/zone/home/user",
			expected: "/zone/home/user/file\rname",
		},
		{
			name:     "newline in name",
			literal:  false,
			path:     "/zone/home/user/file\\nname",
			workdir:  "/zone/home/user",
			expected: "/zone/home/user/file\nname",
		},
		// With --literal flag - escapes should NOT be processed
		{
			name:     "literal flag disables escapes",
			literal:  true,
			path:     "/zone/home/user/file\\rname",
			workdir:  "/zone/home/user",
			expected: "/zone/home/user/file\\rname",
		},
		// Relative paths with escapes
		{
			name:     "relative path with escape",
			literal:  false,
			path:     "file\\rname",
			workdir:  "/zone/home/user",
			expected: "/zone/home/user/file\rname",
		},
		// Current directory references
		{
			name:     "dot with escape in workdir",
			literal:  false,
			path:     ".",
			workdir:  "/zone/home/user/file\\rdir",
			expected: "/zone/home/user/file\\rdir",
		},
		// Multiple escapes in path
		{
			name:     "multiple escapes",
			literal:  false,
			path:     "/zone/home/user/file\\rname\\nmore",
			workdir:  "/zone/home/user",
			expected: "/zone/home/user/file\rname\nmore",
		},
		// Escaped backslash
		{
			name:     "escaped backslash",
			literal:  false,
			path:     "/zone/home/user/file\\\\name",
			workdir:  "/zone/home/user",
			expected: "/zone/home/user/file\\name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &App{
				Literal: tt.literal,
				Workdir: tt.workdir,
			}
			got := app.PathIn(tt.path, tt.workdir)

			if got != tt.expected {
				t.Errorf("PathIn(%q, %q) with Literal=%v = %q, want %q", tt.path, tt.workdir, tt.literal, got, tt.expected)
			}
		})
	}
}

//nolint:goconst
func TestPathNormalization(t *testing.T) {
	tests := []struct {
		name     string
		literal  bool
		path     string
		workdir  string
		expected string
	}{
		// Path normalization without escapes
		{
			name:     "parent directory reference",
			literal:  false,
			path:     "/zone/home/user/../user/file",
			workdir:  "/zone/home/user",
			expected: "/zone/home/user/file",
		},
		// With --literal flag, parent references are kept
		{
			name:     "literal flag keeps parent reference",
			literal:  true,
			path:     "/zone/home/user/../user/file",
			workdir:  "/zone/home/user",
			expected: "/zone/home/user/../user/file",
		},
		// Escape processing happens before normalization
		{
			name:     "escape then normalize",
			literal:  false,
			path:     "/zone/home/user/..\\x2fuser/file",
			workdir:  "/zone/home/user",
			expected: "/zone/home/user/file", // The \x2f becomes / and .. then .. is normalized away
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &App{
				Literal: tt.literal,
				Workdir: tt.workdir,
			}
			got := app.PathIn(tt.path, tt.workdir)

			if got != tt.expected {
				t.Errorf("PathIn(%q, %q) with Literal=%v = %q, want %q", tt.path, tt.workdir, tt.literal, got, tt.expected)
			}
		})
	}
}
