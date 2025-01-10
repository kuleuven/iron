package api

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"gitea.icts.kuleuven.be/coz/iron/msg"
	"go.uber.org/multierr"
)

func TestError(t *testing.T) {
	testErr := &msg.IRODSError{
		Code: -1,
	}

	assert := func(err error) {
		code, ok := ErrorCode(err)
		if !ok {
			t.Errorf("expected *msg.IRODSError, got %s", reflect.TypeOf(err))
		} else if code != testErr.Code {
			t.Errorf("expected %d, got %d", testErr.Code, code)
		}
	}

	assert(testErr)
	assert(fmt.Errorf("%w: test", testErr))
	assert(multierr.Append(fmt.Errorf("%w: wrap", testErr), os.ErrInvalid))
	assert(multierr.Append(os.ErrInvalid, fmt.Errorf("%w: reverse wrap", testErr)))

	if !Is(testErr, -1) {
		t.Error("Is(err, -1) is false")
	}
}
