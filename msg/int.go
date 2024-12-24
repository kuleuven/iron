package msg

import "github.com/sirupsen/logrus"

func MarshalInt32(body int32, msgType string) (*Message, error) {
	return &Message{
		Header: Header{
			Type:    msgType,
			IntInfo: body,
		},
		Body: Body{},
	}, nil
}

func UnmarshalInt32(msg Message, body *int32) error {
	if msg.Header.ErrorLen > 0 {
		logrus.Warnf("error is not empty: %s", string(msg.Body.Error))
	}

	*body = msg.Header.IntInfo

	if msg.Header.BsLen > 0 && msg.Header.IntInfo == 0 {
		logrus.Warnf("intInfo is zero, but bsLen is %d; TODO: rewrite code", msg.Header.BsLen)

		*body = int32(msg.Header.BsLen)
	}

	return nil
}
