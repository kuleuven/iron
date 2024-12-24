package msg

import (
	"encoding/xml"
	"fmt"
	"io"
	"reflect"
)

var ErrUnrecognizedType = fmt.Errorf("unrecognized type")

// Marshal marshals the argument into a message.
// The Message is initialized with Bin unset.
func Marshal(obj any, msgType string) (*Message, error) {
	val := reflect.ValueOf(obj)

	// Marshal argument is allowed to be a pointer
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() == reflect.Slice && val.Type().Elem().Kind() == reflect.Uint8 {
		return MarshalBytes(val.Bytes(), msgType)
	}

	if val.Kind() == reflect.Struct && val.Field(0).Type() == reflect.TypeOf(xml.Name{}) {
		return MarshalXML(obj, msgType)
	}

	if val.Kind() == reflect.Struct {
		return MarshalJSON(obj, msgType)
	}

	if val.Kind() == reflect.Int32 {
		return MarshalInt32(int32(val.Int()), msgType)
	}

	return nil, fmt.Errorf("%w: %T", ErrUnrecognizedType, obj)
}

// Unmarshal unmarshals the Message into the argument.
// This will ignore the Bin field.
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

	if val.Kind() == reflect.Struct && val.Field(0).Type() == reflect.TypeOf(xml.Name{}) {
		return UnmarshalXML(msg, obj)
	}

	if val.Kind() == reflect.Struct {
		return UnmarshalJSON(msg, obj)
	}

	if val.Kind() == reflect.Int32 {
		var result int32

		if err := UnmarshalInt32(msg, &result); err != nil {
			return err
		}

		val.SetInt(int64(result))

		return nil
	}

	return fmt.Errorf("%w: %T", ErrUnrecognizedType, obj)
}

var ErrUnexpectedMessage = fmt.Errorf("unexpected message type")

func Read(r io.Reader, obj any, buf []byte, expectedMsgType string) (int32, error) {
	msg := Message{
		Bin: buf,
	}

	if err := msg.Read(r); err != nil {
		return 0, err
	}

	if msg.Header.Type != expectedMsgType {
		return 0, fmt.Errorf("%w: expected %s, got %s", ErrUnexpectedMessage, expectedMsgType, msg.Header.Type)
	}

	return msg.Header.IntInfo, Unmarshal(msg, obj)
}

func Write(w io.Writer, obj any, buf []byte, msgType string, intInfo int32) error {
	msg, err := Marshal(obj, msgType)
	if err != nil {
		return err
	}

	msg.Bin = buf
	msg.Header.BsLen = uint32(len(buf))
	msg.Header.IntInfo = intInfo

	return msg.Write(w)
}
