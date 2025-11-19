//go:build windows

package cli

import (
	"os"
)

func uid(_ os.FileInfo) int {
	return 1000
}
