package msg

import (
	"errors"
	"log/slog"
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
		slog.Warn("error is not empty", "err", string(msg.Body.Error))
	}

	*body = msg.Body.Message

	return nil
}
