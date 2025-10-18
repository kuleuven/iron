package transfer

import (
	"testing"
	"time"
)

func TestWheel(t *testing.T) {
	wheel(time.Second * -5)
}
