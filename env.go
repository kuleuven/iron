package iron

import (
	"encoding/json"
	"os"
)

type Env struct {
	Host                          string `json:"irods_host"`
	Port                          int    `json:"irods_port"`
	Zone                          string `json:"irods_zone_name"`
	AuthScheme                    string `json:"irods_authentication_scheme"`
	EncryptionAlgorithm           string `json:"irods_encryption_algorithm"`
	EncryptionSaltSize            int    `json:"irods_encryption_salt_size"`
	EncryptionKeySize             int    `json:"irods_encryption_key_size"`
	EncryptionNumHashRounds       int    `json:"irods_encryption_num_hash_rounds"`
	Username                      string `json:"irods_user_name"`
	Password                      string `json:"irods_password"`
	SSLCACertificateFile          string `json:"irods_ssl_ca_certificate_file"`
	SSLVerifyServer               string `json:"irods_ssl_verify_server"`
	ClientServerNegotiation       string `json:"irods_client_server_negotiation"`
	ClientServerNegotiationPolicy string `json:"irods_client_server_policy"`
	DefaultResource               string `json:"irods_default_resource"`
	PamTTL                        int    `json:"-"` // For pam authentication, the TTL to use for the generated native password
	SSLServerName                 string `json:"-"` // If provided, this will be used for server verification
	ProxyUsername                 string `json:"-"` // Authenticate with proxy credentials
	ProxyZone                     string `json:"-"` // Authenticate with proxy credentials
}

const (
	native = "native"
	pam    = "pam_password"
)

const (
	ClientServerRefuseTLS  = "CS_NEG_REFUSE"
	ClientServerRequireTLS = "CS_NEG_REQUIRE"
	ClientServerDontCare   = "CS_NEG_DONT_CARE"
)

var DefaultEnv = Env{
	Port:                          1247,
	AuthScheme:                    native,
	EncryptionAlgorithm:           "AES-256-CBC",
	EncryptionSaltSize:            8,
	EncryptionKeySize:             32,
	EncryptionNumHashRounds:       8,
	Username:                      "rods",
	SSLVerifyServer:               "cert",
	ClientServerNegotiation:       "request_server_negotiation",
	ClientServerNegotiationPolicy: "CS_NEG_REQUIRE",
	DefaultResource:               "demoResc",
}

func (env *Env) LoadFromFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}

	defer f.Close()

	return json.NewDecoder(f).Decode(env)
}

func (env *Env) ApplyDefaults() {
	if env.Port == 0 {
		env.Port = DefaultEnv.Port
	}

	if env.AuthScheme == "" {
		env.AuthScheme = DefaultEnv.AuthScheme
	}

	if env.EncryptionAlgorithm == "" {
		env.EncryptionAlgorithm = DefaultEnv.EncryptionAlgorithm
	}

	if env.EncryptionSaltSize == 0 {
		env.EncryptionSaltSize = DefaultEnv.EncryptionSaltSize
	}

	if env.EncryptionKeySize == 0 {
		env.EncryptionKeySize = DefaultEnv.EncryptionKeySize
	}

	if env.EncryptionNumHashRounds == 0 {
		env.EncryptionNumHashRounds = DefaultEnv.EncryptionNumHashRounds
	}

	if env.Username == "" {
		env.Username = DefaultEnv.Username
	}

	if env.SSLVerifyServer == "" {
		env.SSLVerifyServer = DefaultEnv.SSLVerifyServer
	}

	if env.ClientServerNegotiation == "" {
		env.ClientServerNegotiation = DefaultEnv.ClientServerNegotiation
	}

	if env.ClientServerNegotiationPolicy == "" {
		env.ClientServerNegotiationPolicy = DefaultEnv.ClientServerNegotiationPolicy
	}

	if env.DefaultResource == "" {
		env.DefaultResource = DefaultEnv.DefaultResource
	}

	if env.ProxyUsername == "" {
		env.ProxyUsername = env.Username
	}

	if env.ProxyZone == "" {
		env.ProxyZone = env.Zone
	}

	if env.PamTTL <= 0 {
		env.PamTTL = 60
	}
}
