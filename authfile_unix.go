//go:build !windows

package iron

import (
	"os"
	"syscall"
)

func uidOfFile(fi os.FileInfo) int {
	if s, ok := fi.Sys().(*syscall.Stat_t); ok {
		return int(s.Uid)
	}

	return 0
}
