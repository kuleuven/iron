package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"time"

	"gitea.icts.kuleuven.be/coz/iron/msg"
	"go.uber.org/multierr"
)

type handle struct {
	api            *api
	ctx            context.Context //nolint:containedctx
	conn           Conn
	path           string
	FileDescriptor msg.FileDescriptor

	// Housekeeping for reopened handles
	origin       *handle        // If this handle is a reopened handle, this contains the original handle
	wg           sync.WaitGroup // Waitgroup if the file was reopened, for the reopened handle to be closed
	truncateSize int64          // If nonnegative, truncate the file to this size after closing
	touchTime    time.Time      // If non-zero, touch the file to this time after closing
}

func (h *handle) Close() error {
	if h.origin != nil {
		return h.closeReopenedHandle()
	}

	h.wg.Wait()

	var replicaInfo *ReplicaAccessInfo

	if h.truncateSize >= 0 || !h.touchTime.IsZero() {
		var err error

		replicaInfo, err = h.GetReplicaAccessInfo()
		if err != nil {
			err = multierr.Append(err, h.closeOriginalHandle())

			return err
		}
	}

	if err := h.closeOriginalHandle(); err != nil {
		return err
	}

	if err := h.doTruncate(replicaInfo); err != nil {
		return err
	}

	if err := h.doTouch(replicaInfo); err != nil {
		return err
	}

	return nil
}

func (h *handle) closeReopenedHandle() error {
	defer h.origin.wg.Done()

	request := msg.CloseDataObjectReplicaRequest{
		FileDescriptor: h.FileDescriptor,
	}

	err := h.conn.Request(h.ctx, msg.REPLICA_CLOSE_APN, request, &msg.EmptyResponse{})

	err = multierr.Append(err, h.conn.Close())

	return err
}

func (h *handle) closeOriginalHandle() error {
	request := msg.OpenedDataObjectRequest{
		FileDescriptor: h.FileDescriptor,
	}

	err := h.conn.Request(h.ctx, msg.DATA_OBJ_CLOSE_AN, request, &msg.EmptyResponse{})

	err = multierr.Append(err, h.conn.Close())

	return err
}

func (h *handle) doTruncate(replicaInfo *ReplicaAccessInfo) error {
	if h.truncateSize < 0 {
		return nil
	}

	request := msg.DataObjectRequest{
		Path: h.path,
		Size: h.truncateSize,
	}

	request.KeyVals.Add(msg.RESC_HIER_STR_KW, replicaInfo.ResourceHierarchy)
	request.KeyVals.Add(msg.REPLICA_TOKEN_KW, replicaInfo.ReplicaToken)

	h.api.SetFlags(&request.KeyVals)

	return h.api.Request(h.ctx, msg.REPLICA_TRUNCATE_AN, request, &msg.EmptyResponse{})
}

func (h *handle) doTouch(replicaInfo *ReplicaAccessInfo) error {
	if h.touchTime.IsZero() {
		return nil
	}

	request := msg.TouchDataObjectReplicaRequest{
		Path: h.path,
		Options: msg.TouchOptions{
			SecondsSinceEpoch: h.touchTime.Unix(),
			ReplicaNumber:     replicaInfo.ReplicaNumber,
			NoCreate:          true,
		},
	}

	return h.api.Request(h.ctx, msg.TOUCH_APN, request, &msg.EmptyResponse{})
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

var ErrInvalidSize = errors.New("invalid size")

func (h *handle) Truncate(size int64) error {
	if h.origin != nil {
		return h.origin.Truncate(size)
	}

	if size < 0 {
		return ErrInvalidSize
	}

	h.truncateSize = size

	return nil
}

func (h *handle) Touch(mtime time.Time) error {
	if h.origin != nil {
		return h.origin.Touch(mtime)
	}

	if mtime.IsZero() {
		mtime = time.Now()
	}

	h.touchTime = mtime

	return nil
}

var ErrSameConnection = errors.New("same connection")

func (h *handle) Reopen(conn Conn, mode int) (File, error) {
	if h.origin != nil {
		return h.origin.Reopen(conn, mode)
	}

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

	replicaInfo, err := h.GetReplicaAccessInfo()
	if err != nil {
		err = multierr.Append(err, conn.Close())

		return nil, err
	}

	request := msg.DataObjectRequest{
		Path:      h.path,
		OpenFlags: mode &^ O_APPEND,
	}

	request.KeyVals.Add(msg.RESC_HIER_STR_KW, replicaInfo.ResourceHierarchy)
	request.KeyVals.Add(msg.REPLICA_TOKEN_KW, replicaInfo.ReplicaToken)

	h.api.SetFlags(&request.KeyVals)

	h2 := handle{
		api:          h.api,
		conn:         conn,
		ctx:          h.ctx,
		path:         h.path,
		origin:       h,
		truncateSize: -1,
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

type ReplicaAccessInfo struct {
	ReplicaNumber     int
	ReplicaToken      string
	ResourceHierarchy string
}

var ErrIncompleteReplicaAccessInfo = errors.New("incomplete replica access info")

func (h *handle) GetReplicaAccessInfo() (*ReplicaAccessInfo, error) {
	response := msg.GetDescriptorInfoResponse{}

	if err := h.conn.Request(h.ctx, msg.GET_FILE_DESCRIPTOR_INFO_APN, msg.GetDescriptorInfoRequest{FileDescriptor: h.FileDescriptor}, &response); err != nil {
		return nil, err
	}

	info := ReplicaAccessInfo{
		ReplicaToken: response.ReplicaToken,
	}

	i, ok := response.DataObjectInfo["replica_number"]
	if !ok {
		return nil, ErrIncompleteReplicaAccessInfo
	}

	s, ok := response.DataObjectInfo["resource_hierarchy"]
	if !ok {
		return nil, ErrIncompleteReplicaAccessInfo
	}

	var err error

	info.ReplicaNumber, err = toInt(i)
	if err != nil {
		return nil, err
	}

	info.ResourceHierarchy, err = toString(s)
	if err != nil {
		return nil, err
	}

	return &info, nil
}

func toInt(i interface{}) (int, error) {
	asJ, err := json.Marshal(i)
	if err != nil {
		return 0, err
	}

	var number int

	err = json.Unmarshal(asJ, &number)
	if err != nil {
		return 0, err
	}

	return number, nil
}

func toString(i interface{}) (string, error) {
	asJ, err := json.Marshal(i)
	if err != nil {
		return "", err
	}

	var str string

	err = json.Unmarshal(asJ, &str)
	if err != nil {
		return "", err
	}

	return str, nil
}
