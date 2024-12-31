package msg

import (
	"errors"

	"github.com/sirupsen/logrus"
)

var ErrTypeAssertion = errors.New("type assertion failed")

func marshalBytes(body []byte, msgType string) (*Message, error) {
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

func unmarshalBytes(msg Message, body *[]byte) error {
	if msg.Header.ErrorLen > 0 {
		logrus.Warnf("error is not empty: %s", string(msg.Body.Error))
	}

	*body = msg.Body.Message

	return nil
}
