package msg

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"reflect"
	"slices"
	"strconv"
	"strings"
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
		slog.Warn("error is not empty", "err", string(msg.Body.Error))
	}

	return DecodeC(msg.Body.Message, obj)
}

func EncodeC(obj any) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	w := bufio.NewWriter(buf)

	val := reflect.ValueOf(obj)

	// Marshal argument is allowed to be a pointer
	if val.Kind() == reflect.Pointer {
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

// ErrUnknownType is returned if an unknown type must be translated
var ErrUnknownType = errors.New("unknown type")

const anullstr = "%@#ANULLSTR$%"

func encodeC(e reflect.Value, buf *bufio.Writer) error {
	switch e.Type().Kind() { //nolint:exhaustive
	case reflect.Pointer:
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

// decoder carries the reader plus the chain of structs currently being
// decoded, so a slice length can be resolved against enclosing structs.
type decoder struct {
	buf    *bufio.Reader
	scopes []scope
}

// scope is one struct on the decode stack. Fields before `field` are already
// decoded and may serve as length fields; `field` itself is in progress.
type scope struct {
	val   reflect.Value
	field int
}

func DecodeC(payload []byte, obj any) error {
	e := reflect.ValueOf(obj).Elem()

	// We append a 0 byte just in case, but in principal all strings should be 0-terminated
	d := &decoder{buf: bufio.NewReader(bytes.NewBuffer(append(payload, 0)))}

	err := d.decode(e)
	if errors.Is(err, io.EOF) {
		slog.Info("unexpected EOF while decoding C struct", "cstruct", e.Type().Name(), "payload", payload)
	}

	return err
}

// skipNull consumes an anullstr marker if one is next and reports whether it did.
func (d *decoder) skipNull() (bool, error) {
	peek, err := d.buf.Peek(len(anullstr))
	if err != nil || !bytes.Equal(peek, []byte(anullstr)) {
		return false, nil //nolint:nilerr
	}

	_, err = d.buf.Discard(len(anullstr) + 1) // marker + terminating 0

	return true, err
}

func (d *decoder) decode(e reflect.Value) error {
	switch e.Type().Kind() { //nolint:exhaustive
	case reflect.Pointer:
		if null, err := d.skipNull(); null || err != nil {
			return err
		}

		e.Set(reflect.New(e.Type().Elem()))

		return d.decode(e.Elem())

	case reflect.Struct:
		return d.decodeStruct(e)

	case reflect.Int, reflect.Int32:
		var res int32

		if err := binary.Read(d.buf, binary.BigEndian, &res); err != nil {
			return err
		}

		e.Set(reflect.ValueOf(res).Convert(e.Type()))

		return nil

	case reflect.Int64:
		var res int64

		if err := binary.Read(d.buf, binary.BigEndian, &res); err != nil {
			return err
		}

		e.Set(reflect.ValueOf(res))

		return nil

	case reflect.String:
		value, err := d.buf.ReadString(0)
		if err != nil {
			return fmt.Errorf("could not read next 0-ended string: %w", err)
		}

		value = value[:len(value)-1]
		if value == anullstr {
			return nil
		}

		e.Set(reflect.ValueOf(value).Convert(e.Type()))

		return nil

	default:
		return fmt.Errorf("%w: %s", ErrUnknownType, e.Type().Kind())
	}
}

func (d *decoder) decodeStruct(e reflect.Value) error {
	depth := len(d.scopes)
	d.scopes = append(d.scopes, scope{val: e})

	defer func() { d.scopes = d.scopes[:depth] }()

	for i := 1; i < e.NumField(); i++ {
		d.scopes[depth].field = i

		if err := d.decodeStructField(e, i); err != nil {
			return err
		}
	}

	return nil
}

func (d *decoder) decodeStructField(e reflect.Value, i int) error {
	switch e.Field(i).Type().Kind() { //nolint:exhaustive
	case reflect.Slice:
		if null, err := d.skipNull(); null || err != nil {
			return err
		}

		size, err := d.sliceSize(e.Type().Field(i))
		if err != nil {
			return err
		}

		return d.decodeSlice(e.Field(i), size)

	case reflect.String:
		tag := e.Type().Field(i).Tag.Get("native")
		if !strings.HasPrefix(tag, base64tag) {
			break
		}

		size, err := strconv.Atoi(strings.TrimPrefix(tag, base64tag))
		if err != nil {
			return err
		}

		return d.decodeBase64String(e.Field(i), size)
	default:
	}

	return d.decode(e.Field(i))
}

func (d *decoder) decodeSlice(e reflect.Value, size int) error {
	e.Set(reflect.MakeSlice(e.Type(), size, size))

	for j := range size {
		if err := d.decode(e.Index(j)); err != nil {
			return err
		}
	}

	return nil
}

func (d *decoder) decodeBase64String(e reflect.Value, size int) error {
	value := make([]byte, size)

	if _, err := io.ReadFull(d.buf, value); err != nil {
		return fmt.Errorf("could not read next %d bytes: %w", size, err)
	}

	e.Set(reflect.ValueOf(base64.StdEncoding.EncodeToString(value)))

	return nil
}

// ErrExpectLen is returned if a slice is not proceeded by a int field
var ErrExpectLen = errors.New("except a length integer proceeding a slice")

// sliceSize determines a slice's length from its `size` tag, or else from the
// already-decoded integer field named by its `sizeField` tag.
func (d *decoder) sliceSize(field reflect.StructField) (int, error) {
	if sizeStr := field.Tag.Get("size"); sizeStr != "" {
		return strconv.Atoi(sizeStr)
	}

	name := field.Tag.Get("sizeField")
	if name == "" {
		return 0, ErrExpectLen
	}

	// Innermost struct first, then outward through the enclosing structs, so a
	// nested length field shadows an outer one of the same name.
	for _, sc := range slices.Backward(d.scopes) {
		for j := 1; j < sc.field; j++ {
			if sc.val.Type().Field(j).Tag.Get("xml") != name {
				continue
			}

			if f := sc.val.Field(j); f.CanInt() {
				return int(f.Int()), nil
			}

			return 0, ErrExpectLen
		}
	}

	return 0, ErrExpectLen
}
