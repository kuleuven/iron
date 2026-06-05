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

func TestErrorString(t *testing.T) {
	noMsg := &IRODSError{Code: SYS_SOCK_ACCEPT_ERR}
	if got, want := noMsg.Error(), "IRODS error SYS_SOCK_ACCEPT_ERR"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}

	withMsg := &IRODSError{Code: SYS_SOCK_ACCEPT_ERR, Message: "boom"}
	if got, want := withMsg.Error(), "IRODS error SYS_SOCK_ACCEPT_ERR: boom"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}

	// Unknown error code falls back to the numeric representation.
	unknown := &IRODSError{Code: ErrorCode(-1)}
	if got, want := unknown.Error(), "IRODS error -1"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}

	// Unwrap returns nil when no native error mapping exists.
	if err := (&IRODSError{Code: ErrorCode(-1)}).Unwrap(); err != nil {
		t.Errorf("Unwrap() = %v, want nil", err)
	}
}
