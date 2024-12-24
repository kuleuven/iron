package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"gitea.icts.kuleuven.be/coz/iron/msg"
	"go.uber.org/multierr"
)

type handle struct {
	api            *api
	ctx            context.Context //nolint:containedctx
	conn           Conn
	path           string
	FileDescriptor msg.FileDescriptor
	origin         *handle        // If this handle is a reopened handle, this contains the original handle
	wg             sync.WaitGroup // Waitgroup if the file was reopened, for the reopened handle to be closed
}

func (h *handle) Close() error {
	if h.origin != nil {
		defer h.origin.wg.Done()

		request := msg.CloseDataObjectReplicaRequest{
			FileDescriptor: h.FileDescriptor,
		}

		err := h.conn.Request(h.ctx, msg.REPLICA_CLOSE_APN, request, &msg.EmptyResponse{})

		err = multierr.Append(err, h.conn.Close())

		return err
	}

	h.wg.Wait()

	request := msg.OpenedDataObjectRequest{
		FileDescriptor: h.FileDescriptor,
	}

	err := h.conn.Request(h.ctx, msg.DATA_OBJ_CLOSE_AN, request, &msg.EmptyResponse{})

	err = multierr.Append(err, h.conn.Close())

	return err
}

func (h *handle) Seek(offset int64, whence int) (int64, error) {
	request := msg.OpenedDataObjectRequest{
		FileDescriptor: h.FileDescriptor,
		Whence:         whence,
		Offset:         offset,
	}

	var response msg.SeekResponse

	err := h.conn.Request(h.ctx, msg.DATA_OBJ_LSEEK_AN, request, &response)

	return response.Offset, err
}

func (h *handle) Read(b []byte) (int, error) {
	request := msg.OpenedDataObjectRequest{
		FileDescriptor: h.FileDescriptor,
		Size:           int64(len(b)),
	}

	var response msg.ReadResponse

	if err := h.conn.RequestWithBuffers(h.ctx, msg.DATA_OBJ_READ_AN, request, &response, nil, b); err != nil {
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

	return len(b), h.conn.RequestWithBuffers(h.ctx, msg.DATA_OBJ_WRITE_AN, request, &msg.EmptyResponse{}, b, nil)
}

var ErrSameConnection = errors.New("same connection")

func (h *handle) Reopen(conn Conn, mode int) (File, error) {
	if conn == nil {
		var err error

		conn, err = h.api.Connect(h.ctx)
		if err != nil {
			return nil, err
		}
	}

	// Check that the caller didn't provide the same connection
	if conn == h.conn {
		return nil, ErrSameConnection
	}

	replicaToken, resourceHierarchy, err := h.GetReplicaAccessInfo()
	if err != nil {
		err = multierr.Append(err, conn.Close())

		return nil, err
	}

	request := msg.DataObjectRequest{
		Path:      h.path,
		OpenFlags: mode &^ O_APPEND,
	}

	request.KeyVals.Add(msg.RESC_HIER_STR_KW, resourceHierarchy)
	request.KeyVals.Add(msg.REPLICA_TOKEN_KW, replicaToken)

	h.api.SetFlags(&request.KeyVals)

	h2 := handle{
		api:    h.api,
		conn:   conn,
		ctx:    h.ctx,
		path:   h.path,
		origin: h,
	}

	err = conn.Request(h.ctx, msg.DATA_OBJ_OPEN_AN, request, &h.FileDescriptor)
	if err == nil && mode&O_APPEND != 0 {
		// Irods does not support O_APPEND, we need to seek to the end
		_, err = h.Seek(0, 2)
	}

	if err != nil {
		err = multierr.Append(err, conn.Close())

		return nil, err
	}

	// Add to waitgroup
	h.wg.Add(1)

	return &h2, nil
}

func (h *handle) GetReplicaAccessInfo() (string, string, error) {
	response := msg.GetDescriptorInfoResponse{}

	if err := h.conn.Request(h.ctx, msg.GET_FILE_DESCRIPTOR_INFO_APN, msg.GetDescriptorInfoRequest{FileDescriptor: h.FileDescriptor}, &response); err != nil {
		return "", "", err
	}

	resourceHierarchy := ""

	if response.DataObjectInfo != nil {
		if resourceHierarchyInfo, ok := response.DataObjectInfo["resource_hierarchy"]; ok {
			resourceHierarchy = strings.TrimSpace(fmt.Sprintf("%v", resourceHierarchyInfo))
		}
	}

	return response.ReplicaToken, resourceHierarchy, nil
}
