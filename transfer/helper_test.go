package transfer

import (
	"context"
	"errors"
	"testing"

	"github.com/kuleuven/iron/api"
	"github.com/kuleuven/iron/msg"
)

func TestRemoteCopyMetadata(t *testing.T) {
	shared := api.Metadata{Name: "shared", Value: "value", Units: "unit"}
	missing := api.Metadata{Name: "missing", Value: "value"}
	differentValue := api.Metadata{Name: shared.Name, Value: "different", Units: shared.Units}
	differentUnits := api.Metadata{Name: shared.Name, Value: shared.Value, Units: "different"}
	targetOnly := api.Metadata{Name: "target-only", Value: "keep"}
	requestError := errors.New("metadata request failed")

	tests := []struct {
		name                          string
		source, target, add           []api.Metadata
		sourceErr, targetErr, editErr error
	}{
		{name: "both empty"},
		{name: "identical", source: []api.Metadata{shared}, target: []api.Metadata{shared}},
		{name: "empty source", target: []api.Metadata{targetOnly}},
		{name: "empty target", source: []api.Metadata{missing}, add: []api.Metadata{missing}},
		{
			name:   "add missing triples and preserve target-only entries",
			source: []api.Metadata{shared, missing, differentValue, differentUnits},
			target: []api.Metadata{shared, targetOnly},
			add:    []api.Metadata{missing, differentValue, differentUnits},
		},
		{
			name:   "duplicate source entries",
			source: []api.Metadata{missing, missing},
			add:    []api.Metadata{missing},
		},
		{name: "source listing fails", sourceErr: requestError},
		{name: "target listing fails", targetErr: requestError},
		{name: "modification fails", source: []api.Metadata{missing}, add: []api.Metadata{missing}, editErr: requestError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sourceConn := &api.MockConn{}
			targetConn := &api.MockConn{}

			addMetadataResponse(sourceConn, tt.source, msg.ICAT_COLUMN_META_DATA_ATTR_NAME, tt.sourceErr)

			if tt.sourceErr == nil {
				addMetadataResponse(targetConn, tt.target, msg.ICAT_COLUMN_META_COLL_ATTR_NAME, tt.targetErr)
			}

			addMetadataModification(targetConn, tt.add, tt.editErr)

			sourceAPI := &api.API{Connect: func(context.Context) (api.Conn, error) { return sourceConn, nil }}
			targetAPI := &api.API{Admin: true, Connect: func(context.Context) (api.Conn, error) { return targetConn, nil }}

			err := RemoteCopyMetadata(t.Context(), sourceAPI, targetAPI, testCopySrc, api.DataObjectType, testCopyDest, api.CollectionType)
			wantErr := errors.Join(tt.sourceErr, tt.targetErr, tt.editErr)

			if (wantErr == nil && err != nil) || (wantErr != nil && !errors.Is(err, requestError)) {
				t.Fatalf("RemoteCopyMetadata() error = %v, want %v", err, wantErr)
			}

			if len(sourceConn.Dialog) != 0 || len(targetConn.Dialog) != 0 {
				t.Fatal("not all expected metadata requests were sent")
			}
		})
	}
}

func addMetadataModification(conn *api.MockConn, add []api.Metadata, err error) {
	if len(add) == 0 {
		return
	}

	request := &msg.AtomicMetadataRequest{
		AdminMode: true,
		ItemName:  testCopyDest,
		ItemType:  api.CollectionType.String(),
	}
	for _, metadata := range add {
		request.Operations = append(request.Operations, msg.MetadataOperation{
			Operation: "add",
			Name:      metadata.Name,
			Value:     metadata.Value,
			Units:     metadata.Units,
		})
	}

	var response any = msg.EmptyResponse{}

	if err != nil {
		response = err
	}

	conn.Add(msg.ATOMIC_APPLY_METADATA_OPERATIONS_APN, request, response)
}

func addMetadataResponse(conn *api.MockConn, metadata []api.Metadata, nameColumn msg.ColumnNumber, err error) {
	if err != nil {
		conn.AddResponse(err)

		return
	}

	conn.AddResponse(metadataQueryResponse(metadata, nameColumn))
}

func metadataQueryResponse(metadata []api.Metadata, nameColumn msg.ColumnNumber) msg.QueryResponse {
	response := msg.QueryResponse{
		RowCount:       len(metadata),
		AttributeCount: 3,
		TotalRowCount:  len(metadata),
		SQLResult: []msg.SQLResult{
			{AttributeIndex: nameColumn, ResultLen: len(metadata)},
			{AttributeIndex: nameColumn + 1, ResultLen: len(metadata)},
			{AttributeIndex: nameColumn + 2, ResultLen: len(metadata)},
		},
	}

	for _, value := range metadata {
		response.SQLResult[0].Values = append(response.SQLResult[0].Values, value.Name)
		response.SQLResult[1].Values = append(response.SQLResult[1].Values, value.Value)
		response.SQLResult[2].Values = append(response.SQLResult[2].Values, value.Units)
	}

	return response
}
