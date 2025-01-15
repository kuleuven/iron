package msg

import (
	"errors"
	"testing"
)

func TestError(t *testing.T) {
	errs := []ErrorCode{
		SYS_SOCK_ACCEPT_ERR,
		SYS_USER_NOT_ALLOWED_TO_CONN,
		CATALOG_ALREADY_HAS_ITEM_BY_THAT_NAME,
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

			if native, ok := NativeErrors[err]; ok {
				if errors.Is(testErr, native) {
					continue
				}

				t.Errorf("expected %s to be %s", testErr, native)
			}
		}
	}
}
