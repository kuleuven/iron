//go:build windows

package cli

import (
	"os"
)

func uid(_ os.FileInfo) int {
	return 1000
}

func findParentOf(_ int) (int, error) {
	return 0, os.ErrInvalid
}
