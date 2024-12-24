package api

import (
	"context"
	"io"

	"gitea.icts.kuleuven.be/coz/iron/msg"
)

type handle struct {
	api            *api
	ctx            context.Context //nolint:containedctx
	FileDescriptor msg.FileDescriptor
}

func (h *handle) Close() error {
	request := msg.OpenedDataObjectRequest{
		FileDescriptor: h.FileDescriptor,
	}

	return h.api.Request(h.ctx, msg.DATA_OBJ_CLOSE_AN, request, &msg.EmptyResponse{})
}

func (h *handle) Seek(offset int64, whence int) (int64, error) {
	request := msg.OpenedDataObjectRequest{
		FileDescriptor: h.FileDescriptor,
		Whence:         whence,
		Offset:         offset,
	}

	var response msg.SeekResponse

	err := h.api.Request(h.ctx, msg.DATA_OBJ_LSEEK_AN, request, &response)

	return response.Offset, err
}

func (h *handle) Read(b []byte) (int, error) {
	request := msg.OpenedDataObjectRequest{
		FileDescriptor: h.FileDescriptor,
		Size:           int64(len(b)),
	}

	var response msg.ReadResponse

	if err := h.api.RequestWithBuffers(h.ctx, msg.DATA_OBJ_READ_AN, request, &response, nil, b); err != nil {
		return 0, err
	}

	n := int(response)

	if n >= len(b) {
		return n, nil
	}

	return n, io.EOF
}

func (h *handle) Write(b []byte) (int, error) {
	request := msg.OpenedDataObjectRequest{
		FileDescriptor: h.FileDescriptor,
		Size:           int64(len(b)),
	}

	return len(b), h.api.RequestWithBuffers(h.ctx, msg.DATA_OBJ_WRITE_AN, request, &msg.EmptyResponse{}, b, nil)
}
