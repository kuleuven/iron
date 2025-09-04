package msg

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/sirupsen/logrus"
)

func marshalJSON(obj any, protocol Protocol, msgType string) (*Message, error) {
	jsonBody, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal irods message to json: %w", err)
	}

	logrus.Tracef("-> json: %s", jsonBody)

	xmlObject := BinBytesBuf{
		Length: len(jsonBody),
		Data:   string(jsonBody),
	}

	// only base64 encode for XML
	if protocol == XML {
		xmlObject.Data = base64.StdEncoding.EncodeToString(jsonBody)
	}

	return Marshal(xmlObject, protocol, msgType)
}

func unmarshalJSON(msg Message, protocol Protocol, obj any) error {
	var xmlObject BinBytesBuf

	err := Unmarshal(msg, protocol, &xmlObject)
	if err != nil {
		return err
	}

	jsonBody := []byte(xmlObject.Data)

	// only base64 decode for XML
	if protocol == XML {
		jsonBody, err = base64.StdEncoding.DecodeString(xmlObject.Data)
		if err != nil {
			return fmt.Errorf("failed to decode base64 data: %w", err)
		}
	}

	// remove trail \x00
	for i := range jsonBody {
		if jsonBody[i] == '\x00' {
			jsonBody = jsonBody[:i]

			break
		}
	}

	logrus.Tracef("<- json: %s", jsonBody)

	return json.Unmarshal(jsonBody, obj)
}
