package scramble

import (
	"bytes"
	"crypto/md5" //nolint:gosec
	"encoding/hex"
	"math/rand"
	"strings"
)

var wheel = []byte{
	'0', '1', '2', '3', '4', '5', '6', '7', '8', '9',
	'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M',
	'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z',
	'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm',
	'n', 'o', 'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z',
	'!', '"', '#', '$', '%', '&', '\'', '(', ')', '*', '+', ',', '-', '.', '/',
}

// Very secret key that is part of the public cpp code of irods
const (
	scramblePadding    = "1gCBizHWbwIYyWLoysGzTe6SyzqFKMniZX05faZHWAwQKXf6Fs"
	v2Prefix           = "A.ObfV2"
	defaultPasswordKey = "a9_3fker" //nolint:gosec
)

// ObfuscateNewPassword obfuscates new password for changing
func ObfuscateNewPassword(newPassword, oldPassword, signature string) string {
	pwdLen := len(newPassword)

	if pwdLen > maxPasswordLength {
		newPassword = newPassword[:maxPasswordLength]
	}

	lencopy := maxPasswordLength - 10 - pwdLen

	if lencopy > 15 {
		// protection
		if lencopy > len(scramblePadding) {
			lencopy = len(scramblePadding)
		}

		newPassword += scramblePadding[:lencopy]
	}

	return ScrambleV2(newPassword, oldPassword, signature)
}

// ScrambleV2 scrambles string (ver2)
func ScrambleV2(newPassword, oldPassword, signature string) string {
	v2prefixLen := len(v2Prefix)

	if v2prefixLen > 10 {
		v2prefixLen = 10
	}

	if len(newPassword) > 150 {
		newPassword = newPassword[:150]
	}

	if len(oldPassword) > 90 {
		oldPassword = oldPassword[:90]
	}

	if len(signature) > 100 {
		signature = signature[:100]
	}

	toScramble := string(wheel[rand.Intn(62)]) + v2Prefix[1:v2prefixLen] + newPassword //nolint:gosec

	keyBuf := bytes.Buffer{}
	keyBuf.WriteString(oldPassword)
	keyBuf.WriteString(signature)

	for i := len(oldPassword) + len(signature); i < 100; i++ {
		keyBuf.WriteByte(0)
	}

	hashKeyBytes := md5.Sum(keyBuf.Bytes()) //nolint:gosec
	hashedKey := hex.EncodeToString(hashKeyBytes[:])

	return Scramble(toScramble, hashedKey, "", true)
}

// Scramble scrambles string
func Scramble(toScramble, key, scramblePrefix string, blockChaining bool) string {
	if key == "" {
		key = defaultPasswordKey
	}

	encoderRing := GetEncoderRing(key)
	chain := 0

	scrambledStr := strings.Builder{}

	for p := range len(toScramble) {
		encoderRingIndex := p % 61
		k := int(encoderRing[encoderRingIndex])

		// The character is only encoded if it's one of the ones in wheel
		foundInWheel := false

		for wheelIndex, wheelChar := range wheel {
			if wheelChar != toScramble[p] {
				continue
			}

			// index of the target character in wheel
			newWheelIndex := (wheelIndex + k + chain) % len(wheel)
			scrambledStr.WriteByte(wheel[newWheelIndex])

			if blockChaining {
				chain = int(wheel[newWheelIndex]) & 0xff
			}

			foundInWheel = true

			break
		}

		if !foundInWheel {
			scrambledStr.WriteByte(toScramble[p])
		}
	}

	return scramblePrefix + scrambledStr.String()
}

// GetEncoderRing returns encoder ring
func GetEncoderRing(key string) []byte {
	keyBuf := make([]byte, 100)

	if len(key) > 100 {
		key = key[:100]
	}

	copy(keyBuf[0:], key)

	buffer := make([]byte, 64)

	// first
	hash := md5.Sum(keyBuf) //nolint:gosec

	copy(buffer, hash[:])

	// second
	hash = md5.Sum(buffer[:16]) //nolint:gosec

	copy(buffer[16:], hash[:])

	// two of third
	hash = md5.Sum(buffer[:32]) //nolint:gosec

	copy(buffer[32:], hash[:])
	copy(buffer[48:], hash[:])

	return buffer
}
