package msg

import (
	"bytes"
	"io"
	"reflect"
	"testing"
)

func TestMarshal(t *testing.T) {
	for _, proto := range []Protocol{Native, XML} {
		objects := []any{
			AuthRequest{},
			&AuthRequest{},
			AuthChallenge{
				Challenge: "abc",
			},
			&AuthChallenge{},
			AuthChallengeResponse{
				Response: "test",
				Username: "test",
			},
			&AuthChallengeResponse{},
			AuthResponse{},
			&AuthResponse{},
			GetDescriptorInfoRequest{FileDescriptor: 1},
			&GetDescriptorInfoRequest{},
			QueryResponse{
				RowCount:       1,
				AttributeCount: 5,
				TotalRowCount:  1,
				ContinueIndex:  0,
				SQLResult: []SQLResult{
					{AttributeIndex: 500, ResultLen: 1, Values: []string{"1"}},
					{AttributeIndex: 501, ResultLen: 1, Values: []string{"coll_name"}},
					{AttributeIndex: 503, ResultLen: 1, Values: []string{"rods"}},
					{AttributeIndex: 508, ResultLen: 1, Values: []string{"10000"}},
					{AttributeIndex: 509, ResultLen: 0, Values: []string{}},
				},
			},
			&QueryResponse{},
			DataObjectRequest{
				Path: "/test",
			},
			&DataObjectRequest{},
		}

		for i := 0; i < len(objects); i += 2 {
			t.Run(reflect.TypeOf(objects[i]).String(), func(t *testing.T) {
				testMarshal(t, objects[i], objects[i+1], proto)
			})
		}
	}
}

func testMarshal(t *testing.T, obj, ptr any, proto Protocol) {
	marshaled, err := Marshal(obj, proto, "test")
	if err != nil {
		t.Fatal(err)
	}

	err = Unmarshal(*marshaled, proto, ptr)
	if err != nil {
		t.Fatal(err)
	}

	marshaled2, err := Marshal(ptr, proto, "test")
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(marshaled, marshaled2) {
		t.Fatalf("marshaled objects are not equal: %v != %v", marshaled, marshaled2)
	}
}

func TestReadWrite(t *testing.T) {
	var fd FileDescriptor

	for _, proto := range []Protocol{Native, XML} {
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
			testWrite(t, buf, objects[i], proto, i)
		}

		for i := 0; i < len(objects); i += 2 {
			testRead(t, buf, objects[i+1], proto, i)
		}

		for i := 0; i < len(objects); i += 2 {
			testMarshal(t, objects[i], objects[i+1], proto)
		}
	}
}

func testWrite(t *testing.T, w io.Writer, obj any, proto Protocol, i int) {
	if err := Write(w, obj, nil, proto, "test", int32(i)); err != nil {
		t.Fatal(err)
	}
}

func testRead(t *testing.T, r io.Reader, ptr any, proto Protocol, i int) {
	if info, err := Read(r, ptr, nil, proto, "test"); err != nil {
		t.Fatal(err)
	} else if info != int32(i) {
		t.Fatalf("expected %d, got %d", i, info)
	}
}
