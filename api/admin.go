package api

import (
	"context"
	"errors"
	"fmt"

	"github.com/kuleuven/iron/msg"
	"github.com/kuleuven/iron/scramble"
)

var ErrRequiresAdmin = errors.New("this API call requires api.Admin = true")

// StatPhysicalReplica executes a system stat on the physical replica file.
// This is an administrative call, a connection using a rodsadmin is required.
func (api *API) StatPhysicalReplica(ctx context.Context, path string, replica Replica) (*msg.FileStatResponse, error) {
	if !api.Admin {
		return nil, ErrRequiresAdmin
	}

	request := msg.FileStatRequest{
		Path:              replica.PhysicalPath,
		ResourceHierarchy: replica.ResourceHierarchy,
		ObjectPath:        path,
	}

	var response msg.FileStatResponse

	if err := api.Request(ctx, msg.FILE_STAT_AN, request, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// ModifyReplicaAttribute repairs an attribute of a data object.
// It is equivalent to iadmin modrepl. The keyword can be e.g. dataComments, replStatus, dataSize.
// This is an administrative call, a connection using a rodsadmin is required.
func (api *API) ModifyReplicaAttribute(ctx context.Context, path string, replica Replica, key msg.KeyWord, value string) error {
	if !api.Admin {
		return ErrRequiresAdmin
	}

	request := &msg.ModDataObjMetaRequest{
		DataObj: msg.DataObjectInfo{
			ObjPath: path,
			ReplNum: replica.Number,
		},
	}

	request.KeyVals.Add(key, value)
	request.KeyVals.Add(msg.ADMIN_KW, "")

	return api.Request(ctx, msg.MOD_DATA_OBJ_META_AN, request, &msg.EmptyResponse{})
}

// RegisterReplica registers a replica of a data object.
// This is an administrative call, a connection using a rodsadmin is required.
func (api *API) RegisterReplica(ctx context.Context, path, resource, physicalPath string) error {
	if !api.Admin {
		return ErrRequiresAdmin
	}

	request := &msg.DataObjectRequest{
		Path: path,
	}

	request.KeyVals.Add(msg.DATA_TYPE_KW, "generic")
	request.KeyVals.Add(msg.FILE_PATH_KW, physicalPath)
	request.KeyVals.Add(msg.DEST_RESC_NAME_KW, resource)
	request.KeyVals.Add(msg.REG_REPL_KW, "")

	return api.Request(ctx, msg.PHY_PATH_REG_AN, request, &msg.EmptyResponse{})
}

// CreateUser creates a user with the given type
// If a zone needs to be specified, use the username#zone format.
// This is an administrative call, a connection using a rodsadmin is required.
func (api *API) CreateUser(ctx context.Context, username, userType string) error {
	if !api.Admin {
		return ErrRequiresAdmin
	}

	request := &msg.AdminRequest{
		Arg0: "add",
		Arg1: "user",
		Arg2: username,
		Arg3: userType,
	}

	return api.Request(ctx, msg.GENERAL_ADMIN_AN, request, &msg.EmptyResponse{})
}

// CreateGroup creates a group.
// If a zone needs to be specified, use the groupname#zone format.
// This is an administrative call, a connection using a rodsadmin is required.
func (api *API) CreateGroup(ctx context.Context, groupname string) error {
	if !api.Admin {
		return ErrRequiresAdmin
	}

	request := &msg.AdminRequest{
		Arg0: "add",
		Arg1: "group",
		Arg2: groupname,
	}

	return api.Request(ctx, msg.GENERAL_ADMIN_AN, request, &msg.EmptyResponse{})
}

// ChangeUserPassword changes the password of a user object
// If a zone needs to be specified, use the username#zone format.
// This is an administrative call, a connection using a rodsadmin is required.
func (api *API) ChangeUserPassword(ctx context.Context, username, password string) error {
	if !api.Admin {
		return ErrRequiresAdmin
	}

	conn, err := api.Connect(ctx)
	if err != nil {
		return err
	}

	defer conn.Close()

	scrambledPassword := scramble.ObfuscateNewPassword(password, conn.NativePassword(), conn.ClientSignature())

	request := &msg.AdminRequest{
		Arg0: "modify",
		Arg1: "user",
		Arg2: username,
		Arg3: "password",
		Arg4: scrambledPassword,
	}

	return conn.Request(ctx, msg.GENERAL_ADMIN_AN, request, &msg.EmptyResponse{})
}

// ChangeUserType changes the type of a user
// If a zone needs to be specified, use the username#zone format.
// This is an administrative call, a connection using a rodsadmin is required.
func (api *API) ChangeUserType(ctx context.Context, username, userType string) error {
	if !api.Admin {
		return ErrRequiresAdmin
	}

	request := &msg.AdminRequest{
		Arg0: "modify",
		Arg1: "user",
		Arg2: username,
		Arg3: "type",
		Arg4: userType,
	}

	return api.Request(ctx, msg.GENERAL_ADMIN_AN, request, &msg.EmptyResponse{})
}

// RemoveUser removes a user or a group.
// If a zone needs to be specified, use the username#zone format.
// This is an administrative call, a connection using a rodsadmin is required.
func (api *API) RemoveUser(ctx context.Context, username string) error {
	if !api.Admin {
		return ErrRequiresAdmin
	}

	request := &msg.AdminRequest{
		Arg0: "rm",
		Arg1: "user",
		Arg2: username,
	}

	return api.Request(ctx, msg.GENERAL_ADMIN_AN, request, &msg.EmptyResponse{})
}

// RemoveGroup removes a group.
// If a zone needs to be specified, use the groupname#zone format.
// This is an administrative call, a connection using a rodsadmin is required.
func (api *API) RemoveGroup(ctx context.Context, groupname string) error {
	if !api.Admin {
		return ErrRequiresAdmin
	}

	request := &msg.AdminRequest{
		Arg0: "rm",
		Arg1: "group",
		Arg2: groupname,
	}

	return api.Request(ctx, msg.GENERAL_ADMIN_AN, request, &msg.EmptyResponse{})
}

// AddGroupMember adds a user to a group.
// If a zone needs to be specified, use the username#zone format.
// This is an administrative call, a connection using a rodsadmin is required.
func (api *API) AddGroupMember(ctx context.Context, groupname, username string) error {
	if !api.Admin {
		return ErrRequiresAdmin
	}

	request := &msg.AdminRequest{
		Arg0: "modify",
		Arg1: "group",
		Arg2: groupname,
		Arg3: "add",
		Arg4: username,
	}

	return api.Request(ctx, msg.GENERAL_ADMIN_AN, request, &msg.EmptyResponse{})
}

// RemoveGroupMember removes a user from a group.
// If a zone needs to be specified, use the username#zone format.
// This is an administrative call, a connection using a rodsadmin is required.
func (api *API) RemoveGroupMember(ctx context.Context, groupname, username string) error {
	if !api.Admin {
		return ErrRequiresAdmin
	}

	request := &msg.AdminRequest{
		Arg0: "modify",
		Arg1: "group",
		Arg2: groupname,
		Arg3: "remove",
		Arg4: username,
	}

	return api.Request(ctx, msg.GENERAL_ADMIN_AN, request, &msg.EmptyResponse{})
}

// SetUserQuota sets quota for a given user and resource ('total' for global)
// If a zone needs to be specified, use the username#zone format.
// This is an administrative call, a connection using a rodsadmin is required.
func (api *API) SetUserQuota(ctx context.Context, username, resource, value string) error {
	if !api.Admin {
		return ErrRequiresAdmin
	}

	request := &msg.AdminRequest{
		Arg0: "set-quota",
		Arg1: "user",
		Arg2: username,
		Arg3: resource,
		Arg4: value,
	}

	return api.Request(ctx, msg.GENERAL_ADMIN_AN, request, &msg.EmptyResponse{})
}

// SetGroupQuota sets quota for a given group and resource ('total' for global)
// If a zone needs to be specified, use the groupname#zone format.
// This is an administrative call, a connection using a rodsadmin is required.
func (api *API) SetGroupQuota(ctx context.Context, groupname, resource, value string) error {
	if !api.Admin {
		return ErrRequiresAdmin
	}

	request := &msg.AdminRequest{
		Arg0: "set-quota",
		Arg1: "group",
		Arg2: groupname,
		Arg3: resource,
		Arg4: value,
	}

	return api.Request(ctx, msg.GENERAL_ADMIN_AN, request, &msg.EmptyResponse{})
}

// ExecuteExternalRule executes an irods external rule.
// The rule should be the string representation of the rule, without the
// "@external rule {" and "}" wrappers. The parameters should be a map of
// parameter names to values.
// The optional instance parameter specifies the iRODS instance to run the
// rule on.
// This is an administrative call, a connection using a rodsadmin is required.
func (api *API) ExecuteExternalRule(ctx context.Context, rule string, params map[string]string, instance string) (map[string]string, error) {
	if !api.Admin {
		return nil, ErrRequiresAdmin
	}

	request := msg.ExecRuleRequest{
		Rule:     fmt.Sprintf("@external rule { %s }", rule),
		OutParam: "ruleExecOut",
		Params: msg.MsParamArray{
			Length: len(params),
		},
	}

	for label, value := range params {
		request.Params.Values = append(request.Params.Values, msg.MsParam{
			Label: label,
			Type:  "STR_PI",
			InOut: value,
		})
	}

	if instance != "" {
		request.KeyVals.Add("instance_name", instance)
	}

	var response msg.MsParamArray

	if err := api.Request(ctx, msg.EXEC_MY_RULE_AN, request, &response); err != nil {
		return nil, err
	}

	result := map[string]string{}

	for _, param := range response.Values {
		if param.Type != "STR_PI" {
			continue
		}

		result[param.Label] = param.InOut
	}

	return result, nil
}
