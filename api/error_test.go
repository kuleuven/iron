package api

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/kuleuven/iron/msg"
	"go.uber.org/multierr"
)

func TestError(t *testing.T) {
	testErr := &msg.IRODSError{
		Code: -2001,
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

	if !Is(testErr, -2001) {
		t.Error("Is(err, -2001) is false")
	}

	if !Is(testErr, -2000) {
		t.Error("Is(err, -2000) is false")
	}

	if Is(testErr, -3000) {
		t.Error("Is(err, -3000) is true")
	}
}
