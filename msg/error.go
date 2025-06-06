package msg

import (
	"fmt"
	"os"
	"syscall"
)

type IRODSError struct {
	Code    ErrorCode
	Message string
}

func (e *IRODSError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("IRODS error %s", e.Name())
	}

	return fmt.Sprintf("IRODS error %s: %s", e.Name(), e.Message)
}

func (e *IRODSError) Name() string {
	if name, ok := ErrorCodes[e.Code]; ok {
		return name
	}

	// Try to round code up to a multiple of 1000
	code := (e.Code / 1000) * 1000

	if name, ok := ErrorCodes[code]; ok {
		return name
	}

	return fmt.Sprintf("%d", e.Code)
}

// Unwrap tries to unwrap to an equivalent os/syscall error,
// e.g. os.ErrNotExist or syscall.NOENT. This is useful
// when using e.g. errors.Is(err, os.ErrNotExist).
func (e *IRODSError) Unwrap() error {
	if native, ok := NativeErrors[e.Code]; ok {
		return native
	}

	// Try to round code up to a multiple of 1000
	code := (e.Code / 1000) * 1000

	if native, ok := NativeErrors[code]; ok {
		return native
	}

	// Did not find a native error
	return nil
}

type ErrorCode int32

// error codes
const (
	SYS_SOCK_OPEN_ERR                      ErrorCode = -1000
	SYS_SOCK_LISTEN_ERR                    ErrorCode = -1100
	SYS_SOCK_BIND_ERR                      ErrorCode = -2000
	SYS_SOCK_ACCEPT_ERR                    ErrorCode = -3000
	SYS_HEADER_READ_LEN_ERR                ErrorCode = -4000
	SYS_HEADER_WRITE_LEN_ERR               ErrorCode = -5000
	SYS_HEADER_TPYE_LEN_ERR                ErrorCode = -6000
	SYS_CAUGHT_SIGNAL                      ErrorCode = -7000
	SYS_GETSTARTUP_PACK_ERR                ErrorCode = -8000
	SYS_EXCEED_CONNECT_CNT                 ErrorCode = -9000
	SYS_USER_NOT_ALLOWED_TO_CONN           ErrorCode = -10000
	SYS_READ_MSG_BODY_INPUT_ERR            ErrorCode = -11000
	SYS_UNMATCHED_API_NUM                  ErrorCode = -12000
	SYS_NO_API_PRIV                        ErrorCode = -13000
	SYS_API_INPUT_ERR                      ErrorCode = -14000
	SYS_PACK_INSTRUCT_FORMAT_ERR           ErrorCode = -15000
	SYS_MALLOC_ERR                         ErrorCode = -16000
	SYS_GET_HOSTNAME_ERR                   ErrorCode = -17000
	SYS_OUT_OF_FILE_DESC                   ErrorCode = -18000
	SYS_FILE_DESC_OUT_OF_RANGE             ErrorCode = -19000
	SYS_UNRECOGNIZED_REMOTE_FLAG           ErrorCode = -20000
	SYS_INVALID_SERVER_HOST                ErrorCode = -21000
	SYS_SVR_TO_SVR_CONNECT_FAILED          ErrorCode = -22000
	SYS_BAD_FILE_DESCRIPTOR                ErrorCode = -23000
	SYS_INTERNAL_NULL_INPUT_ERR            ErrorCode = -24000
	SYS_CONFIG_FILE_ERR                    ErrorCode = -25000
	SYS_INVALID_ZONE_NAME                  ErrorCode = -26000
	SYS_COPY_LEN_ERR                       ErrorCode = -27000
	SYS_PORT_COOKIE_ERR                    ErrorCode = -28000
	SYS_KEY_VAL_TABLE_ERR                  ErrorCode = -29000
	SYS_INVALID_RESC_TYPE                  ErrorCode = -30000
	SYS_INVALID_FILE_PATH                  ErrorCode = -31000
	SYS_INVALID_RESC_INPUT                 ErrorCode = -32000
	SYS_INVALID_PORTAL_OPR                 ErrorCode = -33000
	SYS_PARA_OPR_NO_SUPPORT                ErrorCode = -34000
	SYS_INVALID_OPR_TYPE                   ErrorCode = -35000
	SYS_NO_PATH_PERMISSION                 ErrorCode = -36000
	SYS_NO_ICAT_SERVER_ERR                 ErrorCode = -37000
	SYS_AGENT_INIT_ERR                     ErrorCode = -38000
	SYS_PROXYUSER_NO_PRIV                  ErrorCode = -39000
	SYS_NO_DATA_OBJ_PERMISSION             ErrorCode = -40000
	SYS_DELETE_DISALLOWED                  ErrorCode = -41000
	SYS_OPEN_REI_FILE_ERR                  ErrorCode = -42000
	SYS_NO_RCAT_SERVER_ERR                 ErrorCode = -43000
	SYS_UNMATCH_PACK_INSTRUCTI_NAME        ErrorCode = -44000
	SYS_SVR_TO_CLI_MSI_NO_EXIST            ErrorCode = -45000
	SYS_COPY_ALREADY_IN_RESC               ErrorCode = -46000
	SYS_RECONN_OPR_MISMATCH                ErrorCode = -47000
	SYS_INPUT_PERM_OUT_OF_RANGE            ErrorCode = -48000
	SYS_FORK_ERROR                         ErrorCode = -49000
	SYS_PIPE_ERROR                         ErrorCode = -50000
	SYS_EXEC_CMD_STATUS_SZ_ERROR           ErrorCode = -51000
	SYS_PATH_IS_NOT_A_FILE                 ErrorCode = -52000
	SYS_UNMATCHED_SPEC_COLL_TYPE           ErrorCode = -53000
	SYS_TOO_MANY_QUERY_RESULT              ErrorCode = -54000
	SYS_SPEC_COLL_NOT_IN_CACHE             ErrorCode = -55000
	SYS_SPEC_COLL_OBJ_NOT_EXIST            ErrorCode = -56000
	SYS_REG_OBJ_IN_SPEC_COLL               ErrorCode = -57000
	SYS_DEST_SPEC_COLL_SUB_EXIST           ErrorCode = -58000
	SYS_SRC_DEST_SPEC_COLL_CONFLICT        ErrorCode = -59000
	SYS_UNKNOWN_SPEC_COLL_CLASS            ErrorCode = -60000
	SYS_DUPLICATE_XMSG_TICKET              ErrorCode = -61000
	SYS_UNMATCHED_XMSG_TICKET              ErrorCode = -62000
	SYS_NO_XMSG_FOR_MSG_NUMBER             ErrorCode = -63000
	SYS_COLLINFO_2_FORMAT_ERR              ErrorCode = -64000
	SYS_CACHE_STRUCT_FILE_RESC_ERR         ErrorCode = -65000
	SYS_NOT_SUPPORTED                      ErrorCode = -66000
	SYS_TAR_STRUCT_FILE_EXTRACT_ERR        ErrorCode = -67000
	SYS_STRUCT_FILE_DESC_ERR               ErrorCode = -68000
	SYS_TAR_OPEN_ERR                       ErrorCode = -69000
	SYS_TAR_EXTRACT_ALL_ERR                ErrorCode = -70000
	SYS_TAR_CLOSE_ERR                      ErrorCode = -71000
	SYS_STRUCT_FILE_PATH_ERR               ErrorCode = -72000
	SYS_MOUNT_MOUNTED_COLL_ERR             ErrorCode = -73000
	SYS_COLL_NOT_MOUNTED_ERR               ErrorCode = -74000
	SYS_STRUCT_FILE_BUSY_ERR               ErrorCode = -75000
	SYS_STRUCT_FILE_INMOUNTED_COLL         ErrorCode = -76000
	SYS_COPY_NOT_EXIST_IN_RESC             ErrorCode = -77000
	SYS_RESC_DOES_NOT_EXIST                ErrorCode = -78000
	SYS_COLLECTION_NOT_EMPTY               ErrorCode = -79000
	SYS_OBJ_TYPE_NOT_STRUCT_FILE           ErrorCode = -80000
	SYS_WRONG_RESC_POLICY_FOR_BUN_OPR      ErrorCode = -81000
	SYS_DIR_IN_VAULT_NOT_EMPTY             ErrorCode = -82000
	SYS_OPR_FLAG_NOT_SUPPORT               ErrorCode = -83000
	SYS_TAR_APPEND_ERR                     ErrorCode = -84000
	SYS_INVALID_PROTOCOL_TYPE              ErrorCode = -85000
	SYS_UDP_CONNECT_ERR                    ErrorCode = -86000
	SYS_UDP_TRANSFER_ERR                   ErrorCode = -89000
	SYS_UDP_NO_SUPPORT_ERR                 ErrorCode = -90000
	SYS_READ_MSG_BODY_LEN_ERR              ErrorCode = -91000
	CROSS_ZONE_SOCK_CONNECT_ERR            ErrorCode = -92000
	SYS_NO_FREE_RE_THREAD                  ErrorCode = -93000
	SYS_BAD_RE_THREAD_INX                  ErrorCode = -94000
	SYS_CANT_DIRECTLY_ACC_COMPOUND_RESC    ErrorCode = -95000
	SYS_SRC_DEST_RESC_COMPOUND_TYPE        ErrorCode = -96000
	SYS_CACHE_RESC_NOT_ON_SAME_HOST        ErrorCode = -97000
	SYS_NO_CACHE_RESC_IN_GRP               ErrorCode = -98000
	SYS_UNMATCHED_RESC_IN_RESC_GRP         ErrorCode = -99000
	SYS_CANT_MV_BUNDLE_DATA_TO_TRASH       ErrorCode = -100000
	SYS_CANT_MV_BUNDLE_DATA_BY_COPY        ErrorCode = -101000
	SYS_EXEC_TAR_ERR                       ErrorCode = -102000
	SYS_CANT_CHKSUM_COMP_RESC_DATA         ErrorCode = -103000
	SYS_CANT_CHKSUM_BUNDLED_DATA           ErrorCode = -104000
	SYS_RESC_IS_DOWN                       ErrorCode = -105000
	SYS_UPDATE_REPL_INFO_ERR               ErrorCode = -106000
	SYS_COLL_LINK_PATH_ERR                 ErrorCode = -107000
	SYS_LINK_CNT_EXCEEDED_ERR              ErrorCode = -108000
	SYS_CROSS_ZONE_MV_NOT_SUPPORTED        ErrorCode = -109000
	SYS_RESC_QUOTA_EXCEEDED                ErrorCode = -110000
	SYS_RENAME_STRUCT_COUNT_EXCEEDED       ErrorCode = -111000
	SYS_BULK_REG_COUNT_EXCEEDED            ErrorCode = -112000
	SYS_REQUESTED_BUF_TOO_LARGE            ErrorCode = -113000
	SYS_INVALID_RESC_FOR_BULK_OPR          ErrorCode = -114000
	SYS_SOCK_READ_TIMEDOUT                 ErrorCode = -115000
	SYS_SOCK_READ_ERR                      ErrorCode = -116000
	SYS_CONNECT_CONTROL_CONFIG_ERR         ErrorCode = -117000
	SYS_MAX_CONNECT_COUNT_EXCEEDED         ErrorCode = -118000
	SYS_STRUCT_ELEMENT_MISMATCH            ErrorCode = -119000
	SYS_PHY_PATH_INUSE                     ErrorCode = -120000
	SYS_USER_NO_PERMISSION                 ErrorCode = -121000
	SYS_USER_RETRIEVE_ERR                  ErrorCode = -122000
	SYS_FS_LOCK_ERR                        ErrorCode = -123000
	SYS_LOCK_TYPE_INP_ERR                  ErrorCode = -124000
	SYS_LOCK_CMD_INP_ERR                   ErrorCode = -125000
	SYS_ZIP_FORMAT_NOT_SUPPORTED           ErrorCode = -126000
	SYS_ADD_TO_ARCH_OPR_NOT_SUPPORTED      ErrorCode = -127000
	CANT_REG_IN_VAULT_FILE                 ErrorCode = -128000
	PATH_REG_NOT_ALLOWED                   ErrorCode = -129000
	SYS_INVALID_INPUT_PARAM                ErrorCode = -130000
	SYS_GROUP_RETRIEVE_ERR                 ErrorCode = -131000
	SYS_MSSO_APPEND_ERR                    ErrorCode = -132000
	SYS_MSSO_STRUCT_FILE_EXTRACT_ERR       ErrorCode = -133000
	SYS_MSSO_EXTRACT_ALL_ERR               ErrorCode = -134000
	SYS_MSSO_OPEN_ERR                      ErrorCode = -135000
	SYS_MSSO_CLOSE_ERR                     ErrorCode = -136000
	SYS_RULE_NOT_FOUND                     ErrorCode = -144000
	SYS_NOT_IMPLEMENTED                    ErrorCode = -146000
	SYS_SIGNED_SID_NOT_MATCHED             ErrorCode = -147000
	SYS_HASH_IMMUTABLE                     ErrorCode = -148000
	SYS_UNINITIALIZED                      ErrorCode = -149000
	SYS_NEGATIVE_SIZE                      ErrorCode = -150000
	SYS_ALREADY_INITIALIZED                ErrorCode = -151000
	SYS_SETENV_ERR                         ErrorCode = -152000
	SYS_GETENV_ERR                         ErrorCode = -153000
	SYS_INTERNAL_ERR                       ErrorCode = -154000
	SYS_SOCK_SELECT_ERR                    ErrorCode = -155000
	SYS_THREAD_ENCOUNTERED_INTERRUPT       ErrorCode = -156000
	SYS_THREAD_RESOURCE_ERR                ErrorCode = -157000
	SYS_BAD_INPUT                          ErrorCode = -158000
	SYS_PORT_RANGE_EXHAUSTED               ErrorCode = -159000
	SYS_SERVICE_ROLE_NOT_SUPPORTED         ErrorCode = -160000
	SYS_SOCK_WRITE_ERR                     ErrorCode = -161000
	SYS_SOCK_CONNECT_ERR                   ErrorCode = -162000
	SYS_OPERATION_IN_PROGRESS              ErrorCode = -163000
	SYS_REPLICA_DOES_NOT_EXIST             ErrorCode = -164000
	SYS_UNKNOWN_ERROR                      ErrorCode = -165000
	SYS_NO_GOOD_REPLICA                    ErrorCode = -166000
	SYS_LIBRARY_ERROR                      ErrorCode = -167000
	SYS_REPLICA_INACCESSIBLE               ErrorCode = -168000
	SYS_NOT_ALLOWED                        ErrorCode = -169000
	NOT_A_COLLECTION                       ErrorCode = -170000
	NOT_A_DATA_OBJECT                      ErrorCode = -171000
	JSON_VALIDATION_ERROR                  ErrorCode = -172000
	USER_AUTH_SCHEME_ERR                   ErrorCode = -300000
	USER_AUTH_STRING_EMPTY                 ErrorCode = -301000
	USER_RODS_HOST_EMPTY                   ErrorCode = -302000
	USER_RODS_HOSTNAME_ERR                 ErrorCode = -303000
	USER_SOCK_OPEN_ERR                     ErrorCode = -304000
	USER_SOCK_CONNECT_ERR                  ErrorCode = -305000
	USER_STRLEN_TOOLONG                    ErrorCode = -306000
	USER_API_INPUT_ERR                     ErrorCode = -307000
	USER_PACKSTRUCT_INPUT_ERR              ErrorCode = -308000
	USER_NO_SUPPORT_ERR                    ErrorCode = -309000
	USER_FILE_DOES_NOT_EXIST               ErrorCode = -310000
	USER_FILE_TOO_LARGE                    ErrorCode = -311000
	OVERWRITE_WITHOUT_FORCE_FLAG           ErrorCode = -312000
	UNMATCHED_KEY_OR_INDEX                 ErrorCode = -313000
	USER_CHKSUM_MISMATCH                   ErrorCode = -314000
	USER_BAD_KEYWORD_ERR                   ErrorCode = -315000
	USER__NULL_INPUT_ERR                   ErrorCode = -316000
	USER_INPUT_PATH_ERR                    ErrorCode = -317000
	USER_INPUT_OPTION_ERR                  ErrorCode = -318000
	USER_INVALID_USERNAME_FORMAT           ErrorCode = -319000
	USER_DIRECT_RESC_INPUT_ERR             ErrorCode = -320000
	USER_NO_RESC_INPUT_ERR                 ErrorCode = -321000
	USER_PARAM_LABEL_ERR                   ErrorCode = -322000
	USER_PARAM_TYPE_ERR                    ErrorCode = -323000
	BASE64_BUFFER_OVERFLOW                 ErrorCode = -324000
	BASE64_INVALID_PACKET                  ErrorCode = -325000
	USER_MSG_TYPE_NO_SUPPORT               ErrorCode = -326000
	USER_RSYNC_NO_MODE_INPUT_ERR           ErrorCode = -337000
	USER_OPTION_INPUT_ERR                  ErrorCode = -338000
	SAME_SRC_DEST_PATHS_ERR                ErrorCode = -339000
	USER_RESTART_FILE_INPUT_ERR            ErrorCode = -340000
	RESTART_OPR_FAILED                     ErrorCode = -341000
	BAD_EXEC_CMD_PATH                      ErrorCode = -342000
	EXEC_CMD_OUTPUT_TOO_LARGE              ErrorCode = -343000
	EXEC_CMD_ERROR                         ErrorCode = -344000
	BAD_INPUT_DESC_INDEX                   ErrorCode = -345000
	USER_PATH_EXCEEDS_MAX                  ErrorCode = -346000
	USER_SOCK_CONNECT_TIMEDOUT             ErrorCode = -347000
	USER_API_VERSION_MISMATCH              ErrorCode = -348000
	USER_INPUT_FORMAT_ERR                  ErrorCode = -349000
	USER_ACCESS_DENIED                     ErrorCode = -350000
	CANT_RM_MV_BUNDLE_TYPE                 ErrorCode = -351000
	NO_MORE_RESULT                         ErrorCode = -352000
	NO_KEY_WD_IN_MS_INP_STR                ErrorCode = -353000
	CANT_RM_NON_EMPTY_HOME_COLL            ErrorCode = -354000
	CANT_UNREG_IN_VAULT_FILE               ErrorCode = -355000
	NO_LOCAL_FILE_RSYNC_IN_MSI             ErrorCode = -356000
	BULK_OPR_MISMATCH_FOR_RESTART          ErrorCode = -357000
	OBJ_PATH_DOES_NOT_EXIST                ErrorCode = -358000
	SYMLINKED_BUNFILE_NOT_ALLOWED          ErrorCode = -359000
	USER_INPUT_STRING_ERR                  ErrorCode = -360000
	USER_INVALID_RESC_INPUT                ErrorCode = -361000
	USER_NOT_ALLOWED_TO_EXEC_CMD           ErrorCode = -370000
	USER_HASH_TYPE_MISMATCH                ErrorCode = -380000
	USER_INVALID_CLIENT_ENVIRONMENT        ErrorCode = -390000
	USER_INSUFFICIENT_FREE_INODES          ErrorCode = -400000
	USER_FILE_SIZE_MISMATCH                ErrorCode = -401000
	USER_INCOMPATIBLE_PARAMS               ErrorCode = -402000
	USER_INVALID_REPLICA_INPUT             ErrorCode = -403000
	USER_INCOMPATIBLE_OPEN_FLAGS           ErrorCode = -404000
	INTERMEDIATE_REPLICA_ACCESS            ErrorCode = -405000
	LOCKED_DATA_OBJECT_ACCESS              ErrorCode = -406000
	CHECK_VERIFICATION_RESULTS             ErrorCode = -407000
	FILE_INDEX_LOOKUP_ERR                  ErrorCode = -500000
	UNIX_FILE_OPEN_ERR                     ErrorCode = -510000
	UNIX_FILE_CREATE_ERR                   ErrorCode = -511000
	UNIX_FILE_READ_ERR                     ErrorCode = -512000
	UNIX_FILE_WRITE_ERR                    ErrorCode = -513000
	UNIX_FILE_CLOSE_ERR                    ErrorCode = -514000
	UNIX_FILE_UNLINK_ERR                   ErrorCode = -515000
	UNIX_FILE_STAT_ERR                     ErrorCode = -516000
	UNIX_FILE_FSTAT_ERR                    ErrorCode = -517000
	UNIX_FILE_LSEEK_ERR                    ErrorCode = -518000
	UNIX_FILE_FSYNC_ERR                    ErrorCode = -519000
	UNIX_FILE_MKDIR_ERR                    ErrorCode = -520000
	UNIX_FILE_RMDIR_ERR                    ErrorCode = -521000
	UNIX_FILE_OPENDIR_ERR                  ErrorCode = -522000
	UNIX_FILE_CLOSEDIR_ERR                 ErrorCode = -523000
	UNIX_FILE_READDIR_ERR                  ErrorCode = -524000
	UNIX_FILE_STAGE_ERR                    ErrorCode = -525000
	UNIX_FILE_GET_FS_FREESPACE_ERR         ErrorCode = -526000
	UNIX_FILE_CHMOD_ERR                    ErrorCode = -527000
	UNIX_FILE_RENAME_ERR                   ErrorCode = -528000
	UNIX_FILE_TRUNCATE_ERR                 ErrorCode = -529000
	UNIX_FILE_LINK_ERR                     ErrorCode = -530000
	UNIX_FILE_OPR_TIMEOUT_ERR              ErrorCode = -540000
	UNIV_MSS_SYNCTOARCH_ERR                ErrorCode = -550000
	UNIV_MSS_STAGETOCACHE_ERR              ErrorCode = -551000
	UNIV_MSS_UNLINK_ERR                    ErrorCode = -552000
	UNIV_MSS_MKDIR_ERR                     ErrorCode = -553000
	UNIV_MSS_CHMOD_ERR                     ErrorCode = -554000
	UNIV_MSS_STAT_ERR                      ErrorCode = -555000
	UNIV_MSS_RENAME_ERR                    ErrorCode = -556000
	HPSS_AUTH_NOT_SUPPORTED                ErrorCode = -600000
	HPSS_FILE_OPEN_ERR                     ErrorCode = -610000
	HPSS_FILE_CREATE_ERR                   ErrorCode = -611000
	HPSS_FILE_READ_ERR                     ErrorCode = -612000
	HPSS_FILE_WRITE_ERR                    ErrorCode = -613000
	HPSS_FILE_CLOSE_ERR                    ErrorCode = -614000
	HPSS_FILE_UNLINK_ERR                   ErrorCode = -615000
	HPSS_FILE_STAT_ERR                     ErrorCode = -616000
	HPSS_FILE_FSTAT_ERR                    ErrorCode = -617000
	HPSS_FILE_LSEEK_ERR                    ErrorCode = -618000
	HPSS_FILE_FSYNC_ERR                    ErrorCode = -619000
	HPSS_FILE_MKDIR_ERR                    ErrorCode = -620000
	HPSS_FILE_RMDIR_ERR                    ErrorCode = -621000
	HPSS_FILE_OPENDIR_ERR                  ErrorCode = -622000
	HPSS_FILE_CLOSEDIR_ERR                 ErrorCode = -623000
	HPSS_FILE_READDIR_ERR                  ErrorCode = -624000
	HPSS_FILE_STAGE_ERR                    ErrorCode = -625000
	HPSS_FILE_GET_FS_FREESPACE_ERR         ErrorCode = -626000
	HPSS_FILE_CHMOD_ERR                    ErrorCode = -627000
	HPSS_FILE_RENAME_ERR                   ErrorCode = -628000
	HPSS_FILE_TRUNCATE_ERR                 ErrorCode = -629000
	HPSS_FILE_LINK_ERR                     ErrorCode = -630000
	HPSS_AUTH_ERR                          ErrorCode = -631000
	HPSS_WRITE_LIST_ERR                    ErrorCode = -632000
	HPSS_READ_LIST_ERR                     ErrorCode = -633000
	HPSS_TRANSFER_ERR                      ErrorCode = -634000
	HPSS_MOVER_PROT_ERR                    ErrorCode = -635000
	S3_INIT_ERROR                          ErrorCode = -701000
	S3_PUT_ERROR                           ErrorCode = -702000
	S3_GET_ERROR                           ErrorCode = -703000
	S3_FILE_UNLINK_ERR                     ErrorCode = -715000
	S3_FILE_STAT_ERR                       ErrorCode = -716000
	S3_FILE_COPY_ERR                       ErrorCode = -717000
	S3_FILE_OPEN_ERR                       ErrorCode = -718000
	S3_FILE_SEEK_ERR                       ErrorCode = -719000
	S3_FILE_RENAME_ERR                     ErrorCode = -720000
	REPLICA_IS_BEING_STAGED                ErrorCode = -721000
	REPLICA_STAGING_FAILED                 ErrorCode = -722000
	WOS_PUT_ERR                            ErrorCode = -750000
	WOS_STREAM_PUT_ERR                     ErrorCode = -751000
	WOS_STREAM_CLOSE_ERR                   ErrorCode = -752000
	WOS_GET_ERR                            ErrorCode = -753000
	WOS_STREAM_GET_ERR                     ErrorCode = -754000
	WOS_UNLINK_ERR                         ErrorCode = -755000
	WOS_STAT_ERR                           ErrorCode = -756000
	WOS_CONNECT_ERR                        ErrorCode = -757000
	HDFS_FILE_OPEN_ERR                     ErrorCode = -730000
	HDFS_FILE_CREATE_ERR                   ErrorCode = -731000
	HDFS_FILE_READ_ERR                     ErrorCode = -732000
	HDFS_FILE_WRITE_ERR                    ErrorCode = -733000
	HDFS_FILE_CLOSE_ERR                    ErrorCode = -734000
	HDFS_FILE_UNLINK_ERR                   ErrorCode = -735000
	HDFS_FILE_STAT_ERR                     ErrorCode = -736000
	HDFS_FILE_FSTAT_ERR                    ErrorCode = -737000
	HDFS_FILE_LSEEK_ERR                    ErrorCode = -738000
	HDFS_FILE_FSYNC_ERR                    ErrorCode = -739000
	HDFS_FILE_MKDIR_ERR                    ErrorCode = -741000
	HDFS_FILE_RMDIR_ERR                    ErrorCode = -742000
	HDFS_FILE_OPENDIR_ERR                  ErrorCode = -743000
	HDFS_FILE_CLOSEDIR_ERR                 ErrorCode = -744000
	HDFS_FILE_READDIR_ERR                  ErrorCode = -745000
	HDFS_FILE_STAGE_ERR                    ErrorCode = -746000
	HDFS_FILE_GET_FS_FREESPACE_ERR         ErrorCode = -747000
	HDFS_FILE_CHMOD_ERR                    ErrorCode = -748000
	HDFS_FILE_RENAME_ERR                   ErrorCode = -749000
	HDFS_FILE_TRUNCATE_ERR                 ErrorCode = -760000
	HDFS_FILE_LINK_ERR                     ErrorCode = -761000
	HDFS_FILE_OPR_TIMEOUT_ERR              ErrorCode = -762000
	DIRECT_ACCESS_FILE_USER_INVALID_ERR    ErrorCode = -770000
	CATALOG_NOT_CONNECTED                  ErrorCode = -801000
	CAT_ENV_ERR                            ErrorCode = -802000
	CAT_CONNECT_ERR                        ErrorCode = -803000
	CAT_DISCONNECT_ERR                     ErrorCode = -804000
	CAT_CLOSE_ENV_ERR                      ErrorCode = -805000
	CAT_SQL_ERR                            ErrorCode = -806000
	CAT_GET_ROW_ERR                        ErrorCode = -807000
	CAT_NO_ROWS_FOUND                      ErrorCode = -808000
	CATALOG_ALREADY_HAS_ITEM_BY_THAT_NAME  ErrorCode = -809000
	CAT_INVALID_RESOURCE_TYPE              ErrorCode = -810000
	CAT_INVALID_RESOURCE_CLASS             ErrorCode = -811000
	CAT_INVALID_RESOURCE_NET_ADDR          ErrorCode = -812000
	CAT_INVALID_RESOURCE_VAULT_PATH        ErrorCode = -813000
	CAT_UNKNOWN_COLLECTION                 ErrorCode = -814000
	CAT_INVALID_DATA_TYPE                  ErrorCode = -815000
	CAT_INVALID_ARGUMENT                   ErrorCode = -816000
	CAT_UNKNOWN_FILE                       ErrorCode = -817000
	CAT_NO_ACCESS_PERMISSION               ErrorCode = -818000
	CAT_SUCCESS_BUT_WITH_NO_INFO           ErrorCode = -819000
	CAT_INVALID_USER_TYPE                  ErrorCode = -820000
	CAT_COLLECTION_NOT_EMPTY               ErrorCode = -821000
	CAT_TOO_MANY_TABLES                    ErrorCode = -822000
	CAT_UNKNOWN_TABLE                      ErrorCode = -823000
	CAT_NOT_OPEN                           ErrorCode = -824000
	CAT_FAILED_TO_LINK_TABLES              ErrorCode = -825000
	CAT_INVALID_AUTHENTICATION             ErrorCode = -826000
	CAT_INVALID_USER                       ErrorCode = -827000
	CAT_INVALID_ZONE                       ErrorCode = -828000
	CAT_INVALID_GROUP                      ErrorCode = -829000
	CAT_INSUFFICIENT_PRIVILEGE_LEVEL       ErrorCode = -830000
	CAT_INVALID_RESOURCE                   ErrorCode = -831000
	CAT_INVALID_CLIENT_USER                ErrorCode = -832000
	CAT_NAME_EXISTS_AS_COLLECTION          ErrorCode = -833000
	CAT_NAME_EXISTS_AS_DATAOBJ             ErrorCode = -834000
	CAT_RESOURCE_NOT_EMPTY                 ErrorCode = -835000
	CAT_NOT_A_DATAOBJ_AND_NOT_A_COLLECTION ErrorCode = -836000
	CAT_RECURSIVE_MOVE                     ErrorCode = -837000
	CAT_LAST_REPLICA                       ErrorCode = -838000
	CAT_OCI_ERROR                          ErrorCode = -839000
	CAT_PASSWORD_EXPIRED                   ErrorCode = -840000
	CAT_PASSWORD_ENCODING_ERROR            ErrorCode = -850000
	CAT_TABLE_ACCESS_DENIED                ErrorCode = -851000
	CAT_UNKNOWN_RESOURCE                   ErrorCode = -852000
	CAT_UNKNOWN_SPECIFIC_QUERY             ErrorCode = -853000
	CAT_PSEUDO_RESC_MODIFY_DISALLOWED      ErrorCode = -854000
	CAT_HOSTNAME_INVALID                   ErrorCode = -855000
	CAT_BIND_VARIABLE_LIMIT_EXCEEDED       ErrorCode = -856000
	CAT_INVALID_CHILD                      ErrorCode = -857000
	CAT_INVALID_OBJ_COUNT                  ErrorCode = -858000
	CAT_INVALID_RESOURCE_NAME              ErrorCode = -859000
	CAT_STATEMENT_TABLE_FULL               ErrorCode = -860000
	CAT_RESOURCE_NAME_LENGTH_EXCEEDED      ErrorCode = -861000
	CAT_NO_CHECKSUM_FOR_REPLICA            ErrorCode = -862000
	CAT_TICKET_INVALID                     ErrorCode = -890000
	CAT_TICKET_EXPIRED                     ErrorCode = -891000
	CAT_TICKET_USES_EXCEEDED               ErrorCode = -892000
	CAT_TICKET_USER_EXCLUDED               ErrorCode = -893000
	CAT_TICKET_HOST_EXCLUDED               ErrorCode = -894000
	CAT_TICKET_GROUP_EXCLUDED              ErrorCode = -895000
	CAT_TICKET_WRITE_USES_EXCEEDED         ErrorCode = -896000
	CAT_TICKET_WRITE_BYTES_EXCEEDED        ErrorCode = -897000
	FILE_OPEN_ERR                          ErrorCode = -900000
	FILE_READ_ERR                          ErrorCode = -901000
	FILE_WRITE_ERR                         ErrorCode = -902000
	PASSWORD_EXCEEDS_MAX_SIZE              ErrorCode = -903000
	ENVIRONMENT_VAR_HOME_NOT_DEFINED       ErrorCode = -904000
	UNABLE_TO_STAT_FILE                    ErrorCode = -905000
	AUTH_FILE_NOT_ENCRYPTED                ErrorCode = -906000
	AUTH_FILE_DOES_NOT_EXIST               ErrorCode = -907000
	UNLINK_FAILED                          ErrorCode = -908000
	NO_PASSWORD_ENTERED                    ErrorCode = -909000
	REMOTE_SERVER_AUTHENTICATION_FAILURE   ErrorCode = -910000
	REMOTE_SERVER_AUTH_NOT_PROVIDED        ErrorCode = -911000
	REMOTE_SERVER_AUTH_EMPTY               ErrorCode = -912000
	REMOTE_SERVER_SID_NOT_DEFINED          ErrorCode = -913000
	GSI_NOT_COMPILED_IN                    ErrorCode = -921000
	GSI_NOT_BUILT_INTO_CLIENT              ErrorCode = -922000
	GSI_NOT_BUILT_INTO_SERVER              ErrorCode = -923000
	GSI_ERROR_IMPORT_NAME                  ErrorCode = -924000
	GSI_ERROR_INIT_SECURITY_CONTEXT        ErrorCode = -925000
	GSI_ERROR_SENDING_TOKEN_LENGTH         ErrorCode = -926000
	GSI_ERROR_READING_TOKEN_LENGTH         ErrorCode = -927000
	GSI_ERROR_TOKEN_TOO_LARGE              ErrorCode = -928000
	GSI_ERROR_BAD_TOKEN_RCVED              ErrorCode = -929000
	GSI_SOCKET_READ_ERROR                  ErrorCode = -930000
	GSI_PARTIAL_TOKEN_READ                 ErrorCode = -931000
	GSI_SOCKET_WRITE_ERROR                 ErrorCode = -932000
	GSI_ERROR_FROM_GSI_LIBRARY             ErrorCode = -933000
	GSI_ERROR_IMPORTING_NAME               ErrorCode = -934000
	GSI_ERROR_ACQUIRING_CREDS              ErrorCode = -935000
	GSI_ACCEPT_SEC_CONTEXT_ERROR           ErrorCode = -936000
	GSI_ERROR_DISPLAYING_NAME              ErrorCode = -937000
	GSI_ERROR_RELEASING_NAME               ErrorCode = -938000
	GSI_DN_DOES_NOT_MATCH_USER             ErrorCode = -939000
	GSI_QUERY_INTERNAL_ERROR               ErrorCode = -940000
	GSI_NO_MATCHING_DN_FOUND               ErrorCode = -941000
	GSI_MULTIPLE_MATCHING_DN_FOUND         ErrorCode = -942000
	KRB_NOT_COMPILED_IN                    ErrorCode = -951000
	KRB_NOT_BUILT_INTO_CLIENT              ErrorCode = -952000
	KRB_NOT_BUILT_INTO_SERVER              ErrorCode = -953000
	KRB_ERROR_IMPORT_NAME                  ErrorCode = -954000
	KRB_ERROR_INIT_SECURITY_CONTEXT        ErrorCode = -955000
	KRB_ERROR_SENDING_TOKEN_LENGTH         ErrorCode = -956000
	KRB_ERROR_READING_TOKEN_LENGTH         ErrorCode = -957000
	KRB_ERROR_TOKEN_TOO_LARGE              ErrorCode = -958000
	KRB_ERROR_BAD_TOKEN_RCVED              ErrorCode = -959000
	KRB_SOCKET_READ_ERROR                  ErrorCode = -960000
	KRB_PARTIAL_TOKEN_READ                 ErrorCode = -961000
	KRB_SOCKET_WRITE_ERROR                 ErrorCode = -962000
	KRB_ERROR_FROM_KRB_LIBRARY             ErrorCode = -963000
	KRB_ERROR_IMPORTING_NAME               ErrorCode = -964000
	KRB_ERROR_ACQUIRING_CREDS              ErrorCode = -965000
	KRB_ACCEPT_SEC_CONTEXT_ERROR           ErrorCode = -966000
	KRB_ERROR_DISPLAYING_NAME              ErrorCode = -967000
	KRB_ERROR_RELEASING_NAME               ErrorCode = -968000
	KRB_USER_DN_NOT_FOUND                  ErrorCode = -969000
	KRB_NAME_MATCHES_MULTIPLE_USERS        ErrorCode = -970000
	KRB_QUERY_INTERNAL_ERROR               ErrorCode = -971000
	OSAUTH_NOT_BUILT_INTO_CLIENT           ErrorCode = -981000
	OSAUTH_NOT_BUILT_INTO_SERVER           ErrorCode = -982000
	PAM_AUTH_NOT_BUILT_INTO_CLIENT         ErrorCode = -991000
	PAM_AUTH_NOT_BUILT_INTO_SERVER         ErrorCode = -992000
	PAM_AUTH_PASSWORD_FAILED               ErrorCode = -993000
	PAM_AUTH_PASSWORD_INVALID_TTL          ErrorCode = -994000

	OBJPATH_EMPTY_IN_STRUCT_ERR              ErrorCode = -1000000
	RESCNAME_EMPTY_IN_STRUCT_ERR             ErrorCode = -1001000
	DATATYPE_EMPTY_IN_STRUCT_ERR             ErrorCode = -1002000
	DATASIZE_EMPTY_IN_STRUCT_ERR             ErrorCode = -1003000
	CHKSUM_EMPTY_IN_STRUCT_ERR               ErrorCode = -1004000
	VERSION_EMPTY_IN_STRUCT_ERR              ErrorCode = -1005000
	FILEPATH_EMPTY_IN_STRUCT_ERR             ErrorCode = -1006000
	REPLNUM_EMPTY_IN_STRUCT_ERR              ErrorCode = -1007000
	REPLSTATUS_EMPTY_IN_STRUCT_ERR           ErrorCode = -1008000
	DATAOWNER_EMPTY_IN_STRUCT_ERR            ErrorCode = -1009000
	DATAOWNERZONE_EMPTY_IN_STRUCT_ERR        ErrorCode = -1010000
	DATAEXPIRY_EMPTY_IN_STRUCT_ERR           ErrorCode = -1011000
	DATACOMMENTS_EMPTY_IN_STRUCT_ERR         ErrorCode = -1012000
	DATACREATE_EMPTY_IN_STRUCT_ERR           ErrorCode = -1013000
	DATAMODIFY_EMPTY_IN_STRUCT_ERR           ErrorCode = -1014000
	DATAACCESS_EMPTY_IN_STRUCT_ERR           ErrorCode = -1015000
	DATAACCESSINX_EMPTY_IN_STRUCT_ERR        ErrorCode = -1016000
	NO_RULE_FOUND_ERR                        ErrorCode = -1017000
	NO_MORE_RULES_ERR                        ErrorCode = -1018000
	UNMATCHED_ACTION_ERR                     ErrorCode = -1019000
	RULES_FILE_READ_ERROR                    ErrorCode = -1020000
	ACTION_ARG_COUNT_MISMATCH                ErrorCode = -1021000
	MAX_NUM_OF_ARGS_IN_ACTION_EXCEEDED       ErrorCode = -1022000
	UNKNOWN_PARAM_IN_RULE_ERR                ErrorCode = -1023000
	DESTRESCNAME_EMPTY_IN_STRUCT_ERR         ErrorCode = -1024000
	BACKUPRESCNAME_EMPTY_IN_STRUCT_ERR       ErrorCode = -1025000
	DATAID_EMPTY_IN_STRUCT_ERR               ErrorCode = -1026000
	COLLID_EMPTY_IN_STRUCT_ERR               ErrorCode = -1027000
	RESCGROUPNAME_EMPTY_IN_STRUCT_ERR        ErrorCode = -1028000
	STATUSSTRING_EMPTY_IN_STRUCT_ERR         ErrorCode = -1029000
	DATAMAPID_EMPTY_IN_STRUCT_ERR            ErrorCode = -1030000
	USERNAMECLIENT_EMPTY_IN_STRUCT_ERR       ErrorCode = -1031000
	RODSZONECLIENT_EMPTY_IN_STRUCT_ERR       ErrorCode = -1032000
	USERTYPECLIENT_EMPTY_IN_STRUCT_ERR       ErrorCode = -1033000
	HOSTCLIENT_EMPTY_IN_STRUCT_ERR           ErrorCode = -1034000
	AUTHSTRCLIENT_EMPTY_IN_STRUCT_ERR        ErrorCode = -1035000
	USERAUTHSCHEMECLIENT_EMPTY_IN_STRUCT_ERR ErrorCode = -1036000
	USERINFOCLIENT_EMPTY_IN_STRUCT_ERR       ErrorCode = -1037000
	USERCOMMENTCLIENT_EMPTY_IN_STRUCT_ERR    ErrorCode = -1038000
	USERCREATECLIENT_EMPTY_IN_STRUCT_ERR     ErrorCode = -1039000
	USERMODIFYCLIENT_EMPTY_IN_STRUCT_ERR     ErrorCode = -1040000
	USERNAMEPROXY_EMPTY_IN_STRUCT_ERR        ErrorCode = -1041000
	RODSZONEPROXY_EMPTY_IN_STRUCT_ERR        ErrorCode = -1042000
	USERTYPEPROXY_EMPTY_IN_STRUCT_ERR        ErrorCode = -1043000
	HOSTPROXY_EMPTY_IN_STRUCT_ERR            ErrorCode = -1044000
	AUTHSTRPROXY_EMPTY_IN_STRUCT_ERR         ErrorCode = -1045000
	USERAUTHSCHEMEPROXY_EMPTY_IN_STRUCT_ERR  ErrorCode = -1046000
	USERINFOPROXY_EMPTY_IN_STRUCT_ERR        ErrorCode = -1047000
	USERCOMMENTPROXY_EMPTY_IN_STRUCT_ERR     ErrorCode = -1048000
	USERCREATEPROXY_EMPTY_IN_STRUCT_ERR      ErrorCode = -1049000
	USERMODIFYPROXY_EMPTY_IN_STRUCT_ERR      ErrorCode = -1050000
	COLLNAME_EMPTY_IN_STRUCT_ERR             ErrorCode = -1051000
	COLLPARENTNAME_EMPTY_IN_STRUCT_ERR       ErrorCode = -1052000
	COLLOWNERNAME_EMPTY_IN_STRUCT_ERR        ErrorCode = -1053000
	COLLOWNERZONE_EMPTY_IN_STRUCT_ERR        ErrorCode = -1054000
	COLLEXPIRY_EMPTY_IN_STRUCT_ERR           ErrorCode = -1055000
	COLLCOMMENTS_EMPTY_IN_STRUCT_ERR         ErrorCode = -1056000
	COLLCREATE_EMPTY_IN_STRUCT_ERR           ErrorCode = -1057000
	COLLMODIFY_EMPTY_IN_STRUCT_ERR           ErrorCode = -1058000
	COLLACCESS_EMPTY_IN_STRUCT_ERR           ErrorCode = -1059000
	COLLACCESSINX_EMPTY_IN_STRUCT_ERR        ErrorCode = -1060000
	COLLMAPID_EMPTY_IN_STRUCT_ERR            ErrorCode = -1062000
	COLLINHERITANCE_EMPTY_IN_STRUCT_ERR      ErrorCode = -1063000
	RESCZONE_EMPTY_IN_STRUCT_ERR             ErrorCode = -1065000
	RESCLOC_EMPTY_IN_STRUCT_ERR              ErrorCode = -1066000
	RESCTYPE_EMPTY_IN_STRUCT_ERR             ErrorCode = -1067000
	RESCTYPEINX_EMPTY_IN_STRUCT_ERR          ErrorCode = -1068000
	RESCCLASS_EMPTY_IN_STRUCT_ERR            ErrorCode = -1069000
	RESCCLASSINX_EMPTY_IN_STRUCT_ERR         ErrorCode = -1070000
	RESCVAULTPATH_EMPTY_IN_STRUCT_ERR        ErrorCode = -1071000
	NUMOPEN_ORTS_EMPTY_IN_STRUCT_ERR         ErrorCode = -1072000
	PARAOPR_EMPTY_IN_STRUCT_ERR              ErrorCode = -1073000
	RESCID_EMPTY_IN_STRUCT_ERR               ErrorCode = -1074000
	GATEWAYADDR_EMPTY_IN_STRUCT_ERR          ErrorCode = -1075000
	RESCMAX_BJSIZE_EMPTY_IN_STRUCT_ERR       ErrorCode = -1076000
	FREESPACE_EMPTY_IN_STRUCT_ERR            ErrorCode = -1077000
	FREESPACETIME_EMPTY_IN_STRUCT_ERR        ErrorCode = -1078000
	FREESPACETIMESTAMP_EMPTY_IN_STRUCT_ERR   ErrorCode = -1079000
	RESCINFO_EMPTY_IN_STRUCT_ERR             ErrorCode = -1080000
	RESCCOMMENTS_EMPTY_IN_STRUCT_ERR         ErrorCode = -1081000
	RESCCREATE_EMPTY_IN_STRUCT_ERR           ErrorCode = -1082000
	RESCMODIFY_EMPTY_IN_STRUCT_ERR           ErrorCode = -1083000
	INPUT_ARG_NOT_WELL_FORMED_ERR            ErrorCode = -1084000
	INPUT_ARG_OUT_OF_ARGC_RANGE_ERR          ErrorCode = -1085000
	INSUFFICIENT_INPUT_ARG_ERR               ErrorCode = -1086000
	INPUT_ARG_DOES_NOT_MATCH_ERR             ErrorCode = -1087000
	RETRY_WITHOUT_RECOVERY_ERR               ErrorCode = -1088000
	CUT_ACTION_PROCESSED_ERR                 ErrorCode = -1089000
	ACTION_FAILED_ERR                        ErrorCode = -1090000
	FAIL_ACTION_ENCOUNTERED_ERR              ErrorCode = -1091000
	VARIABLE_NAME_TOO_LONG_ERR               ErrorCode = -1092000
	UNKNOWN_VARIABLE_MAP_ERR                 ErrorCode = -1093000
	UNDEFINED_VARIABLE_MAP_ERR               ErrorCode = -1094000
	NULL_VALUE_ERR                           ErrorCode = -1095000
	DVARMAP_FILE_READ_ERROR                  ErrorCode = -1096000
	NO_RULE_OR_MSI_FUNCTION_FOUND_ERR        ErrorCode = -1097000
	FILE_CREATE_ERROR                        ErrorCode = -1098000
	FMAP_FILE_READ_ERROR                     ErrorCode = -1099000
	DATE_FORMAT_ERR                          ErrorCode = -1100000
	RULE_FAILED_ERR                          ErrorCode = -1101000
	NO_MICROSERVICE_FOUND_ERR                ErrorCode = -1102000
	INVALID_REGEXP                           ErrorCode = -1103000
	INVALID_OBJECT_NAME                      ErrorCode = -1104000
	INVALID_OBJECT_TYPE                      ErrorCode = -1105000
	NO_VALUES_FOUND                          ErrorCode = -1106000
	NO_COLUMN_NAME_FOUND                     ErrorCode = -1107000
	BREAK_ACTION_ENCOUNTERED_ERR             ErrorCode = -1108000
	CUT_ACTION_ON_SUCCESS_PROCESSED_ERR      ErrorCode = -1109000
	MSI_OPERATION_NOT_ALLOWED                ErrorCode = -1110000
	MAX_NUM_OF_ACTION_IN_RULE_EXCEEDED       ErrorCode = -1111000
	MSRVC_FILE_READ_ERROR                    ErrorCode = -1112000
	MSRVC_VERSION_MISMATCH                   ErrorCode = -1113000
	MICRO_SERVICE_OBJECT_TYPE_UNDEFINED      ErrorCode = -1114000
	MSO_OBJ_GET_FAILED                       ErrorCode = -1115000
	REMOTE_IRODS_CONNECT_ERR                 ErrorCode = -1116000
	REMOTE_SRB_CONNECT_ERR                   ErrorCode = -1117000
	MSO_OBJ_PUT_FAILED                       ErrorCode = -1118000
	RE_PARSER_ERROR                          ErrorCode = -1201000
	RE_UNPARSED_SUFFIX                       ErrorCode = -1202000
	RE_POINTER_ERROR                         ErrorCode = -1203000
	RE_RUNTIME_ERROR                         ErrorCode = -1205000
	RE_DIVISION_BY_ZERO                      ErrorCode = -1206000
	RE_BUFFER_OVERFLOW                       ErrorCode = -1207000
	RE_UNSUPPORTED_OP_OR_TYPE                ErrorCode = -1208000
	RE_UNSUPPORTED_SESSION_VAR               ErrorCode = -1209000
	RE_UNABLE_TO_WRITE_LOCAL_VAR             ErrorCode = -1210000
	RE_UNABLE_TO_READ_LOCAL_VAR              ErrorCode = -1211000
	RE_UNABLE_TO_WRITE_SESSION_VAR           ErrorCode = -1212000
	RE_UNABLE_TO_READ_SESSION_VAR            ErrorCode = -1213000
	RE_UNABLE_TO_WRITE_VAR                   ErrorCode = -1214000
	RE_UNABLE_TO_READ_VAR                    ErrorCode = -1215000
	RE_PATTERN_NOT_MATCHED                   ErrorCode = -1216000
	RE_STRING_OVERFLOW                       ErrorCode = -1217000
	RE_UNKNOWN_ERROR                         ErrorCode = -1220000
	RE_OUT_OF_MEMORY                         ErrorCode = -1221000
	RE_SHM_UNLINK_ERROR                      ErrorCode = -1222000
	RE_FILE_STAT_ERROR                       ErrorCode = -1223000
	RE_UNSUPPORTED_AST_NODE_TYPE             ErrorCode = -1224000
	RE_UNSUPPORTED_SESSION_VAR_TYPE          ErrorCode = -1225000
	RE_TYPE_ERROR                            ErrorCode = -1230000
	RE_FUNCTION_REDEFINITION                 ErrorCode = -1231000
	RE_DYNAMIC_TYPE_ERROR                    ErrorCode = -1232000
	RE_DYNAMIC_COERCION_ERROR                ErrorCode = -1233000
	RE_PACKING_ERROR                         ErrorCode = -1234000
	PHP_EXEC_SCRIPT_ERR                      ErrorCode = -1600000
	PHP_REQUEST_STARTUP_ERR                  ErrorCode = -1601000
	PHP_OPEN_SCRIPT_FILE_ERR                 ErrorCode = -1602000
	KEY_NOT_FOUND                            ErrorCode = -1800000
	KEY_TYPE_MISMATCH                        ErrorCode = -1801000
	CHILD_EXISTS                             ErrorCode = -1802000
	HIERARCHY_ERROR                          ErrorCode = -1803000
	CHILD_NOT_FOUND                          ErrorCode = -1804000
	NO_NEXT_RESC_FOUND                       ErrorCode = -1805000
	NO_PDMO_DEFINED                          ErrorCode = -1806000
	INVALID_LOCATION                         ErrorCode = -1807000
	PLUGIN_ERROR                             ErrorCode = -1808000
	INVALID_RESC_CHILD_CONTEXT               ErrorCode = -1809000
	INVALID_FILE_OBJECT                      ErrorCode = -1810000
	INVALID_OPERATION                        ErrorCode = -1811000
	CHILD_HAS_PARENT                         ErrorCode = -1812000
	FILE_NOT_IN_VAULT                        ErrorCode = -1813000
	DIRECT_ARCHIVE_ACCESS                    ErrorCode = -1814000
	ADVANCED_NEGOTIATION_NOT_SUPPORTED       ErrorCode = -1815000
	DIRECT_CHILD_ACCESS                      ErrorCode = -1816000
	INVALID_DYNAMIC_CAST                     ErrorCode = -1817000
	INVALID_ACCESS_TO_IMPOSTOR_RESOURCE      ErrorCode = -1818000
	INVALID_LEXICAL_CAST                     ErrorCode = -1819000
	CONTROL_PLANE_MESSAGE_ERROR              ErrorCode = -1820000
	REPLICA_NOT_IN_RESC                      ErrorCode = -1821000
	INVALID_ANY_CAST                         ErrorCode = -1822000
	BAD_FUNCTION_CALL                        ErrorCode = -1823000
	CLIENT_NEGOTIATION_ERROR                 ErrorCode = -1824000
	SERVER_NEGOTIATION_ERROR                 ErrorCode = -1825000
	INVALID_KVP_STRING                       ErrorCode = -1826000
	PLUGIN_ERROR_MISSING_SHARED_OBJECT       ErrorCode = -1827000
	RULE_ENGINE_ERROR                        ErrorCode = -1828000
	REBALANCE_ALREADY_ACTIVE_ON_RESOURCE     ErrorCode = -1829000
	NETCDF_OPEN_ERR                          ErrorCode = -2000000
	NETCDF_CREATE_ERR                        ErrorCode = -2001000
	NETCDF_CLOSE_ERR                         ErrorCode = -2002000
	NETCDF_INVALID_PARAM_TYPE                ErrorCode = -2003000
	NETCDF_INQ_ID_ERR                        ErrorCode = -2004000
	NETCDF_GET_VARS_ERR                      ErrorCode = -2005000
	NETCDF_INVALID_DATA_TYPE                 ErrorCode = -2006000
	NETCDF_INQ_VARS_ERR                      ErrorCode = -2007000
	NETCDF_VARS_DATA_TOO_BIG                 ErrorCode = -2008000
	NETCDF_DIM_MISMATCH_ERR                  ErrorCode = -2009000
	NETCDF_INQ_ERR                           ErrorCode = -2010000
	NETCDF_INQ_FORMAT_ERR                    ErrorCode = -2011000
	NETCDF_INQ_DIM_ERR                       ErrorCode = -2012000
	NETCDF_INQ_ATT_ERR                       ErrorCode = -2013000
	NETCDF_GET_ATT_ERR                       ErrorCode = -2014000
	NETCDF_VAR_COUNT_OUT_OF_RANGE            ErrorCode = -2015000
	NETCDF_UNMATCHED_NAME_ERR                ErrorCode = -2016000
	NETCDF_NO_UNLIMITED_DIM                  ErrorCode = -2017000
	NETCDF_PUT_ATT_ERR                       ErrorCode = -2018000
	NETCDF_DEF_DIM_ERR                       ErrorCode = -2019000
	NETCDF_DEF_VAR_ERR                       ErrorCode = -2020000
	NETCDF_PUT_VARS_ERR                      ErrorCode = -2021000
	NETCDF_AGG_INFO_FILE_ERR                 ErrorCode = -2022000
	NETCDF_AGG_ELE_INX_OUT_OF_RANGE          ErrorCode = -2023000
	NETCDF_AGG_ELE_FILE_NOT_OPENED           ErrorCode = -2024000
	NETCDF_AGG_ELE_FILE_NO_TIME_DIM          ErrorCode = -2025000
	SSL_NOT_BUILT_INTO_CLIENT                ErrorCode = -2100000
	SSL_NOT_BUILT_INTO_SERVER                ErrorCode = -2101000
	SSL_INIT_ERROR                           ErrorCode = -2102000
	SSL_HANDSHAKE_ERROR                      ErrorCode = -2103000
	SSL_SHUTDOWN_ERROR                       ErrorCode = -2104000
	SSL_CERT_ERROR                           ErrorCode = -2105000
	OOI_CURL_EASY_INIT_ERR                   ErrorCode = -2200000
	OOI_JSON_OBJ_SET_ERR                     ErrorCode = -2201000
	OOI_DICT_TYPE_NOT_SUPPORTED              ErrorCode = -2202000
	OOI_JSON_PACK_ERR                        ErrorCode = -2203000
	OOI_JSON_DUMP_ERR                        ErrorCode = -2204000
	OOI_CURL_EASY_PERFORM_ERR                ErrorCode = -2205000
	OOI_JSON_LOAD_ERR                        ErrorCode = -2206000
	OOI_JSON_GET_ERR                         ErrorCode = -2207000
	OOI_JSON_NO_ANSWER_ERR                   ErrorCode = -2208000
	OOI_JSON_TYPE_ERR                        ErrorCode = -2209000
	OOI_JSON_INX_OUT_OF_RANGE                ErrorCode = -2210000
	OOI_REVID_NOT_FOUND                      ErrorCode = -2211000
	DEPRECATED_PARAMETER                     ErrorCode = -3000000
	XML_PARSING_ERR                          ErrorCode = -2300000
	OUT_OF_URL_PATH                          ErrorCode = -2301000
	URL_PATH_INX_OUT_OF_RANGE                ErrorCode = -2302000
	SYS_NULL_INPUT                           ErrorCode = -99999996
	SYS_HANDLER_DONE_WITH_ERROR              ErrorCode = -99999997
	SYS_HANDLER_DONE_NO_ERROR                ErrorCode = -99999998
	SYS_NO_HANDLER_REPLY_MSG                 ErrorCode = -99999999
)

var ErrorCodes = map[ErrorCode]string{
	SYS_SOCK_OPEN_ERR:                      "SYS_SOCK_OPEN_ERR",
	SYS_SOCK_LISTEN_ERR:                    "SYS_SOCK_LISTEN_ERR",
	SYS_SOCK_BIND_ERR:                      "SYS_SOCK_BIND_ERR",
	SYS_SOCK_ACCEPT_ERR:                    "SYS_SOCK_ACCEPT_ERR",
	SYS_HEADER_READ_LEN_ERR:                "SYS_HEADER_READ_LEN_ERR",
	SYS_HEADER_WRITE_LEN_ERR:               "SYS_HEADER_WRITE_LEN_ERR",
	SYS_HEADER_TPYE_LEN_ERR:                "SYS_HEADER_TPYE_LEN_ERR",
	SYS_CAUGHT_SIGNAL:                      "SYS_CAUGHT_SIGNAL",
	SYS_GETSTARTUP_PACK_ERR:                "SYS_GETSTARTUP_PACK_ERR",
	SYS_EXCEED_CONNECT_CNT:                 "SYS_EXCEED_CONNECT_CNT",
	SYS_USER_NOT_ALLOWED_TO_CONN:           "SYS_USER_NOT_ALLOWED_TO_CONN",
	SYS_READ_MSG_BODY_INPUT_ERR:            "SYS_READ_MSG_BODY_INPUT_ERR",
	SYS_UNMATCHED_API_NUM:                  "SYS_UNMATCHED_API_NUM",
	SYS_NO_API_PRIV:                        "SYS_NO_API_PRIV",
	SYS_API_INPUT_ERR:                      "SYS_API_INPUT_ERR",
	SYS_PACK_INSTRUCT_FORMAT_ERR:           "SYS_PACK_INSTRUCT_FORMAT_ERR",
	SYS_MALLOC_ERR:                         "SYS_MALLOC_ERR",
	SYS_GET_HOSTNAME_ERR:                   "SYS_GET_HOSTNAME_ERR",
	SYS_OUT_OF_FILE_DESC:                   "SYS_OUT_OF_FILE_DESC",
	SYS_FILE_DESC_OUT_OF_RANGE:             "SYS_FILE_DESC_OUT_OF_RANGE",
	SYS_UNRECOGNIZED_REMOTE_FLAG:           "SYS_UNRECOGNIZED_REMOTE_FLAG",
	SYS_INVALID_SERVER_HOST:                "SYS_INVALID_SERVER_HOST",
	SYS_SVR_TO_SVR_CONNECT_FAILED:          "SYS_SVR_TO_SVR_CONNECT_FAILED",
	SYS_BAD_FILE_DESCRIPTOR:                "SYS_BAD_FILE_DESCRIPTOR",
	SYS_INTERNAL_NULL_INPUT_ERR:            "SYS_INTERNAL_NULL_INPUT_ERR",
	SYS_CONFIG_FILE_ERR:                    "SYS_CONFIG_FILE_ERR",
	SYS_INVALID_ZONE_NAME:                  "SYS_INVALID_ZONE_NAME",
	SYS_COPY_LEN_ERR:                       "SYS_COPY_LEN_ERR",
	SYS_PORT_COOKIE_ERR:                    "SYS_PORT_COOKIE_ERR",
	SYS_KEY_VAL_TABLE_ERR:                  "SYS_KEY_VAL_TABLE_ERR",
	SYS_INVALID_RESC_TYPE:                  "SYS_INVALID_RESC_TYPE",
	SYS_INVALID_FILE_PATH:                  "SYS_INVALID_FILE_PATH",
	SYS_INVALID_RESC_INPUT:                 "SYS_INVALID_RESC_INPUT",
	SYS_INVALID_PORTAL_OPR:                 "SYS_INVALID_PORTAL_OPR",
	SYS_PARA_OPR_NO_SUPPORT:                "SYS_PARA_OPR_NO_SUPPORT",
	SYS_INVALID_OPR_TYPE:                   "SYS_INVALID_OPR_TYPE",
	SYS_NO_PATH_PERMISSION:                 "SYS_NO_PATH_PERMISSION",
	SYS_NO_ICAT_SERVER_ERR:                 "SYS_NO_ICAT_SERVER_ERR",
	SYS_AGENT_INIT_ERR:                     "SYS_AGENT_INIT_ERR",
	SYS_PROXYUSER_NO_PRIV:                  "SYS_PROXYUSER_NO_PRIV",
	SYS_NO_DATA_OBJ_PERMISSION:             "SYS_NO_DATA_OBJ_PERMISSION",
	SYS_DELETE_DISALLOWED:                  "SYS_DELETE_DISALLOWED",
	SYS_OPEN_REI_FILE_ERR:                  "SYS_OPEN_REI_FILE_ERR",
	SYS_NO_RCAT_SERVER_ERR:                 "SYS_NO_RCAT_SERVER_ERR",
	SYS_UNMATCH_PACK_INSTRUCTI_NAME:        "SYS_UNMATCH_PACK_INSTRUCTI_NAME",
	SYS_SVR_TO_CLI_MSI_NO_EXIST:            "SYS_SVR_TO_CLI_MSI_NO_EXIST",
	SYS_COPY_ALREADY_IN_RESC:               "SYS_COPY_ALREADY_IN_RESC",
	SYS_RECONN_OPR_MISMATCH:                "SYS_RECONN_OPR_MISMATCH",
	SYS_INPUT_PERM_OUT_OF_RANGE:            "SYS_INPUT_PERM_OUT_OF_RANGE",
	SYS_FORK_ERROR:                         "SYS_FORK_ERROR",
	SYS_PIPE_ERROR:                         "SYS_PIPE_ERROR",
	SYS_EXEC_CMD_STATUS_SZ_ERROR:           "SYS_EXEC_CMD_STATUS_SZ_ERROR",
	SYS_PATH_IS_NOT_A_FILE:                 "SYS_PATH_IS_NOT_A_FILE",
	SYS_UNMATCHED_SPEC_COLL_TYPE:           "SYS_UNMATCHED_SPEC_COLL_TYPE",
	SYS_TOO_MANY_QUERY_RESULT:              "SYS_TOO_MANY_QUERY_RESULT",
	SYS_SPEC_COLL_NOT_IN_CACHE:             "SYS_SPEC_COLL_NOT_IN_CACHE",
	SYS_SPEC_COLL_OBJ_NOT_EXIST:            "SYS_SPEC_COLL_OBJ_NOT_EXIST",
	SYS_REG_OBJ_IN_SPEC_COLL:               "SYS_REG_OBJ_IN_SPEC_COLL",
	SYS_DEST_SPEC_COLL_SUB_EXIST:           "SYS_DEST_SPEC_COLL_SUB_EXIST",
	SYS_SRC_DEST_SPEC_COLL_CONFLICT:        "SYS_SRC_DEST_SPEC_COLL_CONFLICT",
	SYS_UNKNOWN_SPEC_COLL_CLASS:            "SYS_UNKNOWN_SPEC_COLL_CLASS",
	SYS_DUPLICATE_XMSG_TICKET:              "SYS_DUPLICATE_XMSG_TICKET",
	SYS_UNMATCHED_XMSG_TICKET:              "SYS_UNMATCHED_XMSG_TICKET",
	SYS_NO_XMSG_FOR_MSG_NUMBER:             "SYS_NO_XMSG_FOR_MSG_NUMBER",
	SYS_COLLINFO_2_FORMAT_ERR:              "SYS_COLLINFO_2_FORMAT_ERR",
	SYS_CACHE_STRUCT_FILE_RESC_ERR:         "SYS_CACHE_STRUCT_FILE_RESC_ERR",
	SYS_NOT_SUPPORTED:                      "SYS_NOT_SUPPORTED",
	SYS_TAR_STRUCT_FILE_EXTRACT_ERR:        "SYS_TAR_STRUCT_FILE_EXTRACT_ERR",
	SYS_STRUCT_FILE_DESC_ERR:               "SYS_STRUCT_FILE_DESC_ERR",
	SYS_TAR_OPEN_ERR:                       "SYS_TAR_OPEN_ERR",
	SYS_TAR_EXTRACT_ALL_ERR:                "SYS_TAR_EXTRACT_ALL_ERR",
	SYS_TAR_CLOSE_ERR:                      "SYS_TAR_CLOSE_ERR",
	SYS_STRUCT_FILE_PATH_ERR:               "SYS_STRUCT_FILE_PATH_ERR",
	SYS_MOUNT_MOUNTED_COLL_ERR:             "SYS_MOUNT_MOUNTED_COLL_ERR",
	SYS_COLL_NOT_MOUNTED_ERR:               "SYS_COLL_NOT_MOUNTED_ERR",
	SYS_STRUCT_FILE_BUSY_ERR:               "SYS_STRUCT_FILE_BUSY_ERR",
	SYS_STRUCT_FILE_INMOUNTED_COLL:         "SYS_STRUCT_FILE_INMOUNTED_COLL",
	SYS_COPY_NOT_EXIST_IN_RESC:             "SYS_COPY_NOT_EXIST_IN_RESC",
	SYS_RESC_DOES_NOT_EXIST:                "SYS_RESC_DOES_NOT_EXIST",
	SYS_COLLECTION_NOT_EMPTY:               "SYS_COLLECTION_NOT_EMPTY",
	SYS_OBJ_TYPE_NOT_STRUCT_FILE:           "SYS_OBJ_TYPE_NOT_STRUCT_FILE",
	SYS_WRONG_RESC_POLICY_FOR_BUN_OPR:      "SYS_WRONG_RESC_POLICY_FOR_BUN_OPR",
	SYS_DIR_IN_VAULT_NOT_EMPTY:             "SYS_DIR_IN_VAULT_NOT_EMPTY",
	SYS_OPR_FLAG_NOT_SUPPORT:               "SYS_OPR_FLAG_NOT_SUPPORT",
	SYS_TAR_APPEND_ERR:                     "SYS_TAR_APPEND_ERR",
	SYS_INVALID_PROTOCOL_TYPE:              "SYS_INVALID_PROTOCOL_TYPE",
	SYS_UDP_CONNECT_ERR:                    "SYS_UDP_CONNECT_ERR",
	SYS_UDP_TRANSFER_ERR:                   "SYS_UDP_TRANSFER_ERR",
	SYS_UDP_NO_SUPPORT_ERR:                 "SYS_UDP_NO_SUPPORT_ERR",
	SYS_READ_MSG_BODY_LEN_ERR:              "SYS_READ_MSG_BODY_LEN_ERR",
	CROSS_ZONE_SOCK_CONNECT_ERR:            "CROSS_ZONE_SOCK_CONNECT_ERR",
	SYS_NO_FREE_RE_THREAD:                  "SYS_NO_FREE_RE_THREAD",
	SYS_BAD_RE_THREAD_INX:                  "SYS_BAD_RE_THREAD_INX",
	SYS_CANT_DIRECTLY_ACC_COMPOUND_RESC:    "SYS_CANT_DIRECTLY_ACC_COMPOUND_RESC",
	SYS_SRC_DEST_RESC_COMPOUND_TYPE:        "SYS_SRC_DEST_RESC_COMPOUND_TYPE",
	SYS_CACHE_RESC_NOT_ON_SAME_HOST:        "SYS_CACHE_RESC_NOT_ON_SAME_HOST",
	SYS_NO_CACHE_RESC_IN_GRP:               "SYS_NO_CACHE_RESC_IN_GRP",
	SYS_UNMATCHED_RESC_IN_RESC_GRP:         "SYS_UNMATCHED_RESC_IN_RESC_GRP",
	SYS_CANT_MV_BUNDLE_DATA_TO_TRASH:       "SYS_CANT_MV_BUNDLE_DATA_TO_TRASH",
	SYS_CANT_MV_BUNDLE_DATA_BY_COPY:        "SYS_CANT_MV_BUNDLE_DATA_BY_COPY",
	SYS_EXEC_TAR_ERR:                       "SYS_EXEC_TAR_ERR",
	SYS_CANT_CHKSUM_COMP_RESC_DATA:         "SYS_CANT_CHKSUM_COMP_RESC_DATA",
	SYS_CANT_CHKSUM_BUNDLED_DATA:           "SYS_CANT_CHKSUM_BUNDLED_DATA",
	SYS_RESC_IS_DOWN:                       "SYS_RESC_IS_DOWN",
	SYS_UPDATE_REPL_INFO_ERR:               "SYS_UPDATE_REPL_INFO_ERR",
	SYS_COLL_LINK_PATH_ERR:                 "SYS_COLL_LINK_PATH_ERR",
	SYS_LINK_CNT_EXCEEDED_ERR:              "SYS_LINK_CNT_EXCEEDED_ERR",
	SYS_CROSS_ZONE_MV_NOT_SUPPORTED:        "SYS_CROSS_ZONE_MV_NOT_SUPPORTED",
	SYS_RESC_QUOTA_EXCEEDED:                "SYS_RESC_QUOTA_EXCEEDED",
	SYS_RENAME_STRUCT_COUNT_EXCEEDED:       "SYS_RENAME_STRUCT_COUNT_EXCEEDED",
	SYS_BULK_REG_COUNT_EXCEEDED:            "SYS_BULK_REG_COUNT_EXCEEDED",
	SYS_REQUESTED_BUF_TOO_LARGE:            "SYS_REQUESTED_BUF_TOO_LARGE",
	SYS_INVALID_RESC_FOR_BULK_OPR:          "SYS_INVALID_RESC_FOR_BULK_OPR",
	SYS_SOCK_READ_TIMEDOUT:                 "SYS_SOCK_READ_TIMEDOUT",
	SYS_SOCK_READ_ERR:                      "SYS_SOCK_READ_ERR",
	SYS_CONNECT_CONTROL_CONFIG_ERR:         "SYS_CONNECT_CONTROL_CONFIG_ERR",
	SYS_MAX_CONNECT_COUNT_EXCEEDED:         "SYS_MAX_CONNECT_COUNT_EXCEEDED",
	SYS_STRUCT_ELEMENT_MISMATCH:            "SYS_STRUCT_ELEMENT_MISMATCH",
	SYS_PHY_PATH_INUSE:                     "SYS_PHY_PATH_INUSE",
	SYS_USER_NO_PERMISSION:                 "SYS_USER_NO_PERMISSION",
	SYS_USER_RETRIEVE_ERR:                  "SYS_USER_RETRIEVE_ERR",
	SYS_FS_LOCK_ERR:                        "SYS_FS_LOCK_ERR",
	SYS_LOCK_TYPE_INP_ERR:                  "SYS_LOCK_TYPE_INP_ERR",
	SYS_LOCK_CMD_INP_ERR:                   "SYS_LOCK_CMD_INP_ERR",
	SYS_ZIP_FORMAT_NOT_SUPPORTED:           "SYS_ZIP_FORMAT_NOT_SUPPORTED",
	SYS_ADD_TO_ARCH_OPR_NOT_SUPPORTED:      "SYS_ADD_TO_ARCH_OPR_NOT_SUPPORTED",
	CANT_REG_IN_VAULT_FILE:                 "CANT_REG_IN_VAULT_FILE",
	PATH_REG_NOT_ALLOWED:                   "PATH_REG_NOT_ALLOWED",
	SYS_INVALID_INPUT_PARAM:                "SYS_INVALID_INPUT_PARAM",
	SYS_GROUP_RETRIEVE_ERR:                 "SYS_GROUP_RETRIEVE_ERR",
	SYS_MSSO_APPEND_ERR:                    "SYS_MSSO_APPEND_ERR",
	SYS_MSSO_STRUCT_FILE_EXTRACT_ERR:       "SYS_MSSO_STRUCT_FILE_EXTRACT_ERR",
	SYS_MSSO_EXTRACT_ALL_ERR:               "SYS_MSSO_EXTRACT_ALL_ERR",
	SYS_MSSO_OPEN_ERR:                      "SYS_MSSO_OPEN_ERR",
	SYS_MSSO_CLOSE_ERR:                     "SYS_MSSO_CLOSE_ERR",
	SYS_RULE_NOT_FOUND:                     "SYS_RULE_NOT_FOUND",
	SYS_NOT_IMPLEMENTED:                    "SYS_NOT_IMPLEMENTED",
	SYS_SIGNED_SID_NOT_MATCHED:             "SYS_SIGNED_SID_NOT_MATCHED",
	SYS_HASH_IMMUTABLE:                     "SYS_HASH_IMMUTABLE",
	SYS_UNINITIALIZED:                      "SYS_UNINITIALIZED",
	SYS_NEGATIVE_SIZE:                      "SYS_NEGATIVE_SIZE",
	SYS_ALREADY_INITIALIZED:                "SYS_ALREADY_INITIALIZED",
	SYS_SETENV_ERR:                         "SYS_SETENV_ERR",
	SYS_GETENV_ERR:                         "SYS_GETENV_ERR",
	SYS_INTERNAL_ERR:                       "SYS_INTERNAL_ERR",
	SYS_SOCK_SELECT_ERR:                    "SYS_SOCK_SELECT_ERR",
	SYS_THREAD_ENCOUNTERED_INTERRUPT:       "SYS_THREAD_ENCOUNTERED_INTERRUPT",
	SYS_THREAD_RESOURCE_ERR:                "SYS_THREAD_RESOURCE_ERR",
	SYS_BAD_INPUT:                          "SYS_BAD_INPUT",
	SYS_PORT_RANGE_EXHAUSTED:               "SYS_PORT_RANGE_EXHAUSTED",
	SYS_SERVICE_ROLE_NOT_SUPPORTED:         "SYS_SERVICE_ROLE_NOT_SUPPORTED",
	SYS_SOCK_WRITE_ERR:                     "SYS_SOCK_WRITE_ERR",
	SYS_SOCK_CONNECT_ERR:                   "SYS_SOCK_CONNECT_ERR",
	SYS_OPERATION_IN_PROGRESS:              "SYS_OPERATION_IN_PROGRESS",
	SYS_REPLICA_DOES_NOT_EXIST:             "SYS_REPLICA_DOES_NOT_EXIST",
	SYS_UNKNOWN_ERROR:                      "SYS_UNKNOWN_ERROR",
	SYS_NO_GOOD_REPLICA:                    "SYS_NO_GOOD_REPLICA",
	SYS_LIBRARY_ERROR:                      "SYS_LIBRARY_ERROR",
	SYS_REPLICA_INACCESSIBLE:               "SYS_REPLICA_INACCESSIBLE",
	SYS_NOT_ALLOWED:                        "SYS_NOT_ALLOWED",
	NOT_A_COLLECTION:                       "NOT_A_COLLECTION",
	NOT_A_DATA_OBJECT:                      "NOT_A_DATA_OBJECT",
	JSON_VALIDATION_ERROR:                  "JSON_VALIDATION_ERROR",
	USER_AUTH_SCHEME_ERR:                   "USER_AUTH_SCHEME_ERR",
	USER_AUTH_STRING_EMPTY:                 "USER_AUTH_STRING_EMPTY",
	USER_RODS_HOST_EMPTY:                   "USER_RODS_HOST_EMPTY",
	USER_RODS_HOSTNAME_ERR:                 "USER_RODS_HOSTNAME_ERR",
	USER_SOCK_OPEN_ERR:                     "USER_SOCK_OPEN_ERR",
	USER_SOCK_CONNECT_ERR:                  "USER_SOCK_CONNECT_ERR",
	USER_STRLEN_TOOLONG:                    "USER_STRLEN_TOOLONG",
	USER_API_INPUT_ERR:                     "USER_API_INPUT_ERR",
	USER_PACKSTRUCT_INPUT_ERR:              "USER_PACKSTRUCT_INPUT_ERR",
	USER_NO_SUPPORT_ERR:                    "USER_NO_SUPPORT_ERR",
	USER_FILE_DOES_NOT_EXIST:               "USER_FILE_DOES_NOT_EXIST",
	USER_FILE_TOO_LARGE:                    "USER_FILE_TOO_LARGE",
	OVERWRITE_WITHOUT_FORCE_FLAG:           "OVERWRITE_WITHOUT_FORCE_FLAG",
	UNMATCHED_KEY_OR_INDEX:                 "UNMATCHED_KEY_OR_INDEX",
	USER_CHKSUM_MISMATCH:                   "USER_CHKSUM_MISMATCH",
	USER_BAD_KEYWORD_ERR:                   "USER_BAD_KEYWORD_ERR",
	USER__NULL_INPUT_ERR:                   "USER__NULL_INPUT_ERR",
	USER_INPUT_PATH_ERR:                    "USER_INPUT_PATH_ERR",
	USER_INPUT_OPTION_ERR:                  "USER_INPUT_OPTION_ERR",
	USER_INVALID_USERNAME_FORMAT:           "USER_INVALID_USERNAME_FORMAT",
	USER_DIRECT_RESC_INPUT_ERR:             "USER_DIRECT_RESC_INPUT_ERR",
	USER_NO_RESC_INPUT_ERR:                 "USER_NO_RESC_INPUT_ERR",
	USER_PARAM_LABEL_ERR:                   "USER_PARAM_LABEL_ERR",
	USER_PARAM_TYPE_ERR:                    "USER_PARAM_TYPE_ERR",
	BASE64_BUFFER_OVERFLOW:                 "BASE64_BUFFER_OVERFLOW",
	BASE64_INVALID_PACKET:                  "BASE64_INVALID_PACKET",
	USER_MSG_TYPE_NO_SUPPORT:               "USER_MSG_TYPE_NO_SUPPORT",
	USER_RSYNC_NO_MODE_INPUT_ERR:           "USER_RSYNC_NO_MODE_INPUT_ERR",
	USER_OPTION_INPUT_ERR:                  "USER_OPTION_INPUT_ERR",
	SAME_SRC_DEST_PATHS_ERR:                "SAME_SRC_DEST_PATHS_ERR",
	USER_RESTART_FILE_INPUT_ERR:            "USER_RESTART_FILE_INPUT_ERR",
	RESTART_OPR_FAILED:                     "RESTART_OPR_FAILED",
	BAD_EXEC_CMD_PATH:                      "BAD_EXEC_CMD_PATH",
	EXEC_CMD_OUTPUT_TOO_LARGE:              "EXEC_CMD_OUTPUT_TOO_LARGE",
	EXEC_CMD_ERROR:                         "EXEC_CMD_ERROR",
	BAD_INPUT_DESC_INDEX:                   "BAD_INPUT_DESC_INDEX",
	USER_PATH_EXCEEDS_MAX:                  "USER_PATH_EXCEEDS_MAX",
	USER_SOCK_CONNECT_TIMEDOUT:             "USER_SOCK_CONNECT_TIMEDOUT",
	USER_API_VERSION_MISMATCH:              "USER_API_VERSION_MISMATCH",
	USER_INPUT_FORMAT_ERR:                  "USER_INPUT_FORMAT_ERR",
	USER_ACCESS_DENIED:                     "USER_ACCESS_DENIED",
	CANT_RM_MV_BUNDLE_TYPE:                 "CANT_RM_MV_BUNDLE_TYPE",
	NO_MORE_RESULT:                         "NO_MORE_RESULT",
	NO_KEY_WD_IN_MS_INP_STR:                "NO_KEY_WD_IN_MS_INP_STR",
	CANT_RM_NON_EMPTY_HOME_COLL:            "CANT_RM_NON_EMPTY_HOME_COLL",
	CANT_UNREG_IN_VAULT_FILE:               "CANT_UNREG_IN_VAULT_FILE",
	NO_LOCAL_FILE_RSYNC_IN_MSI:             "NO_LOCAL_FILE_RSYNC_IN_MSI",
	BULK_OPR_MISMATCH_FOR_RESTART:          "BULK_OPR_MISMATCH_FOR_RESTART",
	OBJ_PATH_DOES_NOT_EXIST:                "OBJ_PATH_DOES_NOT_EXIST",
	SYMLINKED_BUNFILE_NOT_ALLOWED:          "SYMLINKED_BUNFILE_NOT_ALLOWED",
	USER_INPUT_STRING_ERR:                  "USER_INPUT_STRING_ERR",
	USER_INVALID_RESC_INPUT:                "USER_INVALID_RESC_INPUT",
	USER_NOT_ALLOWED_TO_EXEC_CMD:           "USER_NOT_ALLOWED_TO_EXEC_CMD",
	USER_HASH_TYPE_MISMATCH:                "USER_HASH_TYPE_MISMATCH",
	USER_INVALID_CLIENT_ENVIRONMENT:        "USER_INVALID_CLIENT_ENVIRONMENT",
	USER_INSUFFICIENT_FREE_INODES:          "USER_INSUFFICIENT_FREE_INODES",
	USER_FILE_SIZE_MISMATCH:                "USER_FILE_SIZE_MISMATCH",
	USER_INCOMPATIBLE_PARAMS:               "USER_INCOMPATIBLE_PARAMS",
	USER_INVALID_REPLICA_INPUT:             "USER_INVALID_REPLICA_INPUT",
	USER_INCOMPATIBLE_OPEN_FLAGS:           "USER_INCOMPATIBLE_OPEN_FLAGS",
	INTERMEDIATE_REPLICA_ACCESS:            "INTERMEDIATE_REPLICA_ACCESS",
	LOCKED_DATA_OBJECT_ACCESS:              "LOCKED_DATA_OBJECT_ACCESS",
	CHECK_VERIFICATION_RESULTS:             "CHECK_VERIFICATION_RESULTS",
	FILE_INDEX_LOOKUP_ERR:                  "FILE_INDEX_LOOKUP_ERR",
	UNIX_FILE_OPEN_ERR:                     "UNIX_FILE_OPEN_ERR",
	UNIX_FILE_CREATE_ERR:                   "UNIX_FILE_CREATE_ERR",
	UNIX_FILE_READ_ERR:                     "UNIX_FILE_READ_ERR",
	UNIX_FILE_WRITE_ERR:                    "UNIX_FILE_WRITE_ERR",
	UNIX_FILE_CLOSE_ERR:                    "UNIX_FILE_CLOSE_ERR",
	UNIX_FILE_UNLINK_ERR:                   "UNIX_FILE_UNLINK_ERR",
	UNIX_FILE_STAT_ERR:                     "UNIX_FILE_STAT_ERR",
	UNIX_FILE_FSTAT_ERR:                    "UNIX_FILE_FSTAT_ERR",
	UNIX_FILE_LSEEK_ERR:                    "UNIX_FILE_LSEEK_ERR",
	UNIX_FILE_FSYNC_ERR:                    "UNIX_FILE_FSYNC_ERR",
	UNIX_FILE_MKDIR_ERR:                    "UNIX_FILE_MKDIR_ERR",
	UNIX_FILE_RMDIR_ERR:                    "UNIX_FILE_RMDIR_ERR",
	UNIX_FILE_OPENDIR_ERR:                  "UNIX_FILE_OPENDIR_ERR",
	UNIX_FILE_CLOSEDIR_ERR:                 "UNIX_FILE_CLOSEDIR_ERR",
	UNIX_FILE_READDIR_ERR:                  "UNIX_FILE_READDIR_ERR",
	UNIX_FILE_STAGE_ERR:                    "UNIX_FILE_STAGE_ERR",
	UNIX_FILE_GET_FS_FREESPACE_ERR:         "UNIX_FILE_GET_FS_FREESPACE_ERR",
	UNIX_FILE_CHMOD_ERR:                    "UNIX_FILE_CHMOD_ERR",
	UNIX_FILE_RENAME_ERR:                   "UNIX_FILE_RENAME_ERR",
	UNIX_FILE_TRUNCATE_ERR:                 "UNIX_FILE_TRUNCATE_ERR",
	UNIX_FILE_LINK_ERR:                     "UNIX_FILE_LINK_ERR",
	UNIX_FILE_OPR_TIMEOUT_ERR:              "UNIX_FILE_OPR_TIMEOUT_ERR",
	UNIV_MSS_SYNCTOARCH_ERR:                "UNIV_MSS_SYNCTOARCH_ERR",
	UNIV_MSS_STAGETOCACHE_ERR:              "UNIV_MSS_STAGETOCACHE_ERR",
	UNIV_MSS_UNLINK_ERR:                    "UNIV_MSS_UNLINK_ERR",
	UNIV_MSS_MKDIR_ERR:                     "UNIV_MSS_MKDIR_ERR",
	UNIV_MSS_CHMOD_ERR:                     "UNIV_MSS_CHMOD_ERR",
	UNIV_MSS_STAT_ERR:                      "UNIV_MSS_STAT_ERR",
	UNIV_MSS_RENAME_ERR:                    "UNIV_MSS_RENAME_ERR",
	HPSS_AUTH_NOT_SUPPORTED:                "HPSS_AUTH_NOT_SUPPORTED",
	HPSS_FILE_OPEN_ERR:                     "HPSS_FILE_OPEN_ERR",
	HPSS_FILE_CREATE_ERR:                   "HPSS_FILE_CREATE_ERR",
	HPSS_FILE_READ_ERR:                     "HPSS_FILE_READ_ERR",
	HPSS_FILE_WRITE_ERR:                    "HPSS_FILE_WRITE_ERR",
	HPSS_FILE_CLOSE_ERR:                    "HPSS_FILE_CLOSE_ERR",
	HPSS_FILE_UNLINK_ERR:                   "HPSS_FILE_UNLINK_ERR",
	HPSS_FILE_STAT_ERR:                     "HPSS_FILE_STAT_ERR",
	HPSS_FILE_FSTAT_ERR:                    "HPSS_FILE_FSTAT_ERR",
	HPSS_FILE_LSEEK_ERR:                    "HPSS_FILE_LSEEK_ERR",
	HPSS_FILE_FSYNC_ERR:                    "HPSS_FILE_FSYNC_ERR",
	HPSS_FILE_MKDIR_ERR:                    "HPSS_FILE_MKDIR_ERR",
	HPSS_FILE_RMDIR_ERR:                    "HPSS_FILE_RMDIR_ERR",
	HPSS_FILE_OPENDIR_ERR:                  "HPSS_FILE_OPENDIR_ERR",
	HPSS_FILE_CLOSEDIR_ERR:                 "HPSS_FILE_CLOSEDIR_ERR",
	HPSS_FILE_READDIR_ERR:                  "HPSS_FILE_READDIR_ERR",
	HPSS_FILE_STAGE_ERR:                    "HPSS_FILE_STAGE_ERR",
	HPSS_FILE_GET_FS_FREESPACE_ERR:         "HPSS_FILE_GET_FS_FREESPACE_ERR",
	HPSS_FILE_CHMOD_ERR:                    "HPSS_FILE_CHMOD_ERR",
	HPSS_FILE_RENAME_ERR:                   "HPSS_FILE_RENAME_ERR",
	HPSS_FILE_TRUNCATE_ERR:                 "HPSS_FILE_TRUNCATE_ERR",
	HPSS_FILE_LINK_ERR:                     "HPSS_FILE_LINK_ERR",
	HPSS_AUTH_ERR:                          "HPSS_AUTH_ERR",
	HPSS_WRITE_LIST_ERR:                    "HPSS_WRITE_LIST_ERR",
	HPSS_READ_LIST_ERR:                     "HPSS_READ_LIST_ERR",
	HPSS_TRANSFER_ERR:                      "HPSS_TRANSFER_ERR",
	HPSS_MOVER_PROT_ERR:                    "HPSS_MOVER_PROT_ERR",
	S3_INIT_ERROR:                          "S3_INIT_ERROR",
	S3_PUT_ERROR:                           "S3_PUT_ERROR",
	S3_GET_ERROR:                           "S3_GET_ERROR",
	S3_FILE_UNLINK_ERR:                     "S3_FILE_UNLINK_ERR",
	S3_FILE_STAT_ERR:                       "S3_FILE_STAT_ERR",
	S3_FILE_COPY_ERR:                       "S3_FILE_COPY_ERR",
	S3_FILE_OPEN_ERR:                       "S3_FILE_OPEN_ERR",
	S3_FILE_SEEK_ERR:                       "S3_FILE_SEEK_ERR",
	S3_FILE_RENAME_ERR:                     "S3_FILE_RENAME_ERR",
	REPLICA_IS_BEING_STAGED:                "REPLICA_IS_BEING_STAGED",
	REPLICA_STAGING_FAILED:                 "REPLICA_STAGING_FAILED",
	WOS_PUT_ERR:                            "WOS_PUT_ERR",
	WOS_STREAM_PUT_ERR:                     "WOS_STREAM_PUT_ERR",
	WOS_STREAM_CLOSE_ERR:                   "WOS_STREAM_CLOSE_ERR",
	WOS_GET_ERR:                            "WOS_GET_ERR",
	WOS_STREAM_GET_ERR:                     "WOS_STREAM_GET_ERR",
	WOS_UNLINK_ERR:                         "WOS_UNLINK_ERR",
	WOS_STAT_ERR:                           "WOS_STAT_ERR",
	WOS_CONNECT_ERR:                        "WOS_CONNECT_ERR",
	HDFS_FILE_OPEN_ERR:                     "HDFS_FILE_OPEN_ERR",
	HDFS_FILE_CREATE_ERR:                   "HDFS_FILE_CREATE_ERR",
	HDFS_FILE_READ_ERR:                     "HDFS_FILE_READ_ERR",
	HDFS_FILE_WRITE_ERR:                    "HDFS_FILE_WRITE_ERR",
	HDFS_FILE_CLOSE_ERR:                    "HDFS_FILE_CLOSE_ERR",
	HDFS_FILE_UNLINK_ERR:                   "HDFS_FILE_UNLINK_ERR",
	HDFS_FILE_STAT_ERR:                     "HDFS_FILE_STAT_ERR",
	HDFS_FILE_FSTAT_ERR:                    "HDFS_FILE_FSTAT_ERR",
	HDFS_FILE_LSEEK_ERR:                    "HDFS_FILE_LSEEK_ERR",
	HDFS_FILE_FSYNC_ERR:                    "HDFS_FILE_FSYNC_ERR",
	HDFS_FILE_MKDIR_ERR:                    "HDFS_FILE_MKDIR_ERR",
	HDFS_FILE_RMDIR_ERR:                    "HDFS_FILE_RMDIR_ERR",
	HDFS_FILE_OPENDIR_ERR:                  "HDFS_FILE_OPENDIR_ERR",
	HDFS_FILE_CLOSEDIR_ERR:                 "HDFS_FILE_CLOSEDIR_ERR",
	HDFS_FILE_READDIR_ERR:                  "HDFS_FILE_READDIR_ERR",
	HDFS_FILE_STAGE_ERR:                    "HDFS_FILE_STAGE_ERR",
	HDFS_FILE_GET_FS_FREESPACE_ERR:         "HDFS_FILE_GET_FS_FREESPACE_ERR",
	HDFS_FILE_CHMOD_ERR:                    "HDFS_FILE_CHMOD_ERR",
	HDFS_FILE_RENAME_ERR:                   "HDFS_FILE_RENAME_ERR",
	HDFS_FILE_TRUNCATE_ERR:                 "HDFS_FILE_TRUNCATE_ERR",
	HDFS_FILE_LINK_ERR:                     "HDFS_FILE_LINK_ERR",
	HDFS_FILE_OPR_TIMEOUT_ERR:              "HDFS_FILE_OPR_TIMEOUT_ERR",
	DIRECT_ACCESS_FILE_USER_INVALID_ERR:    "DIRECT_ACCESS_FILE_USER_INVALID_ERR",
	CATALOG_NOT_CONNECTED:                  "CATALOG_NOT_CONNECTED",
	CAT_ENV_ERR:                            "CAT_ENV_ERR",
	CAT_CONNECT_ERR:                        "CAT_CONNECT_ERR",
	CAT_DISCONNECT_ERR:                     "CAT_DISCONNECT_ERR",
	CAT_CLOSE_ENV_ERR:                      "CAT_CLOSE_ENV_ERR",
	CAT_SQL_ERR:                            "CAT_SQL_ERR",
	CAT_GET_ROW_ERR:                        "CAT_GET_ROW_ERR",
	CAT_NO_ROWS_FOUND:                      "CAT_NO_ROWS_FOUND",
	CATALOG_ALREADY_HAS_ITEM_BY_THAT_NAME:  "CATALOG_ALREADY_HAS_ITEM_BY_THAT_NAME",
	CAT_INVALID_RESOURCE_TYPE:              "CAT_INVALID_RESOURCE_TYPE",
	CAT_INVALID_RESOURCE_CLASS:             "CAT_INVALID_RESOURCE_CLASS",
	CAT_INVALID_RESOURCE_NET_ADDR:          "CAT_INVALID_RESOURCE_NET_ADDR",
	CAT_INVALID_RESOURCE_VAULT_PATH:        "CAT_INVALID_RESOURCE_VAULT_PATH",
	CAT_UNKNOWN_COLLECTION:                 "CAT_UNKNOWN_COLLECTION",
	CAT_INVALID_DATA_TYPE:                  "CAT_INVALID_DATA_TYPE",
	CAT_INVALID_ARGUMENT:                   "CAT_INVALID_ARGUMENT",
	CAT_UNKNOWN_FILE:                       "CAT_UNKNOWN_FILE",
	CAT_NO_ACCESS_PERMISSION:               "CAT_NO_ACCESS_PERMISSION",
	CAT_SUCCESS_BUT_WITH_NO_INFO:           "CAT_SUCCESS_BUT_WITH_NO_INFO",
	CAT_INVALID_USER_TYPE:                  "CAT_INVALID_USER_TYPE",
	CAT_COLLECTION_NOT_EMPTY:               "CAT_COLLECTION_NOT_EMPTY",
	CAT_TOO_MANY_TABLES:                    "CAT_TOO_MANY_TABLES",
	CAT_UNKNOWN_TABLE:                      "CAT_UNKNOWN_TABLE",
	CAT_NOT_OPEN:                           "CAT_NOT_OPEN",
	CAT_FAILED_TO_LINK_TABLES:              "CAT_FAILED_TO_LINK_TABLES",
	CAT_INVALID_AUTHENTICATION:             "CAT_INVALID_AUTHENTICATION",
	CAT_INVALID_USER:                       "CAT_INVALID_USER",
	CAT_INVALID_ZONE:                       "CAT_INVALID_ZONE",
	CAT_INVALID_GROUP:                      "CAT_INVALID_GROUP",
	CAT_INSUFFICIENT_PRIVILEGE_LEVEL:       "CAT_INSUFFICIENT_PRIVILEGE_LEVEL",
	CAT_INVALID_RESOURCE:                   "CAT_INVALID_RESOURCE",
	CAT_INVALID_CLIENT_USER:                "CAT_INVALID_CLIENT_USER",
	CAT_NAME_EXISTS_AS_COLLECTION:          "CAT_NAME_EXISTS_AS_COLLECTION",
	CAT_NAME_EXISTS_AS_DATAOBJ:             "CAT_NAME_EXISTS_AS_DATAOBJ",
	CAT_RESOURCE_NOT_EMPTY:                 "CAT_RESOURCE_NOT_EMPTY",
	CAT_NOT_A_DATAOBJ_AND_NOT_A_COLLECTION: "CAT_NOT_A_DATAOBJ_AND_NOT_A_COLLECTION",
	CAT_RECURSIVE_MOVE:                     "CAT_RECURSIVE_MOVE",
	CAT_LAST_REPLICA:                       "CAT_LAST_REPLICA",
	CAT_OCI_ERROR:                          "CAT_OCI_ERROR",
	CAT_PASSWORD_EXPIRED:                   "CAT_PASSWORD_EXPIRED",
	CAT_PASSWORD_ENCODING_ERROR:            "CAT_PASSWORD_ENCODING_ERROR",
	CAT_TABLE_ACCESS_DENIED:                "CAT_TABLE_ACCESS_DENIED",
	CAT_UNKNOWN_RESOURCE:                   "CAT_UNKNOWN_RESOURCE",
	CAT_UNKNOWN_SPECIFIC_QUERY:             "CAT_UNKNOWN_SPECIFIC_QUERY",
	CAT_PSEUDO_RESC_MODIFY_DISALLOWED:      "CAT_PSEUDO_RESC_MODIFY_DISALLOWED",
	CAT_HOSTNAME_INVALID:                   "CAT_HOSTNAME_INVALID",
	CAT_BIND_VARIABLE_LIMIT_EXCEEDED:       "CAT_BIND_VARIABLE_LIMIT_EXCEEDED",
	CAT_INVALID_CHILD:                      "CAT_INVALID_CHILD",
	CAT_INVALID_OBJ_COUNT:                  "CAT_INVALID_OBJ_COUNT",
	CAT_INVALID_RESOURCE_NAME:              "CAT_INVALID_RESOURCE_NAME",
	CAT_STATEMENT_TABLE_FULL:               "CAT_STATEMENT_TABLE_FULL",
	CAT_RESOURCE_NAME_LENGTH_EXCEEDED:      "CAT_RESOURCE_NAME_LENGTH_EXCEEDED",
	CAT_NO_CHECKSUM_FOR_REPLICA:            "CAT_NO_CHECKSUM_FOR_REPLICA",
	CAT_TICKET_INVALID:                     "CAT_TICKET_INVALID",
	CAT_TICKET_EXPIRED:                     "CAT_TICKET_EXPIRED",
	CAT_TICKET_USES_EXCEEDED:               "CAT_TICKET_USES_EXCEEDED",
	CAT_TICKET_USER_EXCLUDED:               "CAT_TICKET_USER_EXCLUDED",
	CAT_TICKET_HOST_EXCLUDED:               "CAT_TICKET_HOST_EXCLUDED",
	CAT_TICKET_GROUP_EXCLUDED:              "CAT_TICKET_GROUP_EXCLUDED",
	CAT_TICKET_WRITE_USES_EXCEEDED:         "CAT_TICKET_WRITE_USES_EXCEEDED",
	CAT_TICKET_WRITE_BYTES_EXCEEDED:        "CAT_TICKET_WRITE_BYTES_EXCEEDED",
	FILE_OPEN_ERR:                          "FILE_OPEN_ERR",
	FILE_READ_ERR:                          "FILE_READ_ERR",
	FILE_WRITE_ERR:                         "FILE_WRITE_ERR",
	PASSWORD_EXCEEDS_MAX_SIZE:              "PASSWORD_EXCEEDS_MAX_SIZE",
	ENVIRONMENT_VAR_HOME_NOT_DEFINED:       "ENVIRONMENT_VAR_HOME_NOT_DEFINED",
	UNABLE_TO_STAT_FILE:                    "UNABLE_TO_STAT_FILE",
	AUTH_FILE_NOT_ENCRYPTED:                "AUTH_FILE_NOT_ENCRYPTED",
	AUTH_FILE_DOES_NOT_EXIST:               "AUTH_FILE_DOES_NOT_EXIST",
	UNLINK_FAILED:                          "UNLINK_FAILED",
	NO_PASSWORD_ENTERED:                    "NO_PASSWORD_ENTERED",
	REMOTE_SERVER_AUTHENTICATION_FAILURE:   "REMOTE_SERVER_AUTHENTICATION_FAILURE",
	REMOTE_SERVER_AUTH_NOT_PROVIDED:        "REMOTE_SERVER_AUTH_NOT_PROVIDED",
	REMOTE_SERVER_AUTH_EMPTY:               "REMOTE_SERVER_AUTH_EMPTY",
	REMOTE_SERVER_SID_NOT_DEFINED:          "REMOTE_SERVER_SID_NOT_DEFINED",
	GSI_NOT_COMPILED_IN:                    "GSI_NOT_COMPILED_IN",
	GSI_NOT_BUILT_INTO_CLIENT:              "GSI_NOT_BUILT_INTO_CLIENT",
	GSI_NOT_BUILT_INTO_SERVER:              "GSI_NOT_BUILT_INTO_SERVER",
	GSI_ERROR_IMPORT_NAME:                  "GSI_ERROR_IMPORT_NAME",
	GSI_ERROR_INIT_SECURITY_CONTEXT:        "GSI_ERROR_INIT_SECURITY_CONTEXT",
	GSI_ERROR_SENDING_TOKEN_LENGTH:         "GSI_ERROR_SENDING_TOKEN_LENGTH",
	GSI_ERROR_READING_TOKEN_LENGTH:         "GSI_ERROR_READING_TOKEN_LENGTH",
	GSI_ERROR_TOKEN_TOO_LARGE:              "GSI_ERROR_TOKEN_TOO_LARGE",
	GSI_ERROR_BAD_TOKEN_RCVED:              "GSI_ERROR_BAD_TOKEN_RCVED",
	GSI_SOCKET_READ_ERROR:                  "GSI_SOCKET_READ_ERROR",
	GSI_PARTIAL_TOKEN_READ:                 "GSI_PARTIAL_TOKEN_READ",
	GSI_SOCKET_WRITE_ERROR:                 "GSI_SOCKET_WRITE_ERROR",
	GSI_ERROR_FROM_GSI_LIBRARY:             "GSI_ERROR_FROM_GSI_LIBRARY",
	GSI_ERROR_IMPORTING_NAME:               "GSI_ERROR_IMPORTING_NAME",
	GSI_ERROR_ACQUIRING_CREDS:              "GSI_ERROR_ACQUIRING_CREDS",
	GSI_ACCEPT_SEC_CONTEXT_ERROR:           "GSI_ACCEPT_SEC_CONTEXT_ERROR",
	GSI_ERROR_DISPLAYING_NAME:              "GSI_ERROR_DISPLAYING_NAME",
	GSI_ERROR_RELEASING_NAME:               "GSI_ERROR_RELEASING_NAME",
	GSI_DN_DOES_NOT_MATCH_USER:             "GSI_DN_DOES_NOT_MATCH_USER",
	GSI_QUERY_INTERNAL_ERROR:               "GSI_QUERY_INTERNAL_ERROR",
	GSI_NO_MATCHING_DN_FOUND:               "GSI_NO_MATCHING_DN_FOUND",
	GSI_MULTIPLE_MATCHING_DN_FOUND:         "GSI_MULTIPLE_MATCHING_DN_FOUND",
	KRB_NOT_COMPILED_IN:                    "KRB_NOT_COMPILED_IN",
	KRB_NOT_BUILT_INTO_CLIENT:              "KRB_NOT_BUILT_INTO_CLIENT",
	KRB_NOT_BUILT_INTO_SERVER:              "KRB_NOT_BUILT_INTO_SERVER",
	KRB_ERROR_IMPORT_NAME:                  "KRB_ERROR_IMPORT_NAME",
	KRB_ERROR_INIT_SECURITY_CONTEXT:        "KRB_ERROR_INIT_SECURITY_CONTEXT",
	KRB_ERROR_SENDING_TOKEN_LENGTH:         "KRB_ERROR_SENDING_TOKEN_LENGTH",
	KRB_ERROR_READING_TOKEN_LENGTH:         "KRB_ERROR_READING_TOKEN_LENGTH",
	KRB_ERROR_TOKEN_TOO_LARGE:              "KRB_ERROR_TOKEN_TOO_LARGE",
	KRB_ERROR_BAD_TOKEN_RCVED:              "KRB_ERROR_BAD_TOKEN_RCVED",
	KRB_SOCKET_READ_ERROR:                  "KRB_SOCKET_READ_ERROR",
	KRB_PARTIAL_TOKEN_READ:                 "KRB_PARTIAL_TOKEN_READ",
	KRB_SOCKET_WRITE_ERROR:                 "KRB_SOCKET_WRITE_ERROR",
	KRB_ERROR_FROM_KRB_LIBRARY:             "KRB_ERROR_FROM_KRB_LIBRARY",
	KRB_ERROR_IMPORTING_NAME:               "KRB_ERROR_IMPORTING_NAME",
	KRB_ERROR_ACQUIRING_CREDS:              "KRB_ERROR_ACQUIRING_CREDS",
	KRB_ACCEPT_SEC_CONTEXT_ERROR:           "KRB_ACCEPT_SEC_CONTEXT_ERROR",
	KRB_ERROR_DISPLAYING_NAME:              "KRB_ERROR_DISPLAYING_NAME",
	KRB_ERROR_RELEASING_NAME:               "KRB_ERROR_RELEASING_NAME",
	KRB_USER_DN_NOT_FOUND:                  "KRB_USER_DN_NOT_FOUND",
	KRB_NAME_MATCHES_MULTIPLE_USERS:        "KRB_NAME_MATCHES_MULTIPLE_USERS",
	KRB_QUERY_INTERNAL_ERROR:               "KRB_QUERY_INTERNAL_ERROR",
	OSAUTH_NOT_BUILT_INTO_CLIENT:           "OSAUTH_NOT_BUILT_INTO_CLIENT",
	OSAUTH_NOT_BUILT_INTO_SERVER:           "OSAUTH_NOT_BUILT_INTO_SERVER",
	PAM_AUTH_NOT_BUILT_INTO_CLIENT:         "PAM_AUTH_NOT_BUILT_INTO_CLIENT",
	PAM_AUTH_NOT_BUILT_INTO_SERVER:         "PAM_AUTH_NOT_BUILT_INTO_SERVER",
	PAM_AUTH_PASSWORD_FAILED:               "PAM_AUTH_PASSWORD_FAILED",
	PAM_AUTH_PASSWORD_INVALID_TTL:          "PAM_AUTH_PASSWORD_INVALID_TTL",

	OBJPATH_EMPTY_IN_STRUCT_ERR:       "OBJPATH_EMPTY_IN_STRUCT_ERR",
	RESCNAME_EMPTY_IN_STRUCT_ERR:      "RESCNAME_EMPTY_IN_STRUCT_ERR",
	DATATYPE_EMPTY_IN_STRUCT_ERR:      "DATATYPE_EMPTY_IN_STRUCT_ERR",
	DATASIZE_EMPTY_IN_STRUCT_ERR:      "DATASIZE_EMPTY_IN_STRUCT_ERR",
	CHKSUM_EMPTY_IN_STRUCT_ERR:        "CHKSUM_EMPTY_IN_STRUCT_ERR",
	VERSION_EMPTY_IN_STRUCT_ERR:       "VERSION_EMPTY_IN_STRUCT_ERR",
	FILEPATH_EMPTY_IN_STRUCT_ERR:      "FILEPATH_EMPTY_IN_STRUCT_ERR",
	REPLNUM_EMPTY_IN_STRUCT_ERR:       "REPLNUM_EMPTY_IN_STRUCT_ERR",
	REPLSTATUS_EMPTY_IN_STRUCT_ERR:    "REPLSTATUS_EMPTY_IN_STRUCT_ERR",
	DATAOWNER_EMPTY_IN_STRUCT_ERR:     "DATAOWNER_EMPTY_IN_STRUCT_ERR",
	DATAOWNERZONE_EMPTY_IN_STRUCT_ERR: "DATAOWNERZONE_EMPTY_IN_STRUCT_ERR",
	DATAEXPIRY_EMPTY_IN_STRUCT_ERR:    "DATAEXPIRY_EMPTY_IN_STRUCT_ERR",
	DATACOMMENTS_EMPTY_IN_STRUCT_ERR:  "DATACOMMENTS_EMPTY_IN_STRUCT_ERR",
	DATACREATE_EMPTY_IN_STRUCT_ERR:    "DATACREATE_EMPTY_IN_STRUCT_ERR",
	DATAMODIFY_EMPTY_IN_STRUCT_ERR:    "DATAMODIFY_EMPTY_IN_STRUCT_ERR",
	DATAACCESS_EMPTY_IN_STRUCT_ERR:    "DATAACCESS_EMPTY_IN_STRUCT_ERR",
	DATAACCESSINX_EMPTY_IN_STRUCT_ERR: "DATAACCESSINX_EMPTY_IN_STRUCT_ERR",
	NO_RULE_FOUND_ERR:                 "NO_RULE_FOUND_ERR",
	NO_MORE_RULES_ERR:                 "NO_MORE_RULES_ERR",
	UNMATCHED_ACTION_ERR:              "UNMATCHED_ACTION_ERR",
	RULES_FILE_READ_ERROR:             "RULES_FILE_READ_ERROR",

	ACTION_ARG_COUNT_MISMATCH:                "ACTION_ARG_COUNT_MISMATCH",
	MAX_NUM_OF_ARGS_IN_ACTION_EXCEEDED:       "MAX_NUM_OF_ARGS_IN_ACTION_EXCEEDED",
	UNKNOWN_PARAM_IN_RULE_ERR:                "UNKNOWN_PARAM_IN_RULE_ERR",
	DESTRESCNAME_EMPTY_IN_STRUCT_ERR:         "DESTRESCNAME_EMPTY_IN_STRUCT_ERR",
	BACKUPRESCNAME_EMPTY_IN_STRUCT_ERR:       "BACKUPRESCNAME_EMPTY_IN_STRUCT_ERR",
	DATAID_EMPTY_IN_STRUCT_ERR:               "DATAID_EMPTY_IN_STRUCT_ERR",
	COLLID_EMPTY_IN_STRUCT_ERR:               "COLLID_EMPTY_IN_STRUCT_ERR",
	RESCGROUPNAME_EMPTY_IN_STRUCT_ERR:        "RESCGROUPNAME_EMPTY_IN_STRUCT_ERR",
	STATUSSTRING_EMPTY_IN_STRUCT_ERR:         "STATUSSTRING_EMPTY_IN_STRUCT_ERR",
	DATAMAPID_EMPTY_IN_STRUCT_ERR:            "DATAMAPID_EMPTY_IN_STRUCT_ERR",
	USERNAMECLIENT_EMPTY_IN_STRUCT_ERR:       "USERNAMECLIENT_EMPTY_IN_STRUCT_ERR",
	RODSZONECLIENT_EMPTY_IN_STRUCT_ERR:       "RODSZONECLIENT_EMPTY_IN_STRUCT_ERR",
	USERTYPECLIENT_EMPTY_IN_STRUCT_ERR:       "USERTYPECLIENT_EMPTY_IN_STRUCT_ERR",
	HOSTCLIENT_EMPTY_IN_STRUCT_ERR:           "HOSTCLIENT_EMPTY_IN_STRUCT_ERR",
	AUTHSTRCLIENT_EMPTY_IN_STRUCT_ERR:        "AUTHSTRCLIENT_EMPTY_IN_STRUCT_ERR",
	USERAUTHSCHEMECLIENT_EMPTY_IN_STRUCT_ERR: "USERAUTHSCHEMECLIENT_EMPTY_IN_STRUCT_ERR",
	USERINFOCLIENT_EMPTY_IN_STRUCT_ERR:       "USERINFOCLIENT_EMPTY_IN_STRUCT_ERR",
	USERCOMMENTCLIENT_EMPTY_IN_STRUCT_ERR:    "USERCOMMENTCLIENT_EMPTY_IN_STRUCT_ERR",
	USERCREATECLIENT_EMPTY_IN_STRUCT_ERR:     "USERCREATECLIENT_EMPTY_IN_STRUCT_ERR",
	USERMODIFYCLIENT_EMPTY_IN_STRUCT_ERR:     "USERMODIFYCLIENT_EMPTY_IN_STRUCT_ERR",
	USERNAMEPROXY_EMPTY_IN_STRUCT_ERR:        "USERNAMEPROXY_EMPTY_IN_STRUCT_ERR",
	RODSZONEPROXY_EMPTY_IN_STRUCT_ERR:        "RODSZONEPROXY_EMPTY_IN_STRUCT_ERR",
	USERTYPEPROXY_EMPTY_IN_STRUCT_ERR:        "USERTYPEPROXY_EMPTY_IN_STRUCT_ERR",
	HOSTPROXY_EMPTY_IN_STRUCT_ERR:            "HOSTPROXY_EMPTY_IN_STRUCT_ERR",
	AUTHSTRPROXY_EMPTY_IN_STRUCT_ERR:         "AUTHSTRPROXY_EMPTY_IN_STRUCT_ERR",
	USERAUTHSCHEMEPROXY_EMPTY_IN_STRUCT_ERR:  "USERAUTHSCHEMEPROXY_EMPTY_IN_STRUCT_ERR",
	USERINFOPROXY_EMPTY_IN_STRUCT_ERR:        "USERINFOPROXY_EMPTY_IN_STRUCT_ERR",
	USERCOMMENTPROXY_EMPTY_IN_STRUCT_ERR:     "USERCOMMENTPROXY_EMPTY_IN_STRUCT_ERR",
	USERCREATEPROXY_EMPTY_IN_STRUCT_ERR:      "USERCREATEPROXY_EMPTY_IN_STRUCT_ERR",
	USERMODIFYPROXY_EMPTY_IN_STRUCT_ERR:      "USERMODIFYPROXY_EMPTY_IN_STRUCT_ERR",
	COLLNAME_EMPTY_IN_STRUCT_ERR:             "COLLNAME_EMPTY_IN_STRUCT_ERR",
	COLLPARENTNAME_EMPTY_IN_STRUCT_ERR:       "COLLPARENTNAME_EMPTY_IN_STRUCT_ERR",
	COLLOWNERNAME_EMPTY_IN_STRUCT_ERR:        "COLLOWNERNAME_EMPTY_IN_STRUCT_ERR",
	COLLOWNERZONE_EMPTY_IN_STRUCT_ERR:        "COLLOWNERZONE_EMPTY_IN_STRUCT_ERR",
	COLLEXPIRY_EMPTY_IN_STRUCT_ERR:           "COLLEXPIRY_EMPTY_IN_STRUCT_ERR",
	COLLCOMMENTS_EMPTY_IN_STRUCT_ERR:         "COLLCOMMENTS_EMPTY_IN_STRUCT_ERR",
	COLLCREATE_EMPTY_IN_STRUCT_ERR:           "COLLCREATE_EMPTY_IN_STRUCT_ERR",
	COLLMODIFY_EMPTY_IN_STRUCT_ERR:           "COLLMODIFY_EMPTY_IN_STRUCT_ERR",
	COLLACCESS_EMPTY_IN_STRUCT_ERR:           "COLLACCESS_EMPTY_IN_STRUCT_ERR",
	COLLACCESSINX_EMPTY_IN_STRUCT_ERR:        "COLLACCESSINX_EMPTY_IN_STRUCT_ERR",
	COLLMAPID_EMPTY_IN_STRUCT_ERR:            "COLLMAPID_EMPTY_IN_STRUCT_ERR",
	COLLINHERITANCE_EMPTY_IN_STRUCT_ERR:      "COLLINHERITANCE_EMPTY_IN_STRUCT_ERR",
	RESCZONE_EMPTY_IN_STRUCT_ERR:             "RESCZONE_EMPTY_IN_STRUCT_ERR",
	RESCLOC_EMPTY_IN_STRUCT_ERR:              "RESCLOC_EMPTY_IN_STRUCT_ERR",
	RESCTYPE_EMPTY_IN_STRUCT_ERR:             "RESCTYPE_EMPTY_IN_STRUCT_ERR",
	RESCTYPEINX_EMPTY_IN_STRUCT_ERR:          "RESCTYPEINX_EMPTY_IN_STRUCT_ERR",
	RESCCLASS_EMPTY_IN_STRUCT_ERR:            "RESCCLASS_EMPTY_IN_STRUCT_ERR",
	RESCCLASSINX_EMPTY_IN_STRUCT_ERR:         "RESCCLASSINX_EMPTY_IN_STRUCT_ERR",
	RESCVAULTPATH_EMPTY_IN_STRUCT_ERR:        "RESCVAULTPATH_EMPTY_IN_STRUCT_ERR",
	NUMOPEN_ORTS_EMPTY_IN_STRUCT_ERR:         "NUMOPEN_ORTS_EMPTY_IN_STRUCT_ERR",
	PARAOPR_EMPTY_IN_STRUCT_ERR:              "PARAOPR_EMPTY_IN_STRUCT_ERR",
	RESCID_EMPTY_IN_STRUCT_ERR:               "RESCID_EMPTY_IN_STRUCT_ERR",
	GATEWAYADDR_EMPTY_IN_STRUCT_ERR:          "GATEWAYADDR_EMPTY_IN_STRUCT_ERR",
	RESCMAX_BJSIZE_EMPTY_IN_STRUCT_ERR:       "RESCMAX_BJSIZE_EMPTY_IN_STRUCT_ERR",
	FREESPACE_EMPTY_IN_STRUCT_ERR:            "FREESPACE_EMPTY_IN_STRUCT_ERR",
	FREESPACETIME_EMPTY_IN_STRUCT_ERR:        "FREESPACETIME_EMPTY_IN_STRUCT_ERR",
	FREESPACETIMESTAMP_EMPTY_IN_STRUCT_ERR:   "FREESPACETIMESTAMP_EMPTY_IN_STRUCT_ERR",
	RESCINFO_EMPTY_IN_STRUCT_ERR:             "RESCINFO_EMPTY_IN_STRUCT_ERR",
	RESCCOMMENTS_EMPTY_IN_STRUCT_ERR:         "RESCCOMMENTS_EMPTY_IN_STRUCT_ERR",
	RESCCREATE_EMPTY_IN_STRUCT_ERR:           "RESCCREATE_EMPTY_IN_STRUCT_ERR",
	RESCMODIFY_EMPTY_IN_STRUCT_ERR:           "RESCMODIFY_EMPTY_IN_STRUCT_ERR",
	INPUT_ARG_NOT_WELL_FORMED_ERR:            "INPUT_ARG_NOT_WELL_FORMED_ERR",
	INPUT_ARG_OUT_OF_ARGC_RANGE_ERR:          "INPUT_ARG_OUT_OF_ARGC_RANGE_ERR",
	INSUFFICIENT_INPUT_ARG_ERR:               "INSUFFICIENT_INPUT_ARG_ERR",
	INPUT_ARG_DOES_NOT_MATCH_ERR:             "INPUT_ARG_DOES_NOT_MATCH_ERR",
	RETRY_WITHOUT_RECOVERY_ERR:               "RETRY_WITHOUT_RECOVERY_ERR",
	CUT_ACTION_PROCESSED_ERR:                 "CUT_ACTION_PROCESSED_ERR",
	ACTION_FAILED_ERR:                        "ACTION_FAILED_ERR",
	FAIL_ACTION_ENCOUNTERED_ERR:              "FAIL_ACTION_ENCOUNTERED_ERR",
	VARIABLE_NAME_TOO_LONG_ERR:               "VARIABLE_NAME_TOO_LONG_ERR",
	UNKNOWN_VARIABLE_MAP_ERR:                 "UNKNOWN_VARIABLE_MAP_ERR",
	UNDEFINED_VARIABLE_MAP_ERR:               "UNDEFINED_VARIABLE_MAP_ERR",
	NULL_VALUE_ERR:                           "NULL_VALUE_ERR",
	DVARMAP_FILE_READ_ERROR:                  "DVARMAP_FILE_READ_ERROR",
	NO_RULE_OR_MSI_FUNCTION_FOUND_ERR:        "NO_RULE_OR_MSI_FUNCTION_FOUND_ERR",
	FILE_CREATE_ERROR:                        "FILE_CREATE_ERROR",
	FMAP_FILE_READ_ERROR:                     "FMAP_FILE_READ_ERROR",
	DATE_FORMAT_ERR:                          "DATE_FORMAT_ERR",
	RULE_FAILED_ERR:                          "RULE_FAILED_ERR",
	NO_MICROSERVICE_FOUND_ERR:                "NO_MICROSERVICE_FOUND_ERR",
	INVALID_REGEXP:                           "INVALID_REGEXP",
	INVALID_OBJECT_NAME:                      "INVALID_OBJECT_NAME",
	INVALID_OBJECT_TYPE:                      "INVALID_OBJECT_TYPE",
	NO_VALUES_FOUND:                          "NO_VALUES_FOUND",
	NO_COLUMN_NAME_FOUND:                     "NO_COLUMN_NAME_FOUND",
	BREAK_ACTION_ENCOUNTERED_ERR:             "BREAK_ACTION_ENCOUNTERED_ERR",
	CUT_ACTION_ON_SUCCESS_PROCESSED_ERR:      "CUT_ACTION_ON_SUCCESS_PROCESSED_ERR",
	MSI_OPERATION_NOT_ALLOWED:                "MSI_OPERATION_NOT_ALLOWED",
	MAX_NUM_OF_ACTION_IN_RULE_EXCEEDED:       "MAX_NUM_OF_ACTION_IN_RULE_EXCEEDED",
	MSRVC_FILE_READ_ERROR:                    "MSRVC_FILE_READ_ERROR",
	MSRVC_VERSION_MISMATCH:                   "MSRVC_VERSION_MISMATCH",
	MICRO_SERVICE_OBJECT_TYPE_UNDEFINED:      "MICRO_SERVICE_OBJECT_TYPE_UNDEFINED",
	MSO_OBJ_GET_FAILED:                       "MSO_OBJ_GET_FAILED",
	REMOTE_IRODS_CONNECT_ERR:                 "REMOTE_IRODS_CONNECT_ERR",
	REMOTE_SRB_CONNECT_ERR:                   "REMOTE_SRB_CONNECT_ERR",
	MSO_OBJ_PUT_FAILED:                       "MSO_OBJ_PUT_FAILED",
	RE_PARSER_ERROR:                          "RE_PARSER_ERROR",
	RE_UNPARSED_SUFFIX:                       "RE_UNPARSED_SUFFIX",
	RE_POINTER_ERROR:                         "RE_POINTER_ERROR",
	RE_RUNTIME_ERROR:                         "RE_RUNTIME_ERROR",
	RE_DIVISION_BY_ZERO:                      "RE_DIVISION_BY_ZERO",
	RE_BUFFER_OVERFLOW:                       "RE_BUFFER_OVERFLOW",
	RE_UNSUPPORTED_OP_OR_TYPE:                "RE_UNSUPPORTED_OP_OR_TYPE",
	RE_UNSUPPORTED_SESSION_VAR:               "RE_UNSUPPORTED_SESSION_VAR",
	RE_UNABLE_TO_WRITE_LOCAL_VAR:             "RE_UNABLE_TO_WRITE_LOCAL_VAR",
	RE_UNABLE_TO_READ_LOCAL_VAR:              "RE_UNABLE_TO_READ_LOCAL_VAR",
	RE_UNABLE_TO_WRITE_SESSION_VAR:           "RE_UNABLE_TO_WRITE_SESSION_VAR",
	RE_UNABLE_TO_READ_SESSION_VAR:            "RE_UNABLE_TO_READ_SESSION_VAR",
	RE_UNABLE_TO_WRITE_VAR:                   "RE_UNABLE_TO_WRITE_VAR",
	RE_UNABLE_TO_READ_VAR:                    "RE_UNABLE_TO_READ_VAR",
	RE_PATTERN_NOT_MATCHED:                   "RE_PATTERN_NOT_MATCHED",
	RE_STRING_OVERFLOW:                       "RE_STRING_OVERFLOW",
	RE_UNKNOWN_ERROR:                         "RE_UNKNOWN_ERROR",
	RE_OUT_OF_MEMORY:                         "RE_OUT_OF_MEMORY",
	RE_SHM_UNLINK_ERROR:                      "RE_SHM_UNLINK_ERROR",
	RE_FILE_STAT_ERROR:                       "RE_FILE_STAT_ERROR",
	RE_UNSUPPORTED_AST_NODE_TYPE:             "RE_UNSUPPORTED_AST_NODE_TYPE",
	RE_UNSUPPORTED_SESSION_VAR_TYPE:          "RE_UNSUPPORTED_SESSION_VAR_TYPE",
	RE_TYPE_ERROR:                            "RE_TYPE_ERROR",
	RE_FUNCTION_REDEFINITION:                 "RE_FUNCTION_REDEFINITION",
	RE_DYNAMIC_TYPE_ERROR:                    "RE_DYNAMIC_TYPE_ERROR",
	RE_DYNAMIC_COERCION_ERROR:                "RE_DYNAMIC_COERCION_ERROR",
	RE_PACKING_ERROR:                         "RE_PACKING_ERROR",
	PHP_EXEC_SCRIPT_ERR:                      "PHP_EXEC_SCRIPT_ERR",
	PHP_REQUEST_STARTUP_ERR:                  "PHP_REQUEST_STARTUP_ERR",
	PHP_OPEN_SCRIPT_FILE_ERR:                 "PHP_OPEN_SCRIPT_FILE_ERR",
	KEY_NOT_FOUND:                            "KEY_NOT_FOUND",
	KEY_TYPE_MISMATCH:                        "KEY_TYPE_MISMATCH",
	CHILD_EXISTS:                             "CHILD_EXISTS",
	HIERARCHY_ERROR:                          "HIERARCHY_ERROR",
	CHILD_NOT_FOUND:                          "CHILD_NOT_FOUND",
	NO_NEXT_RESC_FOUND:                       "NO_NEXT_RESC_FOUND",
	NO_PDMO_DEFINED:                          "NO_PDMO_DEFINED",
	INVALID_LOCATION:                         "INVALID_LOCATION",
	PLUGIN_ERROR:                             "PLUGIN_ERROR",
	INVALID_RESC_CHILD_CONTEXT:               "INVALID_RESC_CHILD_CONTEXT",
	INVALID_FILE_OBJECT:                      "INVALID_FILE_OBJECT",
	INVALID_OPERATION:                        "INVALID_OPERATION",
	CHILD_HAS_PARENT:                         "CHILD_HAS_PARENT",
	FILE_NOT_IN_VAULT:                        "FILE_NOT_IN_VAULT",
	DIRECT_ARCHIVE_ACCESS:                    "DIRECT_ARCHIVE_ACCESS",
	ADVANCED_NEGOTIATION_NOT_SUPPORTED:       "ADVANCED_NEGOTIATION_NOT_SUPPORTED",
	DIRECT_CHILD_ACCESS:                      "DIRECT_CHILD_ACCESS",
	INVALID_DYNAMIC_CAST:                     "INVALID_DYNAMIC_CAST",
	INVALID_ACCESS_TO_IMPOSTOR_RESOURCE:      "INVALID_ACCESS_TO_IMPOSTOR_RESOURCE",
	INVALID_LEXICAL_CAST:                     "INVALID_LEXICAL_CAST",
	CONTROL_PLANE_MESSAGE_ERROR:              "CONTROL_PLANE_MESSAGE_ERROR",
	REPLICA_NOT_IN_RESC:                      "REPLICA_NOT_IN_RESC",
	INVALID_ANY_CAST:                         "INVALID_ANY_CAST",
	BAD_FUNCTION_CALL:                        "BAD_FUNCTION_CALL",
	CLIENT_NEGOTIATION_ERROR:                 "CLIENT_NEGOTIATION_ERROR",
	SERVER_NEGOTIATION_ERROR:                 "SERVER_NEGOTIATION_ERROR",
	INVALID_KVP_STRING:                       "INVALID_KVP_STRING",
	PLUGIN_ERROR_MISSING_SHARED_OBJECT:       "PLUGIN_ERROR_MISSING_SHARED_OBJECT",
	RULE_ENGINE_ERROR:                        "RULE_ENGINE_ERROR",
	REBALANCE_ALREADY_ACTIVE_ON_RESOURCE:     "REBALANCE_ALREADY_ACTIVE_ON_RESOURCE",
	NETCDF_OPEN_ERR:                          "NETCDF_OPEN_ERR",
	NETCDF_CREATE_ERR:                        "NETCDF_CREATE_ERR",
	NETCDF_CLOSE_ERR:                         "NETCDF_CLOSE_ERR",
	NETCDF_INVALID_PARAM_TYPE:                "NETCDF_INVALID_PARAM_TYPE",
	NETCDF_INQ_ID_ERR:                        "NETCDF_INQ_ID_ERR",
	NETCDF_GET_VARS_ERR:                      "NETCDF_GET_VARS_ERR",
	NETCDF_INVALID_DATA_TYPE:                 "NETCDF_INVALID_DATA_TYPE",
	NETCDF_INQ_VARS_ERR:                      "NETCDF_INQ_VARS_ERR",
	NETCDF_VARS_DATA_TOO_BIG:                 "NETCDF_VARS_DATA_TOO_BIG",
	NETCDF_DIM_MISMATCH_ERR:                  "NETCDF_DIM_MISMATCH_ERR",
	NETCDF_INQ_ERR:                           "NETCDF_INQ_ERR",
	NETCDF_INQ_FORMAT_ERR:                    "NETCDF_INQ_FORMAT_ERR",
	NETCDF_INQ_DIM_ERR:                       "NETCDF_INQ_DIM_ERR",
	NETCDF_INQ_ATT_ERR:                       "NETCDF_INQ_ATT_ERR",
	NETCDF_GET_ATT_ERR:                       "NETCDF_GET_ATT_ERR",
	NETCDF_VAR_COUNT_OUT_OF_RANGE:            "NETCDF_VAR_COUNT_OUT_OF_RANGE",
	NETCDF_UNMATCHED_NAME_ERR:                "NETCDF_UNMATCHED_NAME_ERR",
	NETCDF_NO_UNLIMITED_DIM:                  "NETCDF_NO_UNLIMITED_DIM",
	NETCDF_PUT_ATT_ERR:                       "NETCDF_PUT_ATT_ERR",
	NETCDF_DEF_DIM_ERR:                       "NETCDF_DEF_DIM_ERR",
	NETCDF_DEF_VAR_ERR:                       "NETCDF_DEF_VAR_ERR",
	NETCDF_PUT_VARS_ERR:                      "NETCDF_PUT_VARS_ERR",
	NETCDF_AGG_INFO_FILE_ERR:                 "NETCDF_AGG_INFO_FILE_ERR",
	NETCDF_AGG_ELE_INX_OUT_OF_RANGE:          "NETCDF_AGG_ELE_INX_OUT_OF_RANGE",
	NETCDF_AGG_ELE_FILE_NOT_OPENED:           "NETCDF_AGG_ELE_FILE_NOT_OPENED",
	NETCDF_AGG_ELE_FILE_NO_TIME_DIM:          "NETCDF_AGG_ELE_FILE_NO_TIME_DIM",
	SSL_NOT_BUILT_INTO_CLIENT:                "SSL_NOT_BUILT_INTO_CLIENT",
	SSL_NOT_BUILT_INTO_SERVER:                "SSL_NOT_BUILT_INTO_SERVER",
	SSL_INIT_ERROR:                           "SSL_INIT_ERROR",
	SSL_HANDSHAKE_ERROR:                      "SSL_HANDSHAKE_ERROR",
	SSL_SHUTDOWN_ERROR:                       "SSL_SHUTDOWN_ERROR",
	SSL_CERT_ERROR:                           "SSL_CERT_ERROR",
	OOI_CURL_EASY_INIT_ERR:                   "OOI_CURL_EASY_INIT_ERR",
	OOI_JSON_OBJ_SET_ERR:                     "OOI_JSON_OBJ_SET_ERR",
	OOI_DICT_TYPE_NOT_SUPPORTED:              "OOI_DICT_TYPE_NOT_SUPPORTED",
	OOI_JSON_PACK_ERR:                        "OOI_JSON_PACK_ERR",
	OOI_JSON_DUMP_ERR:                        "OOI_JSON_DUMP_ERR",
	OOI_CURL_EASY_PERFORM_ERR:                "OOI_CURL_EASY_PERFORM_ERR",
	OOI_JSON_LOAD_ERR:                        "OOI_JSON_LOAD_ERR",
	OOI_JSON_GET_ERR:                         "OOI_JSON_GET_ERR",
	OOI_JSON_NO_ANSWER_ERR:                   "OOI_JSON_NO_ANSWER_ERR",
	OOI_JSON_TYPE_ERR:                        "OOI_JSON_TYPE_ERR",
	OOI_JSON_INX_OUT_OF_RANGE:                "OOI_JSON_INX_OUT_OF_RANGE",
	OOI_REVID_NOT_FOUND:                      "OOI_REVID_NOT_FOUND",
	DEPRECATED_PARAMETER:                     "DEPRECATED_PARAMETER",
	XML_PARSING_ERR:                          "XML_PARSING_ERR",
	OUT_OF_URL_PATH:                          "OUT_OF_URL_PATH",
	URL_PATH_INX_OUT_OF_RANGE:                "URL_PATH_INX_OUT_OF_RANGE",
	SYS_NULL_INPUT:                           "SYS_NULL_INPUT",
	SYS_HANDLER_DONE_WITH_ERROR:              "SYS_HANDLER_DONE_WITH_ERROR",
	SYS_HANDLER_DONE_NO_ERROR:                "SYS_HANDLER_DONE_NO_ERROR",
	SYS_NO_HANDLER_REPLY_MSG:                 "SYS_NO_HANDLER_REPLY_MSG",
}

var NativeErrors = map[ErrorCode]error{
	CAT_NO_ROWS_FOUND:                     os.ErrNotExist,
	CAT_NO_ACCESS_PERMISSION:              os.ErrPermission,
	CAT_COLLECTION_NOT_EMPTY:              syscall.ENOTEMPTY,
	CATALOG_ALREADY_HAS_ITEM_BY_THAT_NAME: os.ErrExist,
	CAT_UNKNOWN_COLLECTION:                os.ErrNotExist,
	CAT_UNKNOWN_FILE:                      os.ErrNotExist,
	CAT_NAME_EXISTS_AS_COLLECTION:         os.ErrExist,
	CAT_NAME_EXISTS_AS_DATAOBJ:            os.ErrExist,
}
