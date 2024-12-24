package msg

import (
	"bytes"
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

func TestReadWrite(t *testing.T) {
	var fd FileDescriptor

	objects := []any{
		AuthRequest{},
		&AuthRequest{},
		AuthChallenge{
			Challenge: "test",
		},
		&AuthChallenge{},
		AuthChallengeResponse{
			Response: "test",
			Username: "tes\tt",
		},
		&AuthChallengeResponse{},
		AuthResponse{},
		&AuthResponse{},
		FileDescriptor(8), // 8th item in list, IntInfo is overwitten by Write call so should match
		&fd,
	}

	buf := bytes.NewBuffer(nil)

	for i := 0; i < len(objects); i += 2 {
		if err := Write(buf, objects[i], nil, "test", int32(i)); err != nil {
			t.Fatal(err)
		}
	}

	for i := 0; i < len(objects); i += 2 {
		if info, err := Read(buf, objects[i+1], nil, "test"); err != nil {
			t.Fatal(err)
		} else if info != int32(i) {
			t.Fatalf("expected %d, got %d", i, info)
		}
	}

	for i := 0; i < len(objects); i += 2 {
		marshal1, err := Marshal(objects[i], "test")
		if err != nil {
			t.Fatal(err)
		}

		marshal2, err := Marshal(objects[i+1], "test")
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(marshal1, marshal2) {
			t.Fatal("marshaled objects are not equal:", marshal1, marshal2)
		}
	}
}
