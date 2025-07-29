package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/kuleuven/iron/msg"
	"github.com/sirupsen/logrus"
	"go.uber.org/multierr"
)

type object struct {
	api          *API
	ctx          context.Context //nolint:containedctx
	path         string
	actualSize   int64          // If nonnegative, the actual size of the file. If negative, the actual size is unknown
	truncateSize int64          // If nonnegative and less than actualSize, truncate the file to this size after closing
	touchTime    time.Time      // If non-zero, touch the file to this time after closing
	wg           sync.WaitGroup // Waitgroup for the reopened handle to be closed
	sync.Mutex
}

func (o *object) ActualSize() int64 {
	o.Lock()
	defer o.Unlock()

	return o.actualSize
}

func (o *object) TruncatedSize() int64 {
	o.Lock()
	defer o.Unlock()

	return o.truncateSize
}

func (o *object) SetActualSize(size int64) {
	o.Lock()
	defer o.Unlock()

	o.actualSize = size
}

func (o *object) SetTruncatedSize(size int64) {
	o.Lock()
	defer o.Unlock()

	o.truncateSize = size
}

func (o *object) SetTouchTime(t time.Time) {
	o.Lock()
	defer o.Unlock()

	o.touchTime = t
}

func (o *object) IncreaseSizesIfNeeded(size int64) {
	o.Lock()
	defer o.Unlock()

	if size > o.truncateSize && o.truncateSize >= 0 {
		o.truncateSize = size
	}

	if size > o.actualSize && o.actualSize >= 0 {
		o.actualSize = size
	}
}

type handle struct {
	object                    *object
	reopened                  bool
	conn                      Conn
	fileDescriptor            msg.FileDescriptor
	curOffset                 int64  // Current offset of the file
	unregisterEmergencyCloser func() // Function to unregister the emergency closer
	sync.Mutex
}

func (h *handle) Name() string {
	return h.object.path
}

func (h *handle) Size() (int64, error) {
	if size := h.object.ActualSize(); size >= 0 {
		return size, nil
	}

	h.Lock()
	defer h.Unlock()

	offset := h.curOffset

	size, err := h.seek(0, 2)
	if err != nil {
		return 0, err
	}

	_, err = h.seek(offset, 0)

	return size, err
}

func (h *handle) Close() error {
	h.unregisterEmergencyCloser()

	if h.reopened {
		h.Lock()
		defer h.Unlock()

		defer h.object.wg.Done()

		err := h.doCloseReopened()
		err = multierr.Append(err, h.conn.Close())

		return err
	}

	h.object.wg.Wait()

	h.Lock()
	defer h.Unlock()

	// If the file was truncated to the actual size, don't truncate it
	if h.object.truncateSize == h.object.actualSize {
		h.object.truncateSize = -1
	}

	var replicaInfo *ReplicaAccessInfo

	// Request the replica info if the file needs to be truncated or touched
	if h.object.truncateSize >= 0 || !h.object.touchTime.IsZero() {
		var err error

		replicaInfo, err = h.getReplicaAccessInfo()
		if err != nil {
			err = multierr.Append(err, h.doCloseHandle())
			err = multierr.Append(err, h.conn.Close())

			return err
		}
	}

	if err := h.doCloseHandle(); err != nil {
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

func (h *handle) doCloseReopened() error {
	request := msg.CloseDataObjectReplicaRequest{
		FileDescriptor: h.fileDescriptor,
	}

	return h.conn.Request(context.Background(), msg.REPLICA_CLOSE_APN, request, &msg.EmptyResponse{})
}

func (h *handle) doCloseHandle() error {
	request := msg.OpenedDataObjectRequest{
		FileDescriptor: h.fileDescriptor,
	}

	return h.conn.Request(context.Background(), msg.DATA_OBJ_CLOSE_AN, request, &msg.EmptyResponse{})
}

func (h *handle) doTruncate(replicaInfo *ReplicaAccessInfo) error {
	if h.object.truncateSize < 0 {
		return nil
	}

	request := msg.DataObjectRequest{
		Path: h.object.path,
		Size: h.object.truncateSize,
	}

	request.KeyVals.Add(msg.RESC_HIER_STR_KW, replicaInfo.ResourceHierarchy)
	request.KeyVals.Add(msg.REPLICA_TOKEN_KW, replicaInfo.ReplicaToken)

	h.object.api.setFlags(&request.KeyVals)

	return h.conn.Request(h.object.ctx, msg.REPLICA_TRUNCATE_AN, request, &msg.EmptyResponse{})
}

func (h *handle) doTouch(replicaInfo *ReplicaAccessInfo) error {
	if h.object.touchTime.IsZero() {
		return nil
	}

	request := msg.TouchDataObjectReplicaRequest{
		Path: h.object.path,
		Options: msg.TouchOptions{
			SecondsSinceEpoch: h.object.touchTime.Unix(),
			ReplicaNumber:     replicaInfo.ReplicaNumber,
			NoCreate:          true,
		},
	}

	return h.conn.Request(h.object.ctx, msg.TOUCH_APN, request, &msg.EmptyResponse{})
}

func (h *handle) Seek(offset int64, whence int) (int64, error) {
	h.Lock()
	defer h.Unlock()

	return h.seek(offset, whence)
}

func (h *handle) seek(offset int64, whence int) (int64, error) {
	if whence == 0 && offset == h.curOffset {
		return h.curOffset, nil
	}

	if whence == 1 && offset == 0 {
		return h.curOffset, nil
	}

	request := msg.OpenedDataObjectRequest{
		FileDescriptor: h.fileDescriptor,
		Whence:         whence,
		Offset:         offset,
	}

	h.object.api.setFlags(&request.KeyVals)

	var response msg.SeekResponse

	if err := h.conn.Request(h.object.ctx, msg.DATA_OBJ_LSEEK_AN, request, &response); err != nil {
		return response.Offset, err
	}

	h.curOffset = response.Offset

	if whence == 2 {
		h.object.SetActualSize(response.Offset - offset)
	}

	return response.Offset, nil
}

func (h *handle) Read(b []byte) (int, error) {
	h.Lock()
	defer h.Unlock()

	truncatedSize := h.object.TruncatedSize()

	if truncatedSize >= 0 && truncatedSize <= h.curOffset {
		return 0, io.EOF
	}

	var returnEOF bool

	if truncatedSize >= 0 && h.curOffset+int64(len(b)) > truncatedSize {
		b = b[:truncatedSize-h.curOffset]

		returnEOF = true
	}

	request := msg.OpenedDataObjectRequest{
		FileDescriptor: h.fileDescriptor,
		Size:           len(b),
	}

	h.object.api.setFlags(&request.KeyVals)

	var response msg.ReadResponse

	if err := h.conn.RequestWithBuffers(h.object.ctx, msg.DATA_OBJ_READ_AN, request, &response, nil, b); err != nil {
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
		FileDescriptor: h.fileDescriptor,
		Size:           len(b),
	}

	h.object.api.setFlags(&request.KeyVals)

	if err := h.conn.RequestWithBuffers(h.object.ctx, msg.DATA_OBJ_WRITE_AN, request, &msg.EmptyResponse{}, b, nil); err != nil {
		return 0, err
	}

	h.curOffset += int64(len(b))

	h.object.IncreaseSizesIfNeeded(h.curOffset)

	return len(b), nil
}

var ErrInvalidSize = errors.New("invalid size")

func (h *handle) Truncate(size int64) error {
	if size < 0 {
		return ErrInvalidSize
	}

	h.object.SetTruncatedSize(size)

	return nil
}

func (h *handle) Touch(mtime time.Time) error {
	if mtime.IsZero() {
		mtime = time.Now()
	}

	h.object.SetTouchTime(mtime)

	return nil
}

var ErrSameConnection = errors.New("same connection")

var ErrCannotTruncate = errors.New("cannot truncate in reopened handle")

func (h *handle) Reopen(conn Conn, mode int) (File, error) {
	h.Lock()
	defer h.Unlock()

	if conn == nil {
		var err error

		conn, err = h.object.api.Connect(h.object.ctx)
		if err != nil {
			return nil, err
		}
	}

	if conn == h.conn { // Check that the caller didn't provide the same connection
		return nil, ErrSameConnection
	}

	if mode&O_TRUNC != 0 {
		return nil, multierr.Append(ErrCannotTruncate, conn.Close())
	}

	replicaInfo, err := h.getReplicaAccessInfo()
	if err != nil {
		err = multierr.Append(err, conn.Close())

		return nil, err
	}

	request := msg.DataObjectRequest{
		Path:      h.object.path,
		OpenFlags: mode &^ O_APPEND,
	}

	request.KeyVals.Add(msg.RESC_HIER_STR_KW, replicaInfo.ResourceHierarchy)
	request.KeyVals.Add(msg.REPLICA_TOKEN_KW, replicaInfo.ReplicaToken)

	h.object.api.setFlags(&request.KeyVals)

	h2 := handle{
		object:   h.object,
		conn:     conn,
		reopened: true,
	}

	err = conn.Request(h.object.ctx, msg.DATA_OBJ_OPEN_AN, request, &h2.fileDescriptor)
	if err == nil && mode&O_APPEND != 0 { // Irods does not support O_APPEND, we need to seek to the end
		_, err = h.Seek(0, 2)
	}

	if err != nil {
		err = multierr.Append(err, conn.Close())

		return nil, err
	}

	h.object.wg.Add(1) // Add to waitgroup

	h2.unregisterEmergencyCloser = conn.RegisterCloseHandler(func() error {
		logrus.Warnf("Emergency close of %s", h.object.path)

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

	if err := h.conn.Request(h.object.ctx, msg.GET_FILE_DESCRIPTOR_INFO_APN, msg.GetDescriptorInfoRequest{FileDescriptor: h.fileDescriptor}, &response); err != nil {
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
