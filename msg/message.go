package msg

import (
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"io"

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
	Bs      []byte
}

type Message struct {
	Header Header
	Body   Body
}

// Write writes an iRODS message to w
func (msg Message) Write(w io.Writer) error {
	if err := msg.Header.Write(w); err != nil {
		return err
	}

	return msg.Body.Write(w)
}

func (header Header) Write(w io.Writer) error {
	payload, err := xml.Marshal(header)
	if err != nil {
		return err
	}

	logrus.Tracef("> %s", payload)

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
	logrus.Tracef("> body length %d", len(body.Message)+len(body.Error)+len(body.Bs))

	if _, err := w.Write(body.Message); err != nil {
		return err
	}

	if _, err := w.Write(body.Error); err != nil {
		return err
	}

	if _, err := w.Write(body.Bs); err != nil {
		return err
	}

	return nil
}

// Read decodes an iRODS message from r
func (msg *Message) Read(r io.Reader) error {
	if err := msg.Header.Read(r); err != nil {
		return err
	}

	return msg.Body.Read(r, msg.Header)
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

	logrus.Tracef("< %s", bytes.ReplaceAll(headerBuffer, []byte("\n"), nil))

	return xml.Unmarshal(headerBuffer, &header)
}

func (body *Body) Read(r io.Reader, header Header) error {
	body.Message = make([]byte, header.MessageLen)
	body.Error = make([]byte, header.ErrorLen)
	body.Bs = make([]byte, header.BsLen)

	if _, err := io.ReadFull(r, body.Message); err != nil {
		return err
	}

	if _, err := io.ReadFull(r, body.Error); err != nil {
		return err
	}

	if _, err := io.ReadFull(r, body.Bs); err != nil {
		return err
	}

	logrus.Tracef("< body length %d", len(body.Message)+len(body.Error)+len(body.Bs))

	return nil
}
