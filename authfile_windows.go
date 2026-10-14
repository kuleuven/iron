//go:build windows

package iron

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
