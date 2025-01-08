package msg

import "encoding/xml"

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
	Challenge string   `xml:"challenge"`
}

type AuthChallengeResponse struct {
	XMLName  xml.Name `xml:"authResponseInp_PI"`
	Response string   `xml:"response"`
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
	XMLName xml.Name `xml:"KeyValPair_PI"`
	Length  int      `xml:"ssLen"`
	Keys    []string `xml:"keyWord" sizeField:"ssLen"`
	Values  []string `xml:"svalue" sizeField:"ssLen"`
}

func (kv *SSKeyVal) Add(key string, val string) {
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

func (kv *IIKeyVal) Add(key int, val int) {
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
	XMLName        xml.Name `xml:"SqlResult_PI"`
	AttributeIndex int      `xml:"attriInx"`
	ResultLen      int      `xml:"reslen"`
	Values         []string `xml:"value,omitempty" sizeField:"reslen"`
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
	Size           int64          `xml:"len"`
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
