//go:build !windows

package cli

import (
	"os"
	"syscall"
)

func uid(fi os.FileInfo) int {
	if s, ok := fi.Sys().(*syscall.Stat_t); ok {
		return int(s.Uid)
	}

	return 0
}
