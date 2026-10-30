package msg

import "log/slog"

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
		slog.Warn("error is not empty", "err", string(msg.Body.Error))
	}

	*body = msg.Header.IntInfo

	return nil
}
