package api

import (
	"slices"
	"testing"

	"github.com/kuleuven/iron/msg"
)

func TestStatPhysicalReplica(t *testing.T) {
	testAPI := newAPI()

	testAPI.Add(msg.FILE_STAT_AN, msg.FileStatRequest{
		Path:              testTestPath,
		ResourceHierarchy: testDemoResc,
		ObjectPath:        testStr,
	}, msg.FileStatResponse{})

	if _, err := testAPI.StatPhysicalReplica(t.Context(), testStr, Replica{PhysicalPath: testTestPath, ResourceHierarchy: testDemoResc}); err != ErrRequiresAdmin {
		t.Error(err)
	}

	if _, err := testAPI.AsAdmin().StatPhysicalReplica(t.Context(), testStr, Replica{PhysicalPath: testTestPath, ResourceHierarchy: testDemoResc}); err != nil {
		t.Error(err)
	}
}

func TestModifyReplicaAttribute(t *testing.T) {
	testAPI := newAPI()

	kv := msg.SSKeyVal{}

	kv.Add("dataComments", "v")
	kv.Add("irodsAdmin", "")

	testAPI.Add(msg.MOD_DATA_OBJ_META_AN, msg.ModDataObjMetaRequest{
		DataObj: msg.DataObjectInfo{
			ObjPath: testStr,
			ReplNum: 0,
		},
		KeyVals: kv,
	}, msg.EmptyResponse{})

	if err := testAPI.ModifyReplicaAttribute(t.Context(), testStr, Replica{PhysicalPath: testTestPath, ResourceHierarchy: testDemoResc}, "dataComments", "v"); err != ErrRequiresAdmin {
		t.Error(err)
	}

	if err := testAPI.AsAdmin().ModifyReplicaAttribute(t.Context(), testStr, Replica{PhysicalPath: testTestPath, ResourceHierarchy: testDemoResc}, "dataComments", "v"); err != nil {
		t.Error(err)
	}
}

func TestRegisterReplica(t *testing.T) {
	testAPI := newAPI()

	kv := msg.SSKeyVal{}

	kv.Add(msg.DATA_TYPE_KW, testGeneric)
	kv.Add(msg.FILE_PATH_KW, testTestPath)
	kv.Add(msg.DEST_RESC_NAME_KW, testStr)
	kv.Add(msg.REG_REPL_KW, "")

	testAPI.Add(msg.PHY_PATH_REG_AN, msg.DataObjectRequest{
		Path:    testStr,
		KeyVals: kv,
	}, msg.EmptyResponse{})

	if err := testAPI.RegisterReplica(t.Context(), testStr, testStr, testTestPath); err != ErrRequiresAdmin {
		t.Error(err)
	}

	if err := testAPI.AsAdmin().RegisterReplica(t.Context(), testStr, testStr, testTestPath); err != nil {
		t.Error(err)
	}
}

func TestAdminCalls(t *testing.T) {
	testAPI := newAPI()

	testAPI.AddResponses(slices.Repeat([]any{msg.EmptyResponse{}}, 10))

	for _, expected := range []error{ErrRequiresAdmin, nil} {
		api := testAPI.API

		if expected == nil {
			api = api.AsAdmin()
		}

		list := []error{
			api.CreateUser(t.Context(), testStr, testRodsUser),
			api.CreateGroup(t.Context(), testStr),
			api.ChangeUserPassword(t.Context(), testStr, testStr),
			api.ChangeUserType(t.Context(), testStr, testRodsUser),
			api.RemoveUser(t.Context(), testStr),
			api.RemoveGroup(t.Context(), testStr),
			api.AddGroupMember(t.Context(), testStr, "test1"),
			api.RemoveGroupMember(t.Context(), testStr, "test1"),
			api.SetUserQuota(t.Context(), testStr, "demooResc", "100"),
			api.SetGroupQuota(t.Context(), testStr, "demooResc", "100"),
		}

		for _, err := range list {
			if err != expected {
				t.Errorf("expected %v, got %v", expected, err)
			}
		}
	}
}

func TestExecuteRule(t *testing.T) {
	testAPI := newAPI()

	testAPI.AddResponse(msg.MsParamArray{})

	if _, err := testAPI.ExecuteExternalRule(t.Context(), testStr, nil, ""); err != ErrRequiresAdmin {
		t.Error(err)
	}

	if _, err := testAPI.AsAdmin().ExecuteExternalRule(t.Context(), testStr, nil, ""); err != nil {
		t.Error(err)
	}
}
