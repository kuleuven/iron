package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"time"

	"gitea.icts.kuleuven.be/coz/iron/msg"
	"github.com/sirupsen/logrus"
	"go.uber.org/multierr"
)

type handle struct {
	api            *API
	ctx            context.Context //nolint:containedctx
	conn           Conn
	path           string
	FileDescriptor msg.FileDescriptor

	// Housekeeping
	origin                    *handle        // If this handle is a reopened handle, this contains the original handle
	wg                        sync.WaitGroup // Waitgroup if the file was reopened, for the reopened handle to be closed
	truncateSize              int64          // If nonnegative, truncate the file to this size after closing
	touchTime                 time.Time      // If non-zero, touch the file to this time after closing
	curOffset                 int64          // Current offset of the file
	unregisterEmergencyCloser func()
	sync.Mutex
}

func (h *handle) Name() string {
	return h.path
}

func (h *handle) Close() error {
	h.unregisterEmergencyCloser()

	if h.origin != nil {
		err := h.closeReopenedHandle()

		err = multierr.Append(err, h.conn.Close())

		return err
	}

	h.wg.Wait()

	h.Lock()
	defer h.Unlock()

	var replicaInfo *ReplicaAccessInfo

	if h.truncateSize >= 0 || !h.touchTime.IsZero() {
		var err error

		replicaInfo, err = h.getReplicaAccessInfo()
		if err != nil {
			err = multierr.Append(err, h.closeOriginalHandle())
			err = multierr.Append(err, h.conn.Close())

			return err
		}
	}

	if err := h.closeOriginalHandle(); err != nil {
		err = multierr.Append(err, h.conn.Close())

		return err
	}

	if err := h.doTruncate(replicaInfo); err != nil {
		err = multierr.Append(err, h.conn.Close())

		return err
	}

	if err := h.doTouch(replicaInfo); err != nil {
		err = multierr.Append(err, h.conn.Close())

		return err
	}

	return h.conn.Close()
}

func (h *handle) closeReopenedHandle() error {
	defer h.origin.wg.Done()

	request := msg.CloseDataObjectReplicaRequest{
		FileDescriptor: h.FileDescriptor,
	}

	return h.conn.Request(context.Background(), msg.REPLICA_CLOSE_APN, request, &msg.EmptyResponse{})
}

func (h *handle) closeOriginalHandle() error {
	request := msg.OpenedDataObjectRequest{
		FileDescriptor: h.FileDescriptor,
	}

	return h.conn.Request(context.Background(), msg.DATA_OBJ_CLOSE_AN, request, &msg.EmptyResponse{})
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

	h.api.setFlags(&request.KeyVals)

	return h.conn.Request(h.ctx, msg.REPLICA_TRUNCATE_AN, request, &msg.EmptyResponse{})
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

	return h.conn.Request(h.ctx, msg.TOUCH_APN, request, &msg.EmptyResponse{})
}

func (h *handle) Seek(offset int64, whence int) (int64, error) {
	h.Lock()
	defer h.Unlock()

	if whence == 0 && offset == h.curOffset {
		return h.curOffset, nil
	}

	if whence == 1 && offset == 0 {
		return h.curOffset, nil
	}

	request := msg.OpenedDataObjectRequest{
		FileDescriptor: h.FileDescriptor,
		Whence:         whence,
		Offset:         offset,
	}

	h.api.setFlags(&request.KeyVals)

	var response msg.SeekResponse

	err := h.conn.Request(h.ctx, msg.DATA_OBJ_LSEEK_AN, request, &response)

	h.curOffset = response.Offset

	return response.Offset, err
}

func (h *handle) Read(b []byte) (int, error) {
	h.Lock()
	defer h.Unlock()

	truncatedSize := h.truncatedSize()

	if truncatedSize >= 0 && truncatedSize <= h.curOffset {
		return 0, io.EOF
	}

	var returnEOF bool

	if truncatedSize >= 0 && h.curOffset+int64(len(b)) > truncatedSize {
		b = b[:truncatedSize-h.curOffset]

		returnEOF = true
	}

	request := msg.OpenedDataObjectRequest{
		FileDescriptor: h.FileDescriptor,
		Size:           len(b),
	}

	h.api.setFlags(&request.KeyVals)

	var response msg.ReadResponse

	if err := h.conn.RequestWithBuffers(h.ctx, msg.DATA_OBJ_READ_AN, request, &response, nil, b); err != nil {
		return 0, err
	}

	n := int(response)
	h.curOffset += int64(n)

	if n < len(b) {
		returnEOF = true
	}

	if returnEOF {
		return n, io.EOF
	}

	return n, nil
}

func (h *handle) Write(b []byte) (int, error) {
	h.Lock()
	defer h.Unlock()

	request := msg.OpenedDataObjectRequest{
		FileDescriptor: h.FileDescriptor,
		Size:           len(b),
	}

	h.api.setFlags(&request.KeyVals)

	if err := h.conn.RequestWithBuffers(h.ctx, msg.DATA_OBJ_WRITE_AN, request, &msg.EmptyResponse{}, b, nil); err != nil {
		return 0, err
	}

	h.curOffset += int64(len(b))

	if h.truncatedSize() >= 0 && h.curOffset > h.truncatedSize() {
		h.setTruncatedSize(h.curOffset)
	}

	return len(b), nil
}

func (h *handle) truncatedSize() int64 {
	if h.origin != nil {
		h.origin.Lock()
		defer h.origin.Unlock()

		return h.origin.truncateSize
	}

	return h.truncateSize
}

func (h *handle) setTruncatedSize(size int64) {
	if h.origin != nil {
		h.origin.Lock()
		defer h.origin.Unlock()

		h.origin.truncateSize = size

		return
	}

	h.truncateSize = size
}

var ErrInvalidSize = errors.New("invalid size")

func (h *handle) Truncate(size int64) error {
	if size < 0 {
		return ErrInvalidSize
	}

	h.Lock()
	defer h.Unlock()

	h.setTruncatedSize(size)

	return nil
}

func (h *handle) Touch(mtime time.Time) error {
	h.Lock()
	defer h.Unlock()

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

func (h *handle) Reopen(conn Conn, mode int) (File, error) { //nolint:funlen
	h.Lock()
	defer h.Unlock()

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

	if conn == h.conn { // Check that the caller didn't provide the same connection
		return nil, ErrSameConnection
	}

	replicaInfo, err := h.getReplicaAccessInfo()
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

	h.api.setFlags(&request.KeyVals)

	h2 := handle{
		api:          h.api,
		conn:         conn,
		ctx:          h.ctx,
		path:         h.path,
		origin:       h,
		truncateSize: -1,
	}

	err = conn.Request(h.ctx, msg.DATA_OBJ_OPEN_AN, request, &h.FileDescriptor)
	if err == nil && mode&O_APPEND != 0 { // Irods does not support O_APPEND, we need to seek to the end
		_, err = h.Seek(0, 2)
	}

	if err != nil {
		err = multierr.Append(err, conn.Close())

		return nil, err
	}

	h.wg.Add(1) // Add to waitgroup

	h2.unregisterEmergencyCloser = conn.RegisterCloseHandler(func() error {
		logrus.Warnf("Emergency close of %s", h2.path)

		return h2.Close()
	})

	return &h2, nil
}

type ReplicaAccessInfo struct {
	ReplicaNumber     int
	ReplicaToken      string
	ResourceHierarchy string
}

var ErrIncompleteReplicaAccessInfo = errors.New("incomplete replica access info")

func (h *handle) getReplicaAccessInfo() (*ReplicaAccessInfo, error) {
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
