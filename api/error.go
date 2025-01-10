package api

import (
	"errors"

	"gitea.icts.kuleuven.be/coz/iron/msg"
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

	return ok && code == errCode
}
