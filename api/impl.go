package api

import "context"

type api struct {
	Connect func(context.Context) (Conn, error)
}
