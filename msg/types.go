package msg

import "encoding/xml"

type StartupPack struct {
	XMLName         xml.Name `xml:"StartupPack_PI"`
	Protocol        int      `xml:"irodsProt"`
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
	Keys    []string `xml:"keyWord,omitempty"`
	Values  []string `xml:"svalue,omitempty"`
}

func (kv *SSKeyVal) Add(key string, val string) {
	kv.Keys = append(kv.Keys, key)
	kv.Values = append(kv.Values, val)
	kv.Length++
}

type IIKeyVal struct {
	XMLName xml.Name `xml:"InxIvalPair_PI"`
	Length  int      `xml:"iiLen"`
	Keys    []int    `xml:"inx,omitempty"`
	Values  []int    `xml:"ivalue,omitempty"`
}

func (kv *IIKeyVal) Add(key int, val int) {
	kv.Keys = append(kv.Keys, key)
	kv.Values = append(kv.Values, val)
	kv.Length++
}

type ISKeyVal struct {
	XMLName xml.Name `xml:"InxValPair_PI"`
	Length  int      `xml:"isLen"`
	Keys    []int    `xml:"inx,omitempty"`
	Values  []string `xml:"svalue,omitempty"`
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
	SQLResult      []SQLResult `xml:"SqlResult_PI"`
}

type SQLResult struct {
	XMLName        xml.Name `xml:"SqlResult_PI"`
	AttributeIndex int      `xml:"attriInx"`
	ResultLen      int      `xml:"reslen"`
	Values         []string `xml:"value,omitempty"`
}
