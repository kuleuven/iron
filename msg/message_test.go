package msg

import (
	"bytes"
	"encoding/xml"
	"reflect"
	"testing"
)

func TestMessage(t *testing.T) {
	buf := bytes.NewBuffer(nil)

	body := []byte("test")

	msg := Message{
		Header: Header{
			Type:       "test",
			MessageLen: uint32(len(body)),
		},
		Body: Body{
			Message: body,
			Error:   []byte{},
		},
		Bin: nil,
	}

	if err := msg.Write(buf); err != nil {
		t.Fatal(err)
	}

	var msg2 Message

	if err := msg2.Read(buf); err != nil {
		t.Fatal(err)
	}

	// When marshaling, XMLName is populated
	msg2.Header.XMLName = xml.Name{}

	if !reflect.DeepEqual(msg, msg2) {
		t.Fatalf("expected %v, got %v", msg, msg2)
	}
}
