package scramble

import (
	"testing"
	"time"
)

func TestEncodeDecodeA(t *testing.T) {
	passwords := []string{
		"pass",
		"",
		"passw0rd",
		"!-@#%&",
	}

	for i, p := range passwords {
		encoded := EncodeIrodsA(p, i, time.Now())

		decoded, err := DecodeIrodsA(encoded, i)
		if err != nil {
			t.Error(err)

			continue
		}

		if decoded != p {
			t.Errorf("expected '%s', got '%s'", p, decoded)
		}
	}
}
