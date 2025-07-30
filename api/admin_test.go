package api

import (
	"context"
	"slices"
	"testing"

	"github.com/kuleuven/iron/msg"
)

func TestStatPhysicalReplica(t *testing.T) {
	testAPI := newAPI()

	testAPI.Add(msg.FILE_STAT_AN, msg.FileStatRequest{
		Path:              "/test",
		ResourceHierarchy: "demoResc",
		ObjectPath:        "test",
	}, msg.FileStatResponse{})

	if _, err := testAPI.StatPhysicalReplica(context.Background(), "test", Replica{PhysicalPath: "/test", ResourceHierarchy: "demoResc"}); err != ErrRequiresAdmin {
		t.Error(err)
	}

	if _, err := testAPI.AsAdmin().StatPhysicalReplica(context.Background(), "test", Replica{PhysicalPath: "/test", ResourceHierarchy: "demoResc"}); err != nil {
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
			ObjPath: "test",
			ReplNum: 0,
		},
		KeyVals: kv,
	}, msg.EmptyResponse{})

	if err := testAPI.ModifyReplicaAttribute(context.Background(), "test", Replica{PhysicalPath: "/test", ResourceHierarchy: "demoResc"}, "dataComments", "v"); err != ErrRequiresAdmin {
		t.Error(err)
	}

	if err := testAPI.AsAdmin().ModifyReplicaAttribute(context.Background(), "test", Replica{PhysicalPath: "/test", ResourceHierarchy: "demoResc"}, "dataComments", "v"); err != nil {
		t.Error(err)
	}
}

func TestRegisterReplica(t *testing.T) {
	testAPI := newAPI()

	kv := msg.SSKeyVal{}

	kv.Add(msg.DATA_TYPE_KW, "generic")
	kv.Add(msg.FILE_PATH_KW, "/test")
	kv.Add(msg.DEST_RESC_NAME_KW, "test")
	kv.Add(msg.REG_REPL_KW, "")

	testAPI.Add(msg.PHY_PATH_REG_AN, msg.DataObjectRequest{
		Path:    "test",
		KeyVals: kv,
	}, msg.EmptyResponse{})

	if err := testAPI.RegisterReplica(context.Background(), "test", "test", "/test"); err != ErrRequiresAdmin {
		t.Error(err)
	}

	if err := testAPI.AsAdmin().RegisterReplica(context.Background(), "test", "test", "/test"); err != nil {
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
			api.CreateUser(context.Background(), "test", "rodsuser"),
			api.CreateGroup(context.Background(), "test"),
			api.ChangeUserPassword(context.Background(), "test", "test"),
			api.ChangeUserType(context.Background(), "test", "rodsuser"),
			api.RemoveUser(context.Background(), "test"),
			api.RemoveGroup(context.Background(), "test"),
			api.AddGroupMember(context.Background(), "test", "test1"),
			api.RemoveGroupMember(context.Background(), "test", "test1"),
			api.SetUserQuota(context.Background(), "test", "demooResc", "100"),
			api.SetGroupQuota(context.Background(), "test", "demooResc", "100"),
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

	if _, err := testAPI.ExecuteExternalRule(context.Background(), "test", nil, ""); err != ErrRequiresAdmin {
		t.Error(err)
	}

	if _, err := testAPI.AsAdmin().ExecuteExternalRule(context.Background(), "test", nil, ""); err != nil {
		t.Error(err)
	}
}
