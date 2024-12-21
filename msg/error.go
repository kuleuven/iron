package msg

import "fmt"

type IRODSError struct {
	Code    int32
	Message string
}

func (e *IRODSError) Error() string {
	return fmt.Sprintf("IRODS error %d: %s", e.Code, e.Message)
}
