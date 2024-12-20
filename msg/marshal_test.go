package msg

import (
	"reflect"
	"testing"
)

func TestMarshal(t *testing.T) {
	objects := []any{
		AuthRequest{},
		&AuthRequest{},
		AuthChallenge{
			Challenge: "test",
		},
		&AuthChallenge{},
		AuthChallengeResponse{
			Response: "test",
			Username: "test",
		},
		&AuthChallengeResponse{},
		AuthResponse{},
		&AuthResponse{},
	}

	for i := 0; i < len(objects); i += 2 {
		t.Run(reflect.TypeOf(objects[i]).String(), func(t *testing.T) {
			marshaled, err := Marshal(objects[i], "test")
			if err != nil {
				t.Fatal(err)
			}

			err = Unmarshal(*marshaled, objects[i+1])
			if err != nil {
				t.Fatal(err)
			}

			marshaled2, err := Marshal(objects[i+1], "test")
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(marshaled, marshaled2) {
				t.Fatal("marshaled objects are not equal")
			}
		})
	}
}
