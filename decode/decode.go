package decode

import (
	"fmt"

	"github.com/kuleuven/iron/msg"
)

var ErrNotARequest = fmt.Errorf("not a request")

var ErrUnknownAPINumber = fmt.Errorf("unknown api number")

func DecodeRequest(m msg.Message, protocol msg.Protocol) (msg.APINumber, interface{}, error) {
	if m.Header.Type != "RODS_API_REQ" {
		return 0, nil, fmt.Errorf("%w: %s", ErrNotARequest, m.Header.Type)
	}

	var obj interface{}

	switch msg.APINumber(m.Header.IntInfo) { //nolint:exhaustive
	case msg.COLL_CREATE_AN, msg.RM_COLL_AN:
		obj = &msg.CreateCollectionRequest{}
	case msg.DATA_OBJ_RENAME_AN, msg.DATA_OBJ_COPY_AN:
		obj = &msg.DataObjectCopyRequest{}
	case msg.DATA_OBJ_UNLINK_AN, msg.DATA_OBJ_CREATE_AN, msg.DATA_OBJ_OPEN_AN, msg.REPLICA_TRUNCATE_AN:
		obj = &msg.DataObjectRequest{}
	case msg.MOD_ACCESS_CONTROL_AN:
		obj = &msg.ModifyAccessRequest{}
	case msg.MOD_AVU_METADATA_AN:
		obj = &msg.ModifyMetadataRequest{}
	case msg.ATOMIC_APPLY_METADATA_OPERATIONS_APN:
		obj = &msg.AtomicMetadataRequest{}
	case msg.GEN_QUERY_AN:
		obj = &msg.QueryRequest{}
	case msg.GET_FILE_DESCRIPTOR_INFO_APN:
		obj = &msg.GetDescriptorInfoRequest{}
	case msg.REPLICA_CLOSE_APN:
		obj = &msg.CloseDataObjectReplicaRequest{}
	case msg.DATA_OBJ_CLOSE_AN, msg.DATA_OBJ_LSEEK_AN, msg.DATA_OBJ_READ_AN, msg.DATA_OBJ_WRITE_AN:
		obj = &msg.OpenedDataObjectRequest{}
	case msg.TOUCH_APN:
		obj = &msg.TouchDataObjectReplicaRequest{}
	case msg.GENERAL_ADMIN_AN:
		obj = &msg.AdminRequest{}
	case msg.FILE_STAT_AN:
		obj = &msg.FileStatRequest{}
	case msg.MOD_DATA_OBJ_META_AN:
		obj = &msg.ModDataObjMetaRequest{}
	case msg.EXEC_MY_RULE_AN:
		obj = &msg.ExecRuleRequest{}
	default:
		return 0, nil, fmt.Errorf("%w: %d", ErrUnknownAPINumber, m.Header.IntInfo)
	}

	if err := msg.Unmarshal(m, protocol, obj); err != nil {
		return 0, nil, err
	}

	return msg.APINumber(m.Header.IntInfo), obj, nil
}

func DecodeResponse(apiNumber msg.APINumber, m msg.Message, protocol msg.Protocol) (interface{}, error) {
	if m.Header.Type != "RODS_API_REPLY" {
		return nil, fmt.Errorf("%w: %s", msg.ErrUnexpectedMessage, m.Header.Type)
	}

	var obj interface{}

	switch apiNumber { //nolint:exhaustive
	case msg.COLL_CREATE_AN, msg.RM_COLL_AN, msg.DATA_OBJ_RENAME_AN, msg.DATA_OBJ_COPY_AN, msg.DATA_OBJ_UNLINK_AN,
		msg.REPLICA_TRUNCATE_AN, msg.MOD_ACCESS_CONTROL_AN, msg.MOD_AVU_METADATA_AN, msg.ATOMIC_APPLY_METADATA_OPERATIONS_APN,
		msg.REPLICA_CLOSE_APN, msg.DATA_OBJ_CLOSE_AN, msg.DATA_OBJ_READ_AN, msg.DATA_OBJ_WRITE_AN, msg.TOUCH_APN, msg.GENERAL_ADMIN_AN, msg.MOD_DATA_OBJ_META_AN:
		obj = &msg.EmptyResponse{}
	case msg.GEN_QUERY_AN:
		obj = &msg.QueryResponse{}
	case msg.DATA_OBJ_CREATE_AN, msg.DATA_OBJ_OPEN_AN:
		var descriptor msg.FileDescriptor

		obj = &descriptor
	case msg.DATA_OBJ_LSEEK_AN:
		obj = &msg.SeekResponse{}
	case msg.GET_FILE_DESCRIPTOR_INFO_APN:
		obj = &msg.GetDescriptorInfoResponse{}
	case msg.FILE_STAT_AN:
		obj = &msg.FileStatResponse{}
	case msg.EXEC_MY_RULE_AN:
		obj = &msg.MsParamArray{}
	default:
		return nil, fmt.Errorf("%w: %d", ErrUnknownAPINumber, apiNumber)
	}

	if err := msg.Unmarshal(m, protocol, obj); err != nil {
		return nil, err
	}

	return obj, nil
}
