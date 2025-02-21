package scramble

import (
	"testing"
)

func TestScramble(t *testing.T) {
	ObfuscateNewPassword("pass", "oldPass", "signature")
}
