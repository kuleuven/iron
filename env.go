package iron

import (
	"encoding/json"
	"os"
	"time"
)

// Env contains the IRODS connection parameters to establish a connection.
type Env struct {
	// Credentials and settings
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
	ProxyUsername                 string `json:"irods_proxy_user"` // Authenticate with proxy credentials
	ProxyZone                     string `json:"irods_proxy_zone"` // Authenticate with proxy credentials

	// Advanced dial settings
	SSLServerName            string        `json:"-"` // If provided, this will be used for server verification, instead of the hostname
	DialTimeout              time.Duration `json:"-"` // If provided, this will be used for dial timeout
	HandshakeTimeout         time.Duration `json:"-"` // If provided, this will be used for the handshake timeout
	GeneratedPasswordTimeout time.Duration `json:"-"` // For pam authentication, the TTL to use for the generated native password

	// Persistent state for PAM interactive authentication
	PersistentState PersistentState `json:"-"` // If provided, this will be used for PAM authentication as persistent store
}

// PersistentState refers to the pstate used for PAM interactive authentication
// The functions must be thread-safe if multiple clients/connections are used concurrently.
type PersistentState interface {
	Load(m map[string]any) error // Load returns the current persistent state into the given map.
	Save(m map[string]any) error // Save persists the given state. Later calls to Load will reconstruct the same map contents.
}

const (
	native         = "native"
	pamPassword    = "pam_password"
	pamInteractive = "pam_interactive"
)

const (
	ClientServerRefuseTLS  = "CS_NEG_REFUSE"
	ClientServerRequireTLS = "CS_NEG_REQUIRE"
	ClientServerDontCare   = "CS_NEG_DONT_CARE"
)

// DefaultEnv contains the default IRODS connection parameters.
// Use ApplyDefaults() to apply the default values to an environment.
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
	DialTimeout:                   time.Minute,
	HandshakeTimeout:              time.Minute,
	GeneratedPasswordTimeout:      time.Hour,
}

// LoadFromFile reads an environment configuration from a JSON file at the given path,
// overwriting the fields of the receiver.
func (env *Env) LoadFromFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}

	defer f.Close()

	return json.NewDecoder(f).Decode(env)
}

// ApplyDefaults sets default values for the environment fields if they are not already set.
// It uses the values from DefaultEnv for most fields. If the ProxyUsername and ProxyZone
// are not specified, it uses the Username and Zone respectively. Additionally, if PamTTL
// is not set or is less than or equal to zero, it defaults to 60
func (env *Env) ApplyDefaults() { //nolint:funlen
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

	if env.GeneratedPasswordTimeout == 0 {
		env.GeneratedPasswordTimeout = DefaultEnv.GeneratedPasswordTimeout
	}

	if env.DialTimeout == 0 {
		env.DialTimeout = DefaultEnv.DialTimeout
	}

	if env.HandshakeTimeout == 0 {
		env.HandshakeTimeout = DefaultEnv.HandshakeTimeout
	}

	if env.PersistentState == nil {
		env.PersistentState = &discardPersistentState{}
	}
}

type discardPersistentState struct{}

func (s *discardPersistentState) Load(m map[string]any) error {
	return nil
}

func (s *discardPersistentState) Save(m map[string]any) error {
	return nil
}
