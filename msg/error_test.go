package msg

import (
	"testing"
)

func TestError(t *testing.T) {
	errs := []ErrorCode{
		SYS_SOCK_ACCEPT_ERR,
		SYS_USER_NOT_ALLOWED_TO_CONN,
	}

	for _, err := range errs {
		expected := ErrorCodes[err]

		for i := range 1000 {
			testErr := &IRODSError{
				Code: err - ErrorCode(i),
			}

			if testErr.Name() != expected {
				t.Errorf("expected %s, got %s", expected, testErr.Name())
			}
		}
	}
}
