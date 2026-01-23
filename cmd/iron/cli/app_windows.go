//go:build windows

package cli

import (
	"os"
	"os/user"
	"strings"
)

func uidOfFile(_ os.FileInfo) int {
	// Calculate a fake uid as python-irodsclient does
	u, err := user.Current()
	if err != nil {
		return 1000
	}

	parts := strings.Split(u.Username, `\`)

	return strToOrd(parts[len(parts)-1])
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
