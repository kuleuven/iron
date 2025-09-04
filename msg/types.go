package msg

import (
	"encoding/xml"
)

type StartupPack struct {
	XMLName         xml.Name `xml:"StartupPack_PI"`
	Protocol        Protocol `xml:"irodsProt"`
	ReconnectFlag   int      `xml:"reconnFlag"`
	ConnectionCount int      `xml:"connectCnt"`
	ProxyUser       string   `xml:"proxyUser"`
	ProxyRcatZone   string   `xml:"proxyRcatZone"`
	ClientUser      string   `xml:"clientUser"`
	ClientRcatZone  string   `xml:"clientRcatZone"`
	ReleaseVersion  string   `xml:"relVersion"`
	APIVersion      string   `xml:"apiVersion"`
	Option          string   `xml:"option"`
}

type ClientServerNegotiation struct {
	XMLName xml.Name `xml:"CS_NEG_PI"`
	Status  int      `xml:"status"`
	Result  string   `xml:"result"`
}

type Version struct {
	XMLName        xml.Name `xml:"Version_PI"`
	Status         int      `xml:"status"`
	ReleaseVersion string   `xml:"relVersion"`
	APIVersion     string   `xml:"apiVersion"`
	ReconnectPort  int      `xml:"reconnPort"`
	ReconnectAddr  string   `xml:"reconnAddr"`
	Cookie         int      `xml:"cookie"`
}

type SSLSharedSecret []byte

type PamAuthRequest struct {
	XMLName  xml.Name `xml:"pamAuthRequestInp_PI"`
	Username string   `xml:"pamUser"`
	Password string   `xml:"pamPassword"`
	TTL      int      `xml:"timeToLive"`
}

type PamAuthResponse struct {
	XMLName           xml.Name `xml:"pamAuthRequestOut_PI"`
	GeneratedPassword string   `xml:"irodsPamPassword"`
}

type AuthRequest []byte // Empty

type AuthChallenge struct {
	XMLName   xml.Name `xml:"authRequestOut_PI"`
	Challenge string   `xml:"challenge" native:"base64,64"`
}

type AuthChallengeResponse struct {
	XMLName  xml.Name `xml:"authResponseInp_PI"`
	Response string   `xml:"response" native:"base64,16"`
	Username string   `xml:"username"`
}

type AuthResponse []byte // Empty

type QueryRequest struct {
	XMLName           xml.Name `xml:"GenQueryInp_PI"`
	MaxRows           int      `xml:"maxRows"`
	ContinueIndex     int      `xml:"continueInx"`       // 1 for continuing, 0 for end
	PartialStartIndex int      `xml:"partialStartIndex"` // unknown
	Options           int      `xml:"options"`
	KeyVals           SSKeyVal `xml:"KeyValPair_PI"`
	Selects           IIKeyVal `xml:"InxIvalPair_PI"`
	Conditions        ISKeyVal `xml:"InxValPair_PI"`
}

type SSKeyVal struct {
	XMLName xml.Name  `xml:"KeyValPair_PI"`
	Length  int       `xml:"ssLen"`
	Keys    []KeyWord `xml:"keyWord" sizeField:"ssLen"`
	Values  []string  `xml:"svalue" sizeField:"ssLen"`
}

func (kv *SSKeyVal) Add(key KeyWord, val string) {
	kv.Keys = append(kv.Keys, key)
	kv.Values = append(kv.Values, val)
	kv.Length++
}

type IIKeyVal struct {
	XMLName xml.Name `xml:"InxIvalPair_PI"`
	Length  int      `xml:"iiLen"`
	Keys    []int    `xml:"inx" sizeField:"iiLen"`
	Values  []int    `xml:"ivalue" sizeField:"iiLen"`
}

func (kv *IIKeyVal) Add(key, val int) {
	kv.Keys = append(kv.Keys, key)
	kv.Values = append(kv.Values, val)
	kv.Length++
}

type ISKeyVal struct {
	XMLName xml.Name `xml:"InxValPair_PI"`
	Length  int      `xml:"isLen"`
	Keys    []int    `xml:"inx" sizeField:"isLen"`
	Values  []string `xml:"svalue" sizeField:"isLen"`
}

func (kv *ISKeyVal) Add(key int, val string) {
	kv.Keys = append(kv.Keys, key)
	kv.Values = append(kv.Values, val)
	kv.Length++
}

type QueryResponse struct {
	XMLName        xml.Name    `xml:"GenQueryOut_PI"`
	RowCount       int         `xml:"rowCnt"`
	AttributeCount int         `xml:"attriCnt"`
	ContinueIndex  int         `xml:"continueInx"`
	TotalRowCount  int         `xml:"totalRowCount"`
	SQLResult      []SQLResult `xml:"SqlResult_PI" sizeField:"attriCnt"`
}

type SQLResult struct {
	XMLName        xml.Name     `xml:"SqlResult_PI"`
	AttributeIndex ColumnNumber `xml:"attriInx"`
	ResultLen      int          `xml:"reslen"`
	Values         []string     `xml:"value,omitempty" sizeField:"reslen"`
}

type CreateCollectionRequest struct {
	XMLName       xml.Name `xml:"CollInpNew_PI"`
	Name          string   `xml:"collName"`
	Flags         int      `xml:"flags"`   // unused
	OperationType int      `xml:"oprType"` // unused
	KeyVals       SSKeyVal `xml:"KeyValPair_PI"`
}

type EmptyResponse []byte // Empty

type CollectionOperationStat struct {
	XMLName        xml.Name `xml:"CollOprStat_PI"`
	FileCount      int      `xml:"filesCnt"`
	TotalFileCount int      `xml:"totalFileCnt"`
	BytesWritten   int64    `xml:"bytesWritten"`
	LastObjectPath string   `xml:"lastObjPath"`
}

type DataObjectCopyRequest struct {
	XMLName xml.Name            `xml:"DataObjCopyInp_PI"`
	Paths   []DataObjectRequest `xml:"DataObjInp_PI" size:"2"`
}

type DataObjectRequest struct {
	XMLName                  xml.Name           `xml:"DataObjInp_PI"`
	Path                     string             `xml:"objPath"`
	CreateMode               int                `xml:"createMode"`
	OpenFlags                int                `xml:"openFlags"`
	Offset                   int64              `xml:"offset"`
	Size                     int64              `xml:"dataSize"`
	Threads                  int                `xml:"numThreads"`
	OperationType            OperationType      `xml:"oprType"`
	SpecialCollectionPointer *SpecialCollection `xml:"SpecColl_PI"`
	KeyVals                  SSKeyVal           `xml:"KeyValPair_PI"`
}

type SpecialCollection struct {
	XMLName           xml.Name `xml:"SpecColl_PI"`
	CollectionClass   int      `xml:"collClass"`
	Type              int      `xml:"type"`
	Collection        string   `xml:"collection"`
	ObjectPath        string   `xml:"objPath"`
	Resource          string   `xml:"resource"`
	ResourceHierarchy string   `xml:"rescHier"`
	PhysicalPath      string   `xml:"phyPath"`
	CacheDirectory    string   `xml:"cacheDir"`
	CacheDirty        int      `xml:"cacheDirty"`
	ReplicationNumber int      `xml:"replNum"`
}

type FileDescriptor int32

type OpenedDataObjectRequest struct {
	XMLName        xml.Name       `xml:"OpenedDataObjInp_PI"`
	FileDescriptor FileDescriptor `xml:"l1descInx"`
	Size           int            `xml:"len"`
	Whence         int            `xml:"whence"`
	OperationType  int            `xml:"oprType"`
	Offset         int64          `xml:"offset"`
	BytesWritten   int64          `xml:"bytesWritten"`
	KeyVals        SSKeyVal       `xml:"KeyValPair_PI"`
}

type SeekResponse struct {
	XMLName xml.Name `xml:"fileLseekOut_PI"`
	Offset  int64    `xml:"offset"`
}

type ReadResponse int32

type BinBytesBuf struct {
	XMLName xml.Name `xml:"BinBytesBuf_PI"`
	Length  int      `xml:"buflen"` // of original data
	Data    string   `xml:"buf"`    // data is base64 encoded
}

type GetDescriptorInfoRequest struct { // No xml.Name means this is a json struct
	FileDescriptor FileDescriptor `json:"fd"`
}

type GetDescriptorInfoResponse struct { // No xml.Name means this is a json struct
	L3DescriptorIndex       int                    `json:"l3descInx"`
	InUseFlag               bool                   `json:"in_use"`
	OperationType           int                    `json:"operation_type"`
	OpenType                int                    `json:"open_type"`
	OperationStatus         int                    `json:"operation_status"`
	ReplicationFlag         int                    `json:"data_object_input_replica_flag"`
	DataObjectInput         map[string]interface{} `json:"data_object_input"`
	DataObjectInfo          map[string]interface{} `json:"data_object_info"`
	OtherDataObjectInfo     map[string]interface{} `json:"other_data_object_info"`
	CopiesNeeded            int                    `json:"copies_needed"`
	BytesWritten            int64                  `json:"bytes_written"`
	DataSize                int64                  `json:"data_size"`
	ReplicaStatus           int                    `json:"replica_status"`
	ChecksumFlag            int                    `json:"checksum_flag"`
	SourceL1DescriptorIndex int                    `json:"source_l1_descriptor_index"`
	Checksum                string                 `json:"checksum"`
	RemoteL1DescriptorIndex int                    `json:"remote_l1_descriptor_index"`
	StageFlag               int                    `json:"stage_flag"`
	PurgeCacheFlag          int                    `json:"purge_cache_flag"`
	LockFileDescriptor      int                    `json:"lock_file_descriptor"`
	PluginData              map[string]interface{} `json:"plugin_data"`
	ReplicaDataObjectInfo   map[string]interface{} `json:"replication_data_object_info"`
	RemoteZoneHost          map[string]interface{} `json:"remote_zone_host"`
	InPDMO                  string                 `json:"in_pdmo"`
	ReplicaToken            string                 `json:"replica_token"`
}

type CloseDataObjectReplicaRequest struct { // No xml.Name means this is a json struct
	FileDescriptor            FileDescriptor `json:"fd"`
	SendNotification          bool           `json:"send_notification"`
	UpdateSize                bool           `json:"update_size"`
	UpdateStatus              bool           `json:"update_status"`
	ComputeChecksum           bool           `json:"compute_checksum"`
	PreserveReplicaStateTable bool           `json:"preserve_replica_state_table"`
}

type TouchDataObjectReplicaRequest struct { // No xml.Name means this is a json struct
	Path    string       `json:"logical_path"`
	Options TouchOptions `json:"options"`
}

type TouchOptions struct {
	SecondsSinceEpoch int64 `json:"seconds_since_epoch,omitempty"`
	ReplicaNumber     int   `json:"replica_number,omitempty"`
	NoCreate          bool  `json:"no_create,omitempty"`
}

type ModifyAccessRequest struct {
	XMLName       xml.Name `xml:"modAccessControlInp_PI"`
	RecursiveFlag int      `xml:"recursiveFlag"`
	AccessLevel   string   `xml:"accessLevel"`
	UserName      string   `xml:"userName"`
	Zone          string   `xml:"zone"`
	Path          string   `xml:"path"`
}

type ModifyMetadataRequest struct {
	XMLName      xml.Name `xml:"ModAVUMetadataInp_PI"`
	Operation    string   `xml:"arg0"` // add, adda, rm, rmw, rmi, cp, mod, set
	ItemType     string   `xml:"arg1"` // -d, -D, -c, -C, -r, -R, -u, -U
	ItemName     string   `xml:"arg2"`
	AttrName     string   `xml:"arg3"`
	AttrValue    string   `xml:"arg4"`
	AttrUnits    string   `xml:"arg5"`
	NewAttrName  string   `xml:"arg6"` // new attr name (for mod)
	NewAttrValue string   `xml:"arg7"` // new attr value (for mod)
	NewAttrUnits string   `xml:"arg8"` // new attr unit (for mod)
	Arg9         string   `xml:"arg9"` // unused
	KeyVals      SSKeyVal `xml:"KeyValPair_PI"`
}

type AtomicMetadataRequest struct { // No xml.Name means this is a json struct
	AdminMode  bool                `json:"admin_mode"`
	ItemName   string              `json:"entity_name"`
	ItemType   string              `json:"entity_type"` // d, C, R, u
	Operations []MetadataOperation `json:"operations"`
}

type MetadataOperation struct {
	Operation string `json:"operation"` // add, remove
	Name      string `json:"attribute"`
	Value     string `json:"value"`
	Units     string `json:"units,omitempty"`
}

type FileStatRequest struct {
	XMLName           xml.Name `xml:"fileStatInp_PI"`
	Host              HostAddr `xml:"RHostAddr_PI"`
	Path              string   `xml:"fileName"`
	ResourceHierarchy string   `xml:"rescHier"`
	ObjectPath        string   `xml:"objPath"`
	ResourceID        int64    `xml:"rescId"`
}

type HostAddr struct {
	XMLName  xml.Name `xml:"RHostAddr_PI"`
	Addr     string   `xml:"hostAddr"`
	Zone     string   `xml:"rodsZone"`
	Port     int      `xml:"port"`
	DummyInt int      `xml:"dummyInt"`
}

type FileStatResponse struct {
	XMLName    xml.Name `xml:"RODS_STAT_T_PI"`
	Size       int64    `xml:"st_size"`
	Dev        int      `xml:"st_dev"`
	Ino        int      `xml:"st_ino"`
	Mode       int      `xml:"st_mode"`
	Links      int      `xml:"st_nlink"`
	UID        int      `xml:"st_uid"`
	GID        int      `xml:"st_gid"`
	Rdev       int      `xml:"st_rdev"`
	AccessTime int      `xml:"st_atim"`
	ModifyTime int      `xml:"st_mtim"`
	ChangeTime int      `xml:"st_ctim"`
	BlkSize    int      `xml:"st_blksize"`
	Blocks     int      `xml:"st_blocks"`
}

type ModDataObjMetaRequest struct {
	XMLName xml.Name       `xml:"ModDataObjMeta_PI"`
	DataObj DataObjectInfo `xml:"DataObjInfo_PI"`
	KeyVals SSKeyVal       `xml:"KeyValPair_PI"`
}

type UnregDataObjRequest struct {
	XMLName xml.Name       `xml:"ModDataObjMeta_PI"`
	DataObj DataObjectInfo `xml:"DataObjInfo_PI"`
	KeyVals SSKeyVal       `xml:"KeyValPair_PI"`
}

type DataObjectInfo struct {
	XMLName                  xml.Name           `xml:"DataObjInfo_PI"`
	ObjPath                  string             `xml:"objPath"`
	RescName                 string             `xml:"rescName"`
	RescHier                 string             `xml:"rescHier"`
	DataType                 string             `xml:"dataType"`
	DataSize                 int64              `xml:"dataSize"`
	Chksum                   string             `xml:"chksum"`
	Version                  string             `xml:"version"`
	FilePath                 string             `xml:"filePath"`
	DataOwnerName            string             `xml:"dataOwnerName"`
	DataOwnerZone            string             `xml:"dataOwnerZone"`
	ReplNum                  int                `xml:"replNum"`
	ReplStatus               int                `xml:"replStatus"`
	StatusString             string             `xml:"statusString"`
	DataID                   int64              `xml:"dataId"`
	CollID                   int64              `xml:"collId"`
	DataMapID                int                `xml:"dataMapId"`
	Flags                    int                `xml:"flags"`
	DataComments             string             `xml:"dataComments"`
	DataMode                 string             `xml:"dataMode"`
	DataExpiry               string             `xml:"dataExpiry"`
	DataCreate               string             `xml:"dataCreate"`
	DataModify               string             `xml:"dataModify"`
	DataAccess               string             `xml:"dataAccess"`
	DataAccessInx            int                `xml:"dataAccessInx"`
	WriteFlag                int                `xml:"writeFlag"`
	DestRescName             string             `xml:"destRescName"`
	BackupRescName           string             `xml:"backupRescName"`
	SubPath                  string             `xml:"subPath"`
	SpecialCollectionPointer *SpecialCollection `xml:"SpecColl_PI"`
	RegUID                   int                `xml:"regUid"`
	OtherFlags               int                `xml:"otherFlags"`
	KeyVals                  SSKeyVal           `xml:"KeyValPair_PI"`
	InPdmo                   string             `xml:"in_pdmo"`
	Next                     *DataObjectInfo    `xml:"DataObjInfo_PI"`
	RescID                   int64              `xml:"rescId"`
}

type AdminRequest struct {
	XMLName xml.Name `xml:"generalAdminInp_PI"`
	Arg0    string   `xml:"arg0"` // add, modify, rm, ...
	Arg1    string   `xml:"arg1"` // user, group, zone, resource, ...
	Arg2    string   `xml:"arg2"`
	Arg3    string   `xml:"arg3"`
	Arg4    string   `xml:"arg4"`
	Arg5    string   `xml:"arg5"`
	Arg6    string   `xml:"arg6"`
	Arg7    string   `xml:"arg7"`
	Arg8    string   `xml:"arg8"` // unused
	Arg9    string   `xml:"arg9"` // unused
}

type ExecRuleRequest struct {
	XMLName  xml.Name     `xml:"ExecMyRuleInp_PI"`
	Rule     string       `xml:"myRule"`
	Host     HostAddr     `xml:"RHostAddr_PI"`
	KeyVals  SSKeyVal     `xml:"KeyValPair_PI"`
	OutParam string       `xml:"outParamDesc"`
	Params   MsParamArray `xml:"MsParamArray_PI"`
}

type MsParamArray struct {
	XMLName       xml.Name      `xml:"MsParamArray_PI"`
	Length        int           `xml:"paramLen"`
	OperationType OperationType `xml:"oprType"`
	Values        []MsParam     `xml:"MsParam_PI,omitempty"`
}

type MsParam struct {
	XMLName     xml.Name    `xml:"MsParam_PI"`
	Label       string      `xml:"label"`
	Type        string      `xml:"piStr"` // Must be STR_PI, we don't support other types (and irods doesn't either)
	InOut       string      `xml:"inOutStruct"`
	BinBytesBuf BinBytesBuf `xml:"BinBytesBuf_PI"`
}

type ErrorResponse struct {
	XMLName xml.Name `xml:"RError_PI"`
	Count   int      `xml:"count"`
	Errors  []RError `xml:"RErrMsg_PI"`
}

type RError struct {
	XMLName xml.Name `xml:"RErrMsg_PI"`
	Status  int      `xml:"status"`
	Message string   `xml:"msg"`
}

type AuthPluginRequest struct { // No xml.Name means this is a json struct
	ForcePasswordPrompt bool           `json:"force_password_prompt"`
	NextOperation       string         `json:"next_operation"`
	RecordAuthFile      bool           `json:"record_auth_file"`
	Scheme              string         `json:"scheme"`
	TTL                 string         `json:"a_ttl"`
	Password            string         `json:"a_pw"`
	UserName            string         `json:"user_name"`
	ZoneName            string         `json:"zone_name"`
	Digest              string         `json:"digest,omitempty"`
	PState              map[string]any `json:"pstate,omitempty"`
	PDirty              bool           `json:"pdirty,omitempty"`
	Response            string         `json:"resp,omitempty"`
}

type AuthPluginResponse struct { // No xml.Name means this is a json struct
	ForcePasswordPrompt bool           `json:"force_password_prompt"`
	NextOperation       string         `json:"next_operation"`
	RecordAuthFile      bool           `json:"record_auth_file"`
	Scheme              string         `json:"scheme"`
	TTL                 string         `json:"a_ttl"`
	UserName            string         `json:"user_name"`
	ZoneName            string         `json:"zone_name"`
	Digest              string         `json:"digest,omitempty"`
	PState              map[string]any `json:"pstate,omitempty"`
	PDirty              bool           `json:"pdirty,omitempty"`
	Message             struct {
		Prompt      string           `json:"prompt,omitempty"`
		Retrieve    string           `json:"retrieve,omitempty"`
		DefaultPath string           `json:"default_path,omitempty"`
		Patch       []map[string]any `json:"patch,omitempty"`
	} `json:"msg,omitempty"`
	RequestResult string `json:"request_result,omitempty"`
}

type Checksum struct {
	XMLName  xml.Name `xml:"STR_PI"`
	Checksum string   `xml:"myStr"`
}
