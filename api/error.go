package api

import (
	"errors"

	"github.com/kuleuven/iron/msg"
)

func ErrorCode(err error) (msg.ErrorCode, bool) {
	if err == nil {
		return 0, false
	}

	rodsErr := &msg.IRODSError{}

	if errors.As(err, &rodsErr) {
		return rodsErr.Code, true
	}

	return 0, false
}

func Is(err error, code msg.ErrorCode) bool {
	errCode, ok := ErrorCode(err)
	if !ok {
		return false
	}

	if code == errCode {
		return true
	}

	// If we compare against a multiple of 1000, try
	// to round the received error code. IRODS adds
	// 1 - 999 to the error code for subcodes.
	if code%1000 == 0 && code/1000 == errCode/1000 {
		return true
	}

	return ok && code == errCode
}
