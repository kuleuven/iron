package msg

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/sirupsen/logrus"
)

var (
	// escapes from xml.Encode
	escQuot = []byte("&#34;") // shorter than "&quot;", \"
	escApos = []byte("&#39;") // shorter than "&apos;", \'
	escTab  = []byte("&#x9;")
	escNL   = []byte("&#xA;")
	escCR   = []byte("&#xD;")
	escFFFD = []byte("\uFFFD") // Unicode replacement character

	// escapes for irods
	irodsEscQuot = []byte("&quot;")
	irodsEscApos = []byte("&apos;")
)

// ErrInvalidUTF8 is returned if an invalid utf-8 character is found.
var ErrInvalidUTF8 = errors.New("invalid utf-8 character")

func marshalXML(obj any, msgType string) (*Message, error) {
	body, err := xml.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal irods message to xml: %w", err)
	}

	body, err = PreprocessXML(body)
	if err != nil {
		return nil, fmt.Errorf("failed to preprocess xml: %w", err)
	}

	return &Message{
		Header: Header{
			Type:       msgType,
			MessageLen: uint32(len(body)),
		},
		Body: Body{
			Message: body,
		},
	}, nil
}

func unmarshalXML(msg Message, obj any) error {
	if msg.Header.MessageLen == 0 {
		// A CollectionOperationStat is a special case and allowed to be empty if the server doesn't send it
		if _, ok := obj.(*CollectionOperationStat); ok {
			return nil
		}

		return fmt.Errorf("message length is zero")
	}

	if msg.Header.ErrorLen > 0 {
		logrus.Warnf("error is not empty: %s", string(msg.Body.Error))
	}

	body, err := PostprocessXML(msg.Body.Message)
	if err != nil {
		return fmt.Errorf("failed to postprocess xml: %w", err)
	}

	return xml.Unmarshal(body, obj)
}

// PreprocessXML translates output of xml.Marshal into XML that IRODS understands.
func PreprocessXML(in []byte) ([]byte, error) {
	buf := in
	out := &bytes.Buffer{}

	for len(buf) > 0 {
		switch {
		// turn &#34; into &quot;
		case bytes.HasPrefix(buf, escQuot):
			out.Write(irodsEscQuot)

			buf = buf[len(escQuot):]
		// turn &#39 into &apos; or '
		case bytes.HasPrefix(buf, escApos):
			out.Write(irodsEscApos)

			buf = buf[len(escApos):]
		// irods does not decode encoded tabs
		case bytes.HasPrefix(buf, escTab):
			out.WriteByte('\t')

			buf = buf[len(escTab):]
		// irods does not decode encoded carriage returns
		case bytes.HasPrefix(buf, escCR):
			out.WriteByte('\r')

			buf = buf[len(escCR):]
		// irods does not decode encoded newlines
		case bytes.HasPrefix(buf, escNL):
			out.WriteByte('\n')

			buf = buf[len(escNL):]
		// pass utf8 characters
		default:
			r, size := utf8.DecodeRune(buf)

			if r == utf8.RuneError && size == 1 {
				return in, ErrInvalidUTF8
			}

			out.Write(buf[:size])

			buf = buf[size:]
		}
	}

	return out.Bytes(), nil
}

// PostprocessXML checks for invalid utf-8 characters.
func PostprocessXML(in []byte) ([]byte, error) {
	buf := in
	out := &bytes.Buffer{}

	for len(buf) > 0 {
		r, size := utf8.DecodeRune(buf)

		if r == utf8.RuneError && size == 1 {
			return in, ErrInvalidUTF8
		}

		if isValidChar(r) {
			out.Write(buf[:size])
		} else {
			out.Write(escFFFD)
		}

		buf = buf[size:]
	}

	return out.Bytes(), nil
}

func isValidChar(r rune) bool {
	return r == 0x09 ||
		r == 0x0A ||
		r == 0x0D ||
		r >= 0x20 && r <= 0xD7FF ||
		r >= 0xE000 && r <= 0xFFFD ||
		r >= 0x10000 && r <= 0x10FFFF
}
