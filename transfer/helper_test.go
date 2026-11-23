package transfer

import (
	"errors"
	"fmt"
	"testing"

	"github.com/kuleuven/iron/api"
	"github.com/kuleuven/iron/msg"
)

func TestRemoteCopyMetadata(t *testing.T) {
	shared := api.Metadata{Name: "shared", Value: testValue, Units: "unit"}
	missing := api.Metadata{Name: "missing", Value: testValue}
	differentValue := api.Metadata{Name: shared.Name, Value: testDifferent, Units: shared.Units}
	differentUnits := api.Metadata{Name: shared.Name, Value: shared.Value, Units: testDifferent}
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

			sourceAPI, targetAPI := mockTransferAPI(sourceConn), mockTransferAPI(targetConn)
			sourceAPI.Zone, targetAPI.Zone, targetAPI.Admin = testZone, testZone, true

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

func TestRemoteCopyACL(t *testing.T) {
	sourceConn, targetConn := &api.MockConn{}, &api.MockConn{}
	sourceConn.AddResponse(copySourceResponse())
	addACLDialog(sourceConn, api.DataObjectType)
	addACLModification(targetConn)

	sourceAPI := mockTransferAPI(sourceConn)
	targetAPI := mockTransferAPI(targetConn)

	if err := RemoteCopyACL(t.Context(), sourceAPI, targetAPI, testCopySrc, api.DataObjectType, testCopyDest); err != nil {
		t.Fatal(err)
	}

	assertTransferDialogs(t, sourceConn, targetConn)
}

func TestRemoteCopyCollectionACLInheritance(t *testing.T) {
	for _, inherit := range []bool{false, true} {
		t.Run(fmt.Sprintf("inherit=%t", inherit), func(t *testing.T) {
			sourceConn, targetConn := &api.MockConn{}, &api.MockConn{}
			sourceConn.AddResponse(msg.QueryResponse{})
			sourceConn.AddResponse(collectionResponse(inherit))
			addACLDialog(sourceConn, api.CollectionType)
			addACLModification(targetConn)

			inheritance := testNoInherit
			if inherit {
				inheritance = testInherit
			}

			targetConn.Add(msg.MOD_ACCESS_CONTROL_AN, msg.ModifyAccessRequest{
				Path:        testCopyDest,
				AccessLevel: inheritance,
			}, msg.EmptyResponse{})

			sourceAPI := mockTransferAPI(sourceConn)
			targetAPI := mockTransferAPI(targetConn)

			if err := RemoteCopyACL(t.Context(), sourceAPI, targetAPI, testCopySrc, api.CollectionType, testCopyDest); err != nil {
				t.Fatal(err)
			}

			assertTransferDialogs(t, sourceConn, targetConn)
		})
	}
}

func TestCopyACLCollectionInheritance(t *testing.T) {
	targetConn := &api.MockConn{}
	targetConn.Add(msg.MOD_ACCESS_CONTROL_AN, msg.ModifyAccessRequest{
		Path:        testCopyDest,
		UserName:    testUser,
		Zone:        testZone,
		AccessLevel: testOwnPermission,
	}, msg.EmptyResponse{})
	targetConn.Add(msg.MOD_ACCESS_CONTROL_AN, msg.ModifyAccessRequest{
		Path:        testCopyDest,
		AccessLevel: testInherit,
	}, msg.EmptyResponse{})

	targetAPI := mockTransferAPI(targetConn)
	record := &testCollectionRecord{
		Collection: &api.Collection{Inheritance: true},
		access:     []api.Access{{User: api.User{Name: testUser, Zone: testZone}, Permission: testOwnPermission}},
	}

	attributes, err := ComputeAttributeChanges(record, nil, false, true, true)
	if err != nil {
		t.Fatal(err)
	}

	if err := attributes.Apply(t.Context(), targetAPI, testCopyDest); err != nil {
		t.Fatal(err)
	}

	assertTransferDialogs(t, targetConn)
}

func TestAttributeChangesMetadataStrategy(t *testing.T) {
	source := &testCopyRecord{
		DataObject: &api.DataObject{Path: testCopySrc},
		metadata:   []api.Metadata{{Name: testMetadataName, Value: testValue}},
	}
	target := &testCopyRecord{
		DataObject: &api.DataObject{Path: testCopyDest},
		metadata:   []api.Metadata{{Name: testMetadataName, Value: testDifferent}},
	}

	for _, tc := range []struct {
		name        string
		sameZone    bool
		copyFrom    string
		wantAdd     int
		wantChanges bool
	}{
		{name: "same zone", sameZone: true, copyFrom: testCopySrc, wantChanges: true},
		{name: "different zones", wantAdd: 1, wantChanges: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changes, err := ComputeAttributeChanges(source, target, true, false, tc.sameZone)
			if err != nil {
				t.Fatal(err)
			}

			if changes.MetadataToCopyFrom != tc.copyFrom {
				t.Errorf("MetadataToCopyFrom = %q, want %q", changes.MetadataToCopyFrom, tc.copyFrom)
			}

			if len(changes.MetadataToAdd) != tc.wantAdd {
				t.Errorf("MetadataToAdd = %v, want %d entries", changes.MetadataToAdd, tc.wantAdd)
			}

			if changes.HasChanges() != tc.wantChanges {
				t.Errorf("hasChanges() = %t, want %t", changes.HasChanges(), tc.wantChanges)
			}
		})
	}

	t.Run("identical metadata", func(t *testing.T) {
		target.metadata = append([]api.Metadata(nil), source.metadata...)

		changes, err := ComputeAttributeChanges(source, target, true, false, true)
		if err != nil {
			t.Fatal(err)
		}

		if changes.HasChanges() {
			t.Fatalf("identical metadata produced changes: %+v", changes)
		}
	})
}

func TestAttributeChangesApplyCopiesMetadataAndACL(t *testing.T) {
	targetConn := &api.MockConn{}
	targetConn.Add(msg.MOD_AVU_METADATA_AN, msg.ModifyMetadataRequest{
		Operation: "cp",
		ItemType:  "-d",
		ItemName:  "-d",
		AttrName:  testCopySrc,
		AttrValue: testCopyDest,
	}, msg.EmptyResponse{})
	targetConn.Add(msg.MOD_ACCESS_CONTROL_AN, msg.ModifyAccessRequest{
		Path:        testCopyDest,
		UserName:    testUser,
		Zone:        testZone,
		AccessLevel: testOwnPermission,
	}, msg.EmptyResponse{})
	targetConn.Add(msg.MOD_ACCESS_CONTROL_AN, msg.ModifyAccessRequest{
		Path:        testCopyDest,
		AccessLevel: testInherit,
	}, msg.EmptyResponse{})

	inheritance := true
	changes := AttributeChanges{
		MetadataToCopyFrom:    testCopySrc,
		ItemType:              api.DataObjectType,
		AccessToAdd:           []api.Access{{User: api.User{Name: testUser, Zone: testZone}, Permission: testOwnPermission}},
		CollectionInheritance: &inheritance,
	}

	if err := changes.Apply(t.Context(), mockTransferAPI(targetConn), testCopyDest); err != nil {
		t.Fatal(err)
	}

	assertTransferDialogs(t, targetConn)
}

func collectionResponse(inherit bool) msg.QueryResponse {
	inheritance := "0"
	if inherit {
		inheritance = "1"
	}

	return msg.QueryResponse{
		RowCount:       1,
		AttributeCount: 7,
		TotalRowCount:  1,
		SQLResult: []msg.SQLResult{
			{AttributeIndex: 500, ResultLen: 1, Values: []string{"1"}},
			{AttributeIndex: 503, ResultLen: 1, Values: []string{testUser}},
			{AttributeIndex: 504, ResultLen: 1, Values: []string{testZone}},
			{AttributeIndex: 508, ResultLen: 1, Values: []string{"10000"}},
			{AttributeIndex: 509, ResultLen: 1, Values: []string{"10000"}},
			{AttributeIndex: 506, ResultLen: 1, Values: []string{inheritance}},
			{AttributeIndex: 512, ResultLen: 1, Values: []string{"{}"}},
		},
	}
}

func addACLDialog(conn *api.MockConn, itemType api.ObjectType) {
	accessColumn, userColumn := msg.ICAT_COLUMN_DATA_ACCESS_NAME, msg.ICAT_COLUMN_DATA_ACCESS_USER_ID
	if itemType == api.CollectionType {
		accessColumn, userColumn = msg.ICAT_COLUMN_COLL_ACCESS_NAME, msg.ICAT_COLUMN_COLL_ACCESS_USER_ID
	}

	conn.AddResponse(msg.QueryResponse{
		RowCount:       1,
		AttributeCount: 2,
		TotalRowCount:  1,
		SQLResult: []msg.SQLResult{
			{AttributeIndex: accessColumn, ResultLen: 1, Values: []string{testOwnPermission}},
			{AttributeIndex: userColumn, ResultLen: 1, Values: []string{"1"}},
		},
	})
	conn.AddResponse(msg.QueryResponse{
		RowCount:       1,
		AttributeCount: 6,
		TotalRowCount:  1,
		SQLResult: []msg.SQLResult{
			{AttributeIndex: msg.ICAT_COLUMN_USER_ID, ResultLen: 1, Values: []string{"1"}},
			{AttributeIndex: msg.ICAT_COLUMN_USER_NAME, ResultLen: 1, Values: []string{testUser}},
			{AttributeIndex: msg.ICAT_COLUMN_USER_ZONE, ResultLen: 1, Values: []string{testZone}},
			{AttributeIndex: msg.ICAT_COLUMN_USER_TYPE, ResultLen: 1, Values: []string{"rodsuser"}},
			{AttributeIndex: msg.ICAT_COLUMN_USER_CREATE_TIME, ResultLen: 1, Values: []string{"10000"}},
			{AttributeIndex: msg.ICAT_COLUMN_USER_MODIFY_TIME, ResultLen: 1, Values: []string{"10000"}},
		},
	})
}

type testCollectionRecord struct {
	*api.Collection
	metadata []api.Metadata
	access   []api.Access
}

func (record testCollectionRecord) Metadata() []api.Metadata {
	return record.metadata
}

func (record testCollectionRecord) Access() []api.Access {
	return record.access
}

func (testCollectionRecord) Type() api.ObjectType {
	return api.CollectionType
}

func addACLModification(conn *api.MockConn) {
	request := msg.ModifyAccessRequest{
		Path:        testCopyDest,
		UserName:    testUser,
		Zone:        testZone,
		AccessLevel: testOwnPermission,
	}
	conn.Add(msg.MOD_ACCESS_CONTROL_AN, request, msg.EmptyResponse{})
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
			Operation: testMetadataAdd,
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
