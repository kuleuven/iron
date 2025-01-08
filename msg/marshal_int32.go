package msg

import "github.com/sirupsen/logrus"

func marshalInt32(body int32, msgType string) (*Message, error) {
	return &Message{
		Header: Header{
			Type:    msgType,
			IntInfo: body,
		},
		Body: Body{},
	}, nil
}

func unmarshalInt32(msg Message, body *int32) error {
	if msg.Header.ErrorLen > 0 {
		logrus.Warnf("error is not empty: %s", string(msg.Body.Error))
	}

	*body = msg.Header.IntInfo

	return nil
}
