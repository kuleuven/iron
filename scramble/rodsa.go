package scramble

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"time"
)

var seqList = []uint{
	0xD768B678,
	0xEDFDAF56,
	0x2420231B,
	0x987098D8,
	0xC1BDFEEE,
	0xF572341F,
	0x478DEF3A,
	0xA830D343,
	0x774DFA2A,
	0x6720731E,
	0x346FA320,
	0x6FFDF43A,
	0x7723A320,
	0xDF67D02E,
	0x86AD240A,
	0xE76D342E,
}

// Decode a password from a .irodsA file
func DecodeIrodsA(s []byte, uid int) (string, error) {
	if len(s) < 8 {
		return "", errors.New("string too short")
	}

	// This value lets us know which seq value to use
	// Referred to as "rval" in the C code
	seqIndex := int(s[6]) - int('e')
	if seqIndex < 0 || seqIndex >= len(seqList) {
		return "", errors.New("invalid seq index")
	}

	seq := seqList[seqIndex]

	// How much we bitshift seq by when we use it
	// Referred to as "addin_i" in the C code
	// Since we're skipping five bytes that are normally read,
	// we start at 15
	bitshift := 15

	// The first byte is a dot, the next five are literally irrelevant
	// garbage, and we already used the seventh one. The string to decode
	// starts at byte eight.
	encodedString := s[7:]

	var decodedString strings.Builder

	for _, c := range encodedString {
		if c == 0 {
			break
		}

		// How far this character is from the target character in wheel
		// Referred to as "add_in" in the C code
		offset := int(((seq >> bitshift) & 0x1F)) + (uid & 0xF5F)

		bitshift += 3
		if bitshift > 28 {
			bitshift = 0
		}

		// The character is only encoded if it's one of the ones in wheel
		if index := slices.Index(wheel, c); index >= 0 {
			// index of the target character in wheel
			wheelIndex := (index - offset) % len(wheel)

			if wheelIndex < 0 {
				wheelIndex += len(wheel)
			}

			decodedString.WriteByte(wheel[wheelIndex])
		} else {
			decodedString.WriteByte(c)
		}
	}

	return decodedString.String(), nil
}

// Encode passwords to store in the .irodsA file
func EncodeIrodsA(s string, uid int, mtime time.Time) []byte {
	// mtime & 65535 needs to be within 20 seconds of the
	// .irodsA file's mtime & 65535
	mtimeVal := mtime.Unix()

	// How much we bitshift seq by when we use it
	// Referred to as "addin_i" in the C code
	// We can't skip the first five bytes this time,
	// so we start at 0
	bitshift := 0

	// This value lets us know which seq value to use
	// Referred to as "rval" in the C code
	// The C code is very specific about this being mtime & 15,
	// but it's never checked. Let's use zero.
	seqIndex := 0
	seq := seqList[seqIndex]

	var toEncode strings.Builder

	// The C code DOES really care about this value matching
	// the seq_index, though
	toEncode.WriteByte(byte(int('S') - ((seqIndex & 0x7) * 2)))

	// And this is also a song and dance to
	// convince the C code we are legitimate
	toEncode.WriteByte(byte(((mtimeVal >> 4) & 0xF) + int64('a')))
	toEncode.WriteByte(byte((mtimeVal & 0xF) + int64('a')))
	toEncode.WriteByte(byte(((mtimeVal >> 12) & 0xF) + int64('a')))
	toEncode.WriteByte(byte(((mtimeVal >> 8) & 0xF) + int64('a')))

	// We also want to actually encode the passed string
	toEncode.WriteString(s)

	// Yeah, the string starts with a dot. Whatever.
	var encodedBuf bytes.Buffer

	encodedBuf.WriteString(".")

	toEncodeStr := toEncode.String()

	for _, c := range toEncodeStr {
		if c == 0 {
			break
		}

		charByte := byte(c)

		// How far this character is from the target character in wheel
		// Referred to as "add_in" in the C code
		offset := int(((seq >> bitshift) & 0x1F)) + (uid & 0xF5F)

		bitshift += 3
		if bitshift > 28 {
			bitshift = 0
		}

		// The character is only encoded if it's one of the ones in wheel
		if slices.Contains(wheel, charByte) {
			// index of the target character in wheel
			wheelIndex := (slices.Index(wheel, charByte) + offset) % len(wheel)
			encodedBuf.WriteByte(wheel[wheelIndex])
		} else {
			encodedBuf.WriteByte(charByte)
		}
	}

	// insert the seq_index (which is NOT encoded):
	result := encodedBuf.String()
	seqChar := string(byte(seqIndex + int('e')))
	result = result[:6] + seqChar + result[6:]

	// aaaaand, append a null character. because we want to print
	// a null character to the file. because that's a good idea.
	result += string(byte(0))

	return []byte(result)
}
