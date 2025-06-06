package msg

import (
	"bytes"
	"encoding/xml"
	"testing"
)

func TestXMLMarshal(t *testing.T) {
	badString := "\t\n\r'\"<"

	buf := &bytes.Buffer{}

	if err := xml.NewEncoder(buf).Encode(badString); err != nil {
		t.Fatal(err)
	}

	preprocessed, err := PreprocessXML(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	expected := []byte("<string>\t\n\r&apos;&quot;&lt;</string>")

	if !bytes.Equal(preprocessed, expected) {
		t.Fatalf("expected %s, got %s", expected, preprocessed)
	}

	postprocessed, err := PostprocessXML(preprocessed)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(postprocessed, preprocessed) {
		t.Fatalf("expected %s, got %s", preprocessed, postprocessed)
	}
}

func TestXMLUnmarshalError(t *testing.T) {
	msg := Message{
		Header: Header{
			Type:     "test",
			ErrorLen: 5,
		},
		Body: Body{
			Message: []byte{},
			Error:   []byte("error"),
		},
		Bin: []byte{},
	}

	var obj AuthChallengeResponse

	if err := unmarshalXML(msg, &obj); err == nil {
		t.Fatal("expected error, got nil")
	}
}
