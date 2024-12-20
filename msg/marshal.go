package msg

import (
	"fmt"
	"io"
	"reflect"
)

var ErrUnrecognizedType = fmt.Errorf("unrecognized type")

func Marshal(obj any, msgType string) (*Message, error) {
	val := reflect.ValueOf(obj)

	// Marshal argument is allowed to be a pointer
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() == reflect.Slice && val.Type().Elem().Kind() == reflect.Uint8 {
		return MarshalBytes(val.Bytes(), msgType)
	}

	if val.Kind() == reflect.Struct {
		return MarshalXML(obj, msgType)
	}

	return nil, fmt.Errorf("%w: %T", ErrUnrecognizedType, obj)
}

func Unmarshal(msg Message, obj any) error {
	ptr := reflect.ValueOf(obj)

	// Unmarshal argument is required to be a pointer
	if ptr.Kind() != reflect.Ptr {
		return fmt.Errorf("%w: expected ptr, got %T", ErrUnrecognizedType, obj)
	}

	val := ptr.Elem()

	if val.Kind() == reflect.Slice && val.Type().Elem().Kind() == reflect.Uint8 {
		var result []byte

		if err := UnmarshalBytes(msg, &result); err != nil {
			return err
		}

		val.Set(reflect.ValueOf(result))

		return nil
	}

	if val.Kind() == reflect.Struct {
		return UnmarshalXML(msg, obj)
	}

	return fmt.Errorf("%w: %T", ErrUnrecognizedType, obj)
}

var ErrUnexpectedMessage = fmt.Errorf("unexpected message type")

func Read(r io.Reader, obj any, expectedMsgType string) (int32, error) {
	msg := Message{}

	if err := msg.Read(r); err != nil {
		return 0, err
	}

	if msg.Header.Type != expectedMsgType {
		return 0, fmt.Errorf("%w: expected %s, got %s", ErrUnexpectedMessage, expectedMsgType, msg.Header.Type)
	}

	return msg.Header.IntInfo, Unmarshal(msg, obj)
}

func Write(w io.Writer, obj any, msgType string, intInfo int32) error {
	msg, err := Marshal(obj, msgType)
	if err != nil {
		return err
	}

	msg.Header.IntInfo = intInfo

	return msg.Write(w)
}
