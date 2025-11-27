//go:build !windows

package cli

import (
	"fmt"
	"os"
	"syscall"

	linuxproc "github.com/c9s/goprocinfo/linux"
)

func uid(fi os.FileInfo) int {
	if s, ok := fi.Sys().(*syscall.Stat_t); ok {
		return int(s.Uid)
	}

	return 0
}

func findParentOf(pid int) (int, error) {
	stat, err := linuxproc.ReadProcessStatus(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0, err
	}

	return int(stat.PPid), nil
}
