package msg

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

func marshalCStruct(obj any, msgType string) (*Message, error) {
	body, err := EncodeC(obj)

	return &Message{
		Header: Header{
			Type:       msgType,
			MessageLen: uint32(len(body)),
		},
		Body: Body{
			Message: body,
		},
	}, err
}

func unmarshalCStruct(msg Message, obj any) error {
	if msg.Header.MessageLen == 0 {
		// A CollectionOperationStat is a special case and allowed to be empty if the server doesn't send it
		if _, ok := obj.(*CollectionOperationStat); ok {
			return nil
		}

		return fmt.Errorf("message length is zero")
	}

	if msg.Header.ErrorLen > 0 {
		logrus.Warnf("error is not empty: %s", string(msg.Body.Error))
	}

	return DecodeC(msg.Body.Message, obj)
}

func EncodeC(obj any) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	w := bufio.NewWriter(buf)

	val := reflect.ValueOf(obj)

	// Marshal argument is allowed to be a pointer
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if err := encodeC(val, w); err != nil {
		return nil, err
	}

	if err := w.Flush(); err != nil {
		return nil, err
	}

	body := buf.Bytes()

	return body, nil
}

func DecodeC(payload []byte, obj any) error {
	e := reflect.ValueOf(obj).Elem()

	// We append a 0 byte just in case, but in principal all strings should be 0-terminated
	buf := bufio.NewReader(bytes.NewBuffer(append(payload, 0)))

	return decodeC(e, buf)
}

// ErrUnknownType is returned if an unknown type must be translated
var ErrUnknownType = errors.New("unknown type")

const anullstr = "%@#ANULLSTR$%"

func encodeC(e reflect.Value, buf *bufio.Writer) error {
	switch e.Type().Kind() { //nolint:exhaustive
	case reflect.Ptr:
		if !e.IsNil() {
			return encodeC(e.Elem(), buf)
		}

		if _, err := buf.WriteString(anullstr); err != nil {
			return err
		}

		if _, err := buf.Write([]byte{0}); err != nil {
			return err
		}

		return nil

	case reflect.Struct:
		return encodeCStruct(e, buf)

	case reflect.Slice:
		return encodeCSlice(e, buf)

	case reflect.Int, reflect.Int32:
		return binary.Write(buf, binary.BigEndian, int32(e.Int()))

	case reflect.Int64:
		return binary.Write(buf, binary.BigEndian, e.Int())

	case reflect.String:
		if _, err := buf.WriteString(e.String()); err != nil {
			return err
		}

		if _, err := buf.Write([]byte{0}); err != nil {
			return err
		}

		return nil

	default:
		return fmt.Errorf("%w: %s", ErrUnknownType, e.Type().Kind())
	}
}

func encodeCStruct(e reflect.Value, buf *bufio.Writer) error {
	for i := 1; i < e.NumField(); i++ {
		if err := encodeCStructField(e, buf, i); err != nil {
			return err
		}
	}

	return nil
}

const base64tag = "base64,"

func encodeCStructField(e reflect.Value, buf *bufio.Writer, i int) error {
	if e.Field(i).Type().Kind() == reflect.String && strings.HasPrefix(e.Type().Field(i).Tag.Get("native"), base64tag) {
		return encodeCBase64String(e.Field(i), buf)
	}

	return encodeC(e.Field(i), buf)
}

func encodeCBase64String(e reflect.Value, buf *bufio.Writer) error {
	payload, err := base64.StdEncoding.DecodeString(e.String())
	if err != nil {
		return err
	}

	if _, err := buf.Write(payload); err != nil {
		return err
	}

	return nil
}

func encodeCSlice(e reflect.Value, buf *bufio.Writer) error {
	if e.Len() == 0 {
		if _, err := buf.WriteString(anullstr); err != nil {
			return err
		}

		if _, err := buf.Write([]byte{0}); err != nil {
			return err
		}

		return nil
	}

	for i := range e.Len() {
		if err := encodeC(e.Index(i), buf); err != nil {
			return err
		}
	}

	return nil
}

func decodeC(e reflect.Value, buf *bufio.Reader) error {
	switch e.Type().Kind() { //nolint:exhaustive
	case reflect.Ptr:
		if peek, err := buf.Peek(13); err == nil && bytes.Equal(peek, []byte(anullstr)) {
			_, err = buf.Discard(14)

			return err
		}

		e.Set(reflect.New(e.Type().Elem()))

		return decodeC(e.Elem(), buf)
	case reflect.Struct:
		return decodeCStruct(e, buf)

	case reflect.Int, reflect.Int32:
		var res int32

		if err := binary.Read(buf, binary.BigEndian, &res); err != nil {
			return err
		}

		e.Set(reflect.ValueOf(res).Convert(e.Type()))

		return nil

	case reflect.Int64:
		var res int64

		if err := binary.Read(buf, binary.BigEndian, &res); err != nil {
			return err
		}

		e.Set(reflect.ValueOf(res))

		return nil

	case reflect.String:
		value, err := buf.ReadString(0)
		if err != nil {
			return fmt.Errorf("could not read next 0-ended string: %w", err)
		}

		value = value[0 : len(value)-1]

		if value == anullstr {
			return nil
		}

		e.Set(reflect.ValueOf(value).Convert(e.Type()))

		return nil

	default:
		return fmt.Errorf("%w: %s", ErrUnknownType, e.Type().Kind())
	}
}

func decodeCStruct(e reflect.Value, buf *bufio.Reader) error {
	for i := 1; i < e.NumField(); i++ {
		if err := decodeCStructField(e, buf, i); err != nil {
			return err
		}
	}

	return nil
}

func decodeCStructField(e reflect.Value, buf *bufio.Reader, i int) error {
	switch e.Field(i).Type().Kind() { //nolint:exhaustive
	case reflect.Slice:
		if peek, err := buf.Peek(13); err == nil && bytes.Equal(peek, []byte(anullstr)) {
			_, err = buf.Discard(14)

			return err
		}

		size, err := findCStructSliceSize(e, i)
		if err != nil {
			return err
		}

		return decodeCSlice(e.Field(i), size, buf)
	case reflect.String:
		tag := e.Type().Field(i).Tag.Get("native")
		if !strings.HasPrefix(tag, base64tag) {
			break
		}

		size, err := strconv.Atoi(strings.TrimPrefix(tag, base64tag))
		if err != nil {
			return err
		}

		return decodeCBase64String(e.Field(i), buf, size)
	default:
	}

	return decodeC(e.Field(i), buf)
}

func decodeCSlice(e reflect.Value, size int, buf *bufio.Reader) error {
	slice := reflect.MakeSlice(e.Type(), size, size)

	e.Set(slice)

	for j := range size {
		if err := decodeC(e.Index(j), buf); err != nil {
			return err
		}
	}

	return nil
}

func decodeCBase64String(e reflect.Value, buf *bufio.Reader, size int) error {
	value := make([]byte, size)

	_, err := buf.Read(value)
	if err != nil {
		return fmt.Errorf("could not read next %d bytes: %w", size, err)
	}

	e.Set(reflect.ValueOf(base64.StdEncoding.EncodeToString(value)))

	return nil
}

// ErrExpectLen is returned if a slice is not proceeded by a int field
var ErrExpectLen = errors.New("except a length integer proceeding a slice")

func findCStructSliceSize(e reflect.Value, i int) (int, error) {
	if sizeStr := e.Type().Field(i).Tag.Get("size"); sizeStr != "" {
		return strconv.Atoi(sizeStr)
	}

	sizeField := e.Type().Field(i).Tag.Get("sizeField")
	if sizeField == "" {
		return 0, ErrExpectLen
	}

	// Find field in struct
	for j := 1; j < i; j++ {
		if e.Type().Field(j).Tag.Get("xml") != sizeField {
			continue
		}

		return int(e.Field(j).Int()), nil
	}

	return 0, ErrExpectLen
}
