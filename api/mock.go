package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/kuleuven/iron/msg"
)

type MockConn struct {
	Dialog []Dialog
}

type Dialog struct {
	msg.APINumber
	Request, Response       any
	RequestBuf, ResponseBuf []byte
}

func (c *MockConn) Add(apiNumber msg.APINumber, request, response any) {
	c.Dialog = append(c.Dialog, Dialog{
		APINumber: apiNumber,
		Request:   request,
		Response:  response,
	})
}

func (c *MockConn) AddBuffer(apiNumber msg.APINumber, request, response any, requestBuf, responseBuf []byte) {
	c.Dialog = append(c.Dialog, Dialog{
		APINumber:   apiNumber,
		Request:     request,
		Response:    response,
		RequestBuf:  requestBuf,
		ResponseBuf: responseBuf,
	})
}

func (c *MockConn) AddResponse(response any) {
	c.Dialog = append(c.Dialog, Dialog{
		APINumber: -1,
		Response:  response,
	})
}

func (c *MockConn) AddResponses(responses []any) {
	for _, response := range responses {
		c.AddResponse(response)
	}
}

func (c *MockConn) ClientSignature() string {
	return "testsignature"
}

func (c *MockConn) NativePassword() string {
	return "testpassword"
}

func (c *MockConn) Request(ctx context.Context, apiNumber msg.APINumber, request, response any) error {
	return c.RequestWithBuffers(ctx, apiNumber, request, response, nil, nil)
}

func (c *MockConn) RequestWithBuffers(ctx context.Context, apiNumber msg.APINumber, request, response any, requestBuf, responseBuf []byte) error {
	if len(c.Dialog) == 0 {
		return errors.New("no dialog found")
	}

	dialog := c.Dialog[0]
	c.Dialog = c.Dialog[1:]

	if dialog.APINumber == -1 {
		return setResponse(dialog, response, responseBuf)
	}

	if apiNumber != dialog.APINumber {
		return fmt.Errorf("unexpected API number: expected %d, got %d", dialog.APINumber, apiNumber)
	}

	requestMsg, err := msg.Marshal(request, msg.XML, "RODS_API_REQ")
	if err != nil {
		return err
	}

	expectedMsg, err := msg.Marshal(dialog.Request, msg.XML, "RODS_API_REQ")
	if err != nil {
		return err
	}

	if !bytes.Equal(requestMsg.Body.Message, expectedMsg.Body.Message) {
		return fmt.Errorf("[%d] unexpected request body: expected %s (%d), got %s (%d)", apiNumber, expectedMsg.Body.Message, len(expectedMsg.Body.Message), requestMsg.Body.Message, len(requestMsg.Body.Message))
	}

	if !reflect.DeepEqual(requestMsg, expectedMsg) {
		return fmt.Errorf("[%d] unexpected request: expected %v, got %v", apiNumber, expectedMsg, requestMsg)
	}

	if !bytes.Equal(requestBuf, dialog.RequestBuf) {
		return fmt.Errorf("[%d] unexpected request buffer: expected %s, got %s", apiNumber, dialog.RequestBuf, requestBuf)
	}

	return setResponse(dialog, response, responseBuf)
}

func setResponse(dialog Dialog, response any, responseBuf []byte) error {
	if err, ok := dialog.Response.(error); ok {
		return err
	}

	val := reflect.ValueOf(response)

	if val.Kind() != reflect.Ptr {
		return fmt.Errorf("%w: expected ptr, got %T", msg.ErrUnrecognizedType, response)
	}

	val.Elem().Set(reflect.ValueOf(dialog.Response))

	copy(responseBuf, dialog.ResponseBuf)

	return nil
}

func (c *MockConn) Close() error {
	return nil
}

func (c *MockConn) RegisterCloseHandler(handler func() error) context.CancelFunc {
	return func() {
		// do nothing
	}
}
