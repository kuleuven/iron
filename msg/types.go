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
