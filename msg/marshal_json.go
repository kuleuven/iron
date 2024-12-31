package msg

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

func marshalJSON(obj any, protocol Protocol, msgType string) (*Message, error) {
	jsonBody, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal irods message to json: %w", err)
	}

	xmlObject := BinBytesBuf{
		Length: len(jsonBody),
		Data:   base64.StdEncoding.EncodeToString(jsonBody),
	}

	return Marshal(xmlObject, protocol, msgType)
}

func unmarshalJSON(msg Message, protocol Protocol, obj any) error {
	var xmlObject BinBytesBuf

	if err := Unmarshal(msg, protocol, &xmlObject); err != nil {
		return err
	}

	jsonBody, err := base64.StdEncoding.DecodeString(xmlObject.Data)
	if err != nil {
		return fmt.Errorf("failed to decode base64 data: %w", err)
	}

	// remove trail \x00
	for i := range jsonBody {
		if jsonBody[i] == '\x00' {
			jsonBody = jsonBody[:i]

			break
		}
	}

	return json.Unmarshal(jsonBody, obj)
}
