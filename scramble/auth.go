package scramble

import (
	"crypto/md5" //nolint:gosec
	"encoding/base64"
)

const (
	maxPasswordLength int = 50
	challengeLen      int = 64
	authResponseLen   int = 16
)

// GenerateAuthResponse generates an authentication response using the given
// challenge and password. The response is the MD5 hash of the first 64 bytes
// of the challenge and the padded password (padded to maxPasswordLength).
// The first 16 bytes of the hash are then base64 encoded to produce the
// final response string.
func GenerateAuthResponse(challenge []byte, password string) string {
	paddedPassword := make([]byte, maxPasswordLength)
	copy(paddedPassword, password)

	m := md5.New() //nolint:gosec
	m.Write(challenge[:challengeLen])
	m.Write(paddedPassword)
	encodedPassword := m.Sum(nil)

	// replace 0x00 to 0x01
	for idx := 0; idx < len(encodedPassword); idx++ {
		if encodedPassword[idx] == 0 {
			encodedPassword[idx] = 1
		}
	}

	return base64.StdEncoding.EncodeToString(encodedPassword[:authResponseLen])
}
