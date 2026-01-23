//go:build windows

package cli

import (
	"os"
	"os/user"
)

func uidOfFile(_ os.FileInfo) int {
	// Calculate a fake uid as python-irodsclient does
	u, err := user.Current()
	if err != nil {
		return 1000
	}

	return strToOrd(u.Username)
}

func strToOrd(s string) int {
	var sum int

	for _, r := range s {
		sum += int(r)
	}

	return sum
}

func findParentOf(_ int) (int, error) {
	return 0, os.ErrInvalid
}
