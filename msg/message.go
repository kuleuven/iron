package msg

import (
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/sirupsen/logrus"
)

type Header struct {
	XMLName    xml.Name `xml:"MsgHeader_PI"`
	Type       string   `xml:"type"`
	MessageLen uint32   `xml:"msgLen"`
	ErrorLen   uint32   `xml:"errorLen"`
	BsLen      uint32   `xml:"bsLen"`
	IntInfo    int32    `xml:"intInfo"`
}

type Body struct {
	Message []byte
	Error   []byte
}

type Message struct {
	Header Header
	Body   Body
	Bin    []byte
}

// Write writes an iRODS message to w
func (msg Message) Write(w io.Writer) error {
	if err := msg.Header.Write(w); err != nil {
		return err
	}

	if err := msg.Body.Write(w); err != nil {
		return err
	}

	logrus.Tracef("-> bin: %d bytes", len(msg.Bin))

	_, err := w.Write(msg.Bin)

	return err
}

func (header Header) Write(w io.Writer) error {
	payload, err := xml.Marshal(header)
	if err != nil {
		return err
	}

	logrus.Tracef("-> %s", payload)

	// Write header
	headerLenBuffer := make([]byte, 4)
	binary.BigEndian.PutUint32(headerLenBuffer, uint32(len(payload)))

	if _, err := w.Write(headerLenBuffer); err != nil {
		return err
	}

	if _, err := w.Write(payload); err != nil {
		return err
	}

	return nil
}

func (body Body) Write(w io.Writer) error {
	if toLog := strings.ReplaceAll(fmt.Sprintf("%s %s", body.Message, body.Error), "\n", ""); isPrintable(toLog) {
		logrus.Tracef("-> %s", toLog)
	} else {
		logrus.Tracef("-> %d bytes", len(body.Message)+len(body.Error))
	}

	if _, err := w.Write(body.Message); err != nil {
		return err
	}

	if _, err := w.Write(body.Error); err != nil {
		return err
	}

	return nil
}

func isPrintable(s string) bool {
	for _, r := range s {
		if r > unicode.MaxASCII || !unicode.IsPrint(r) {
			return false
		}
	}

	return true
}

// Read decodes an iRODS message from r.
// The caller should provide an empty Message with a large enough Bin buffer.
// If the provided buffer is too small, a larger one will be allocated.
func (msg *Message) Read(r io.Reader) error {
	if err := msg.Header.Read(r); err != nil {
		return err
	}

	if err := msg.Body.Read(r, msg.Header); err != nil {
		return err
	}

	if len(msg.Bin) < int(msg.Header.BsLen) {
		logrus.Warnf("expected %d bytes, got %d, cannot use provided buffer", msg.Header.BsLen, len(msg.Bin))

		msg.Bin = make([]byte, msg.Header.BsLen)
	}

	logrus.Tracef("<- bin: %d bytes", msg.Header.BsLen)

	_, err := io.ReadFull(r, msg.Bin[:msg.Header.BsLen])
	if err != nil {
		return err
	}

	return nil
}

func (header *Header) Read(r io.Reader) error {
	headerLenBuffer := make([]byte, 4)

	if _, err := io.ReadFull(r, headerLenBuffer); err != nil {
		return err
	}

	headerLen := binary.BigEndian.Uint32(headerLenBuffer)

	headerBuffer := make([]byte, headerLen)

	if _, err := io.ReadFull(r, headerBuffer); err != nil {
		return err
	}

	logrus.Tracef("<- %s", bytes.ReplaceAll(headerBuffer, []byte("\n"), nil))

	return xml.Unmarshal(headerBuffer, &header)
}

func (body *Body) Read(r io.Reader, header Header) error {
	body.Message = make([]byte, header.MessageLen)
	body.Error = make([]byte, header.ErrorLen)

	if _, err := io.ReadFull(r, body.Message); err != nil {
		return err
	}

	if _, err := io.ReadFull(r, body.Error); err != nil {
		return err
	}

	if toLog := strings.ReplaceAll(fmt.Sprintf("%s %s", body.Message, body.Error), "\n", ""); isPrintable(toLog) {
		logrus.Tracef("<- %s", toLog)
	} else {
		logrus.Tracef("<- %d bytes", len(body.Message)+len(body.Error))
	}

	return nil
}
