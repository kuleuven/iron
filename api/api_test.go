package api

import (
	"context"

	"github.com/kuleuven/iron/msg"
)

type testAPI struct {
	*API
	conn *MockConn
}

func newAPI() *testAPI {
	testConn := &MockConn{}

	return &testAPI{
		API: &API{
			Username: "testuser",
			Zone:     "testzone",
			Connect: func(context.Context) (Conn, error) {
				return testConn, nil
			},
			DefaultResource: "demoResc",
		},
		conn: testConn,
	}
}

func (a *testAPI) Add(apiNumber msg.APINumber, request, response any) {
	a.conn.Add(apiNumber, request, response)
}

func (a *testAPI) AddBuffer(apiNumber msg.APINumber, request, response any, requestBuf, responseBuf []byte) {
	a.conn.AddBuffer(apiNumber, request, response, requestBuf, responseBuf)
}

func (a *testAPI) AddResponse(response any) {
	a.conn.AddResponse(response)
}

func (a *testAPI) AddResponses(responses []any) {
	a.conn.AddResponses(responses)
}
