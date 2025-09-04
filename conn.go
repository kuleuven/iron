package iron

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/go-openapi/jsonpointer"
	"github.com/hashicorp/go-rootcerts"
	"github.com/kuleuven/iron/api"
	"github.com/kuleuven/iron/msg"
	"github.com/kuleuven/iron/scramble"
	"github.com/sirupsen/logrus"
	"go.uber.org/multierr"
)

type Conn interface {
	// Env returns the connection environment
	Env() Env

	// Conn returns the underlying net.Conn
	Conn() net.Conn

	// ServerVersion returns the version that the iRODS server reports
	// e.g. "4.3.2"
	ServerVersion() string

	// ClientSignature returns the client signature
	ClientSignature() string

	// NativePassword returns the native password
	// In case of PAM authentication, this is the generated password
	NativePassword() string

	// Request sends an API request for the given API number and expects a response.
	// Both request and response should represent a type such as in `msg/types.go`.
	// The request and response will be marshaled and unmarshaled automatically.
	// If a negative IntInfo is returned, an appropriate error will be returned.
	// This method is thread-safe.
	Request(ctx context.Context, apiNumber msg.APINumber, request, response any) error

	// RequestWithBuffers behaves as Request, with provided buffers for the request
	// and response binary data. Both requestBuf and responseBuf could be nil.
	// This method is thread-safe.
	RequestWithBuffers(ctx context.Context, apiNumber msg.APINumber, request, response any, requestBuf, responseBuf []byte) error

	// API returns an API using the current connection.
	API() *api.API

	// Close closes the connection.
	// It is safe to call Close multiple times.
	// This method is thread-safe but will obviously make future requests fail.
	Close() error

	// RegisterCloseHandler registers a function to be called when the connection is
	// about to closed. It is used to clean up state before the connection is closed.
	// The CloseHandler can be unregistered by calling the returned function.
	RegisterCloseHandler(handler func() error) context.CancelFunc
}

type conn struct {
	transport net.Conn
	env       *Env
	option    string
	protocol  msg.Protocol

	// Set during handshake
	connectedAt     time.Time
	useTLS          bool
	version         *msg.Version
	clientSignature string
	nativePassword  string
	transportErrors int
	sqlErrors       int

	// housekeeping
	doRequest     sync.Mutex
	doClose       sync.Mutex
	closeHandlers []func() error
	closed        bool
	closeErr      error
}

// Dialer is used to connect to an IRODS server.
var Dialer = net.Dialer{
	Timeout: time.Minute,
}

type DialFunc func(ctx context.Context, env Env, clientName string) (net.Conn, error)

func DefaultDialFunc(ctx context.Context, env Env, clientName string) (net.Conn, error) {
	return Dialer.DialContext(ctx, "tcp", net.JoinHostPort(env.Host, strconv.FormatInt(int64(env.Port), 10)))
}

// Dial connects to an IRODS server and creates a new connection.
// The caller is responsible for closing the connection when it is no longer needed.
func Dial(ctx context.Context, env Env, clientName string) (Conn, error) {
	return dial(ctx, env, clientName, DefaultDialFunc, StdPrompt, msg.XML)
}

// PromptDial connects to an IRODS server and creates a new connection.
// Prompt is used for possible interactive authentication.
// The caller is responsible for closing the connection when it is no longer needed.
func PromptDial(ctx context.Context, env Env, prompt Prompt, clientName string) (Conn, error) {
	return dial(ctx, env, clientName, DefaultDialFunc, prompt, msg.XML)
}

func dial(ctx context.Context, env Env, clientName string, dialFunc DialFunc, prompt Prompt, protocol msg.Protocol) (*conn, error) {
	if dialFunc == nil {
		dialFunc = DefaultDialFunc
	}

	conn, err := dialFunc(ctx, env, clientName)
	if err != nil {
		return nil, err
	}

	return newConn(ctx, conn, env, clientName, prompt, protocol)
}

var HandshakeTimeout = time.Minute

// NewConn initializes a new Conn instance with the provided network connection and environment settings.
// It performs a handshake as part of the initialization process and returns the constructed Conn instance.
// Returns an error if the handshake fails.
func NewConn(ctx context.Context, transport net.Conn, env Env, clientName string) (Conn, error) {
	return newConn(ctx, transport, env, clientName, StdPrompt, msg.XML)
}

// NewPromptConn initializes a new Conn instance with the provided network connection and environment settings.
// It performs a handshake as part of the initialization process and returns the constructed Conn instance.
// For interactive authentication, the provided prompt will be used. Returns an error if the handshake fails.
func NewPromptConn(ctx context.Context, transport net.Conn, env Env, prompt Prompt, clientName string) (Conn, error) {
	return newConn(ctx, transport, env, clientName, prompt, msg.XML)
}

const requestServerNegotiationToken = "request_server_negotiation"

func newConn(ctx context.Context, transport net.Conn, env Env, clientName string, prompt Prompt, protocol msg.Protocol) (*conn, error) {
	c := &conn{
		transport:   transport,
		env:         &env,
		option:      clientName,
		protocol:    protocol,
		connectedAt: time.Now(),
	}

	// Make sure TLS is required when not using native authentication
	if c.env.AuthScheme != native {
		if c.env.ClientServerNegotiation != requestServerNegotiationToken {
			return nil, ErrTLSRequired
		}

		if c.env.ClientServerNegotiationPolicy == ClientServerRefuseTLS {
			return nil, ErrTLSRequired
		}

		c.env.ClientServerNegotiationPolicy = ClientServerRequireTLS
	}

	ctx, cancel := context.WithTimeout(ctx, HandshakeTimeout)

	defer cancel()

	return c, c.Handshake(ctx, prompt)
}

var ErrTLSRequired = fmt.Errorf("TLS is required for authentication but not enabled")

// Env returns the connection environment
func (c *conn) Env() Env {
	return *c.env
}

// Conn returns the underlying network connection.
func (c *conn) Conn() net.Conn {
	return c.transport
}

// ServerVersion returns the version that the iRODS server reports
func (c *conn) ServerVersion() string {
	return c.version.ReleaseVersion[4:]
}

// ClientSignature returns the client signature
func (c *conn) ClientSignature() string {
	return c.clientSignature
}

// NativePassword returns the native password
func (c *conn) NativePassword() string {
	return c.nativePassword
}

// Handshake performs a handshake with the IRODS server.
func (c *conn) Handshake(ctx context.Context, prompt Prompt) error {
	if err := c.startup(ctx); err != nil {
		// If the context is closed, the error will be a "closed network connection" error.
		// In that case, return the context error instead
		if cErr := ctx.Err(); cErr != nil && errors.Is(err, net.ErrClosed) {
			return cErr
		}

		return err
	}

	return c.authenticate(ctx, prompt)
}

var ErrUnsupportedVersion = fmt.Errorf("unsupported server version")

func (c *conn) startup(ctx context.Context) error {
	cancel := c.CloseOnCancel(ctx)

	defer cancel()

	pack := msg.StartupPack{
		Protocol:       c.protocol,
		ReleaseVersion: "rods4.3.0",
		APIVersion:     "d",
		ClientUser:     c.env.Username,
		ClientRcatZone: c.env.Zone,
		ProxyUser:      c.env.ProxyUsername,
		ProxyRcatZone:  c.env.ProxyZone,
		Option:         c.option,
	}

	if c.env.ClientServerNegotiation == requestServerNegotiationToken {
		pack.Option = fmt.Sprintf("%s%s", pack.Option, c.env.ClientServerNegotiation)
	}

	if err := msg.Write(c.transport, pack, nil, msg.XML, "RODS_CONNECT", 0); err != nil {
		return err
	}

	if c.env.ClientServerNegotiation == requestServerNegotiationToken {
		if err := c.handshakeNegotiation(); err != nil {
			return err
		}
	}

	version := msg.Version{}

	if _, err := msg.Read(c.transport, &version, nil, msg.XML, "RODS_VERSION"); err != nil {
		return err
	}

	if !checkVersion(version, 4, 3, 2) {
		return fmt.Errorf("%w: server version %v", ErrUnsupportedVersion, version.ReleaseVersion)
	}

	c.version = &version

	if !c.useTLS {
		return nil
	}

	return c.handshakeTLS()
}

// checkVersion returns true if the server version is greater than or equal to the given version
func checkVersion(version msg.Version, major, minor, release int) bool {
	if !strings.HasPrefix(version.ReleaseVersion, "rods") {
		return false
	}

	parts := strings.Split(version.ReleaseVersion[4:], ".")

	if len(parts) != 3 {
		return false
	}

	myMajor, err := strconv.Atoi(parts[0])
	if err != nil {
		return false
	}

	myMinor, err := strconv.Atoi(parts[1])
	if err != nil {
		return false
	}

	myRelease, err := strconv.Atoi(parts[2])
	if err != nil {
		return false
	}

	return myMajor > major || (myMajor == major && myMinor > minor) || (myMajor == major && myMinor == minor && myRelease >= release)
}

var ErrSSLNegotiationFailed = fmt.Errorf("SSL negotiation failed")

func (c *conn) handshakeNegotiation() error {
	neg := msg.ClientServerNegotiation{}

	if _, err := msg.Read(c.transport, &neg, nil, msg.XML, "RODS_CS_NEG_T"); err != nil {
		return err
	}

	failure := msg.ClientServerNegotiation{
		Result: "cs_neg_result_kw=CS_NEG_FAILURE;",
		Status: 0,
	}

	if neg.Result == ClientServerRefuseTLS && c.env.ClientServerNegotiationPolicy == ClientServerRequireTLS {
		// Report failure
		msg.Write(c.transport, failure, nil, msg.XML, "RODS_CS_NEG_T", 0) //nolint:errcheck

		return fmt.Errorf("%w: server refuses SSL, client requires SSL", ErrSSLNegotiationFailed)
	}

	if neg.Result == ClientServerRequireTLS && c.env.ClientServerNegotiationPolicy == ClientServerRefuseTLS {
		// Report failure
		msg.Write(c.transport, failure, nil, msg.XML, "RODS_CS_NEG_T", 0) //nolint:errcheck

		return fmt.Errorf("%w: client refuses SSL, server requires SSL", ErrSSLNegotiationFailed)
	}

	// Only disable SSL if it is refused by the server or the client
	if neg.Result == ClientServerRefuseTLS || c.env.ClientServerNegotiationPolicy == ClientServerRefuseTLS {
		neg.Result = "cs_neg_result_kw=CS_NEG_USE_TCP;"
	} else {
		neg.Result = "cs_neg_result_kw=CS_NEG_USE_SSL;"
		c.useTLS = true
	}

	neg.Status = 1

	return msg.Write(c.transport, neg, nil, msg.XML, "RODS_CS_NEG_T", 0)
}

var ErrUnknownSSLVerifyPolicy = fmt.Errorf("unknown SSL verification policy")

// Make configurable for testing
var tlsTime = time.Now

func verifyPeerCertificateNoHostname(tlsConfig *tls.Config, certificates [][]byte) error {
	certs := make([]*x509.Certificate, len(certificates))

	for i, asn1Data := range certificates {
		cert, err := x509.ParseCertificate(asn1Data)
		if err != nil {
			return err
		}

		certs[i] = cert
	}

	opts := x509.VerifyOptions{
		Roots:         tlsConfig.RootCAs,
		CurrentTime:   tlsConfig.Time(),
		Intermediates: x509.NewCertPool(),
	}

	for _, cert := range certs[1:] {
		opts.Intermediates.AddCert(cert)
	}

	if _, err := certs[0].Verify(opts); err != nil {
		return &tls.CertificateVerificationError{UnverifiedCertificates: certs, Err: err}
	}

	return nil
}

func (c *conn) handshakeTLS() error {
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		Time:       tlsTime,
	}

	switch c.env.SSLVerifyServer {
	case "cert":
		tlsConfig.ServerName = c.env.Host

		if c.env.SSLServerName != "" {
			tlsConfig.ServerName = c.env.SSLServerName
		}
	case "host":
		tlsConfig.InsecureSkipVerify = true
		tlsConfig.VerifyPeerCertificate = func(certificates [][]byte, _ [][]*x509.Certificate) error {
			return verifyPeerCertificateNoHostname(tlsConfig, certificates)
		}
	case "none":
		tlsConfig.InsecureSkipVerify = true
	default:
		return fmt.Errorf("%w: %s", ErrUnknownSSLVerifyPolicy, c.env.SSLVerifyServer)
	}

	if c.env.SSLCACertificateFile != "" {
		var err error

		tlsConfig.RootCAs, err = rootcerts.LoadCACerts(&rootcerts.Config{
			CAFile: c.env.SSLCACertificateFile,
		})
		if err != nil {
			return err
		}
	}

	tlsConn := tls.Client(c.transport, tlsConfig)

	if err := tlsConn.Handshake(); err != nil {
		return err
	}

	c.transport = tlsConn
	encryptionKey := make([]byte, c.env.EncryptionKeySize) // Generate encryption key

	if _, err := rand.Read(encryptionKey); err != nil {
		return err
	}

	// The encryption key is not sent as a packet but abuses the header format to send it
	sslSettings := msg.Header{
		Type:       c.env.EncryptionAlgorithm,
		MessageLen: uint32(c.env.EncryptionKeySize),
		ErrorLen:   uint32(c.env.EncryptionSaltSize),
		BsLen:      uint32(c.env.EncryptionNumHashRounds),
	}

	if err := sslSettings.Write(c.transport); err != nil {
		return err
	}

	return msg.Write(c.transport, msg.SSLSharedSecret(encryptionKey), nil, c.protocol, "SHARED_SECRET", 0)
}

var ErrNotImplemented = fmt.Errorf("not implemented")

func (c *conn) authenticate(ctx context.Context, prompt Prompt) error {
	if prompt == nil {
		prompt = StdPrompt
	}

	switch c.env.AuthScheme {
	case pamPassword:
		if err := c.askPassword(prompt); err != nil {
			return err
		}

		if err := c.authenticatePAMPassword(ctx); err != nil {
			return err
		}
	case pamInteractive:
		if err := c.authenticatePAM(ctx, prompt); err != nil {
			return err
		}
	case native:
		if err := c.askPassword(prompt); err != nil {
			return err
		}

		c.nativePassword = c.env.Password

	default:
		return fmt.Errorf("%w: authentication scheme %s", ErrNotImplemented, c.env.AuthScheme)
	}

	return c.authenticateNative(ctx)
}

func (c *conn) askPassword(prompt Prompt) error {
	if c.env.Password != "" {
		return nil
	}

	var err error

	c.env.Password, err = prompt.Password("Password")

	return err
}

func (c *conn) authenticateNative(ctx context.Context) error {
	if !checkVersion(*c.version, 5, 0, 0) {
		return c.authenticateNativeDeprecated(ctx)
	}

	// Request challenge
	request := msg.AuthPluginRequest{
		Scheme:        "native",
		NextOperation: "auth_agent_auth_request",
		UserName:      c.env.ProxyUsername,
		ZoneName:      c.env.ProxyZone,
	}

	var response msg.AuthPluginResponse

	if err := c.Request(ctx, msg.AUTH_PLUGIN, request, &response); err != nil {
		return err
	}

	request.NextOperation = "auth_agent_auth_response"

	// In the new authentication scheme, the server sends the challenge not base64 encoded
	challengeBytes := []byte(response.RequestResult)

	// Compute digest
	request.Digest = scramble.GenerateAuthResponse(challengeBytes, c.nativePassword)

	// Save client signature
	c.clientSignature = hex.EncodeToString(challengeBytes[:min(16, len(challengeBytes))])

	return c.Request(ctx, msg.AUTH_PLUGIN, request, &response)
}

func (c *conn) authenticateNativeDeprecated(ctx context.Context) error {
	// Request challenge
	challenge := msg.AuthChallenge{}

	if err := c.Request(ctx, msg.AUTH_REQUEST_AN, msg.AuthRequest{}, &challenge); err != nil {
		return err
	}

	challengeBytes, err := base64.StdEncoding.DecodeString(challenge.Challenge)
	if err != nil {
		return err
	}

	// Save client signature
	c.clientSignature = hex.EncodeToString(challengeBytes[:min(16, len(challengeBytes))])

	// Create challenge response
	response := msg.AuthChallengeResponse{
		Response: scramble.GenerateAuthResponse(challengeBytes, c.nativePassword),
		Username: c.env.ProxyUsername,
	}

	logrus.Debugf("Responding %s %s ", response.Response, response.Username)

	return c.Request(ctx, msg.AUTH_RESPONSE_AN, response, &msg.AuthResponse{})
}

func (c *conn) authenticatePAM(ctx context.Context, prompt Prompt) error {
	// Request challenge
	request := msg.AuthPluginRequest{
		Scheme:              "pam_interactive",
		TTL:                 strconv.Itoa(c.env.PamTTL),
		ForcePasswordPrompt: true,
		RecordAuthFile:      true,
		NextOperation:       "auth_agent_auth_request",
		UserName:            c.env.ProxyUsername,
		ZoneName:            c.env.ProxyZone,
		PState:              map[string]any{},
	}

	for {
		var response msg.AuthPluginResponse

		if err := c.Request(ctx, msg.AUTH_PLUGIN, request, &response); err != nil {
			return err
		}

		var err error

		switch response.NextOperation {
		case "auth_agent_auth_request":
			request.NextOperation = "auth_agent_auth_response"

		case "next":
			if response.Message.Prompt == "" {
				break
			}

			err = prompt.Print(response.Message.Prompt)

		case "waiting", "waiting_pw":
			request.Response, err = getValue(request.PState, prompt, response.Message.Prompt, response.NextOperation == "waiting_pw", response.Message.Retrieve, response.Message.DefaultPath)

		case "authenticated":
			c.nativePassword = response.RequestResult

			return nil

		default:
			return fmt.Errorf("unexpected next operation %s", response.NextOperation)
		}

		if err != nil {
			return err
		}

		if err := patchState(request.PState, response.Message.Patch, &request.PDirty, request.Response); err != nil {
			return err
		}
	}
}

func getValue(state map[string]any, prompt Prompt, message string, sensitive bool, retrievePath, defaultPath string) (string, error) {
	if retrievePath != "" {
		return retrieveValue(state, retrievePath)
	}

	// Get default value
	var defaultValue string

	if defaultPath != "" {
		var err error

		defaultValue, err = retrieveValue(state, defaultPath)
		if err != nil {
			return "", err
		}
	}

	// Prompt for value
	return promptValue(prompt, message, sensitive, defaultValue)
}

func retrieveValue(state map[string]any, path string) (string, error) {
	pointer, err := jsonpointer.New(path)
	if err != nil {
		return "", err
	}

	value, _, err := pointer.Get(state)
	if err != nil {
		return "", nil //nolint:nilerr
	}

	s, _ := value.(string)

	return s, nil
}

func patchState(state map[string]any, patch []map[string]any, dirty *bool, defaultValue string) error {
	if len(patch) == 0 {
		return nil
	}

	for _, p := range patch {
		if p["op"] != "add" && p["op"] != "replace" {
			continue
		}

		if _, ok := p["value"]; ok {
			continue
		}

		p["value"] = defaultValue
	}

	patchPayload, err := json.Marshal(patch)
	if err != nil || len(patchPayload) == 0 {
		return err
	}

	decodedPatch, err := jsonpatch.DecodePatch(patchPayload)
	if err != nil {
		return err
	}

	statePayload, err := json.Marshal(state)
	if err != nil {
		return err
	}

	statePayload, err = decodedPatch.Apply(statePayload)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(statePayload, &state); err != nil {
		return err
	}

	*dirty = true

	return nil
}

func promptValue(prompt Prompt, message string, sensitive bool, defaultValue string) (string, error) {
	if defaultValue != "" {
		message = fmt.Sprintf("%s [%s]", message, defaultValue)
	}

	var (
		value string
		err   error
	)

	if sensitive {
		value, err = prompt.Password(message)
	} else {
		value, err = prompt.Ask(message)
	}

	if value == "" {
		value = defaultValue
	}

	return value, err
}

func (c *conn) authenticatePAMPassword(ctx context.Context) error {
	if !checkVersion(*c.version, 5, 0, 0) {
		return c.authenticatePAMPasswordDeprecated(ctx)
	}

	// Request challenge
	request := msg.AuthPluginRequest{
		Scheme:        "pam_password",
		TTL:           strconv.Itoa(c.env.PamTTL),
		NextOperation: "pam_password_auth_client_request",
		UserName:      c.env.ProxyUsername,
		ZoneName:      c.env.ProxyZone,
		Password:      c.env.Password,
	}

	var response msg.AuthPluginResponse

	if err := c.Request(ctx, msg.AUTH_PLUGIN, request, &response); err != nil {
		return err
	}

	c.nativePassword = response.RequestResult

	return nil
}

func (c *conn) authenticatePAMPasswordDeprecated(ctx context.Context) error {
	request := msg.PamAuthRequest{
		Username: c.env.ProxyUsername,
		Password: c.env.Password,
		TTL:      c.env.PamTTL,
	}

	response := msg.PamAuthResponse{}

	if err := c.Request(ctx, msg.PAM_AUTH_REQUEST_AN, request, &response); err != nil {
		return err
	}

	c.nativePassword = response.GeneratedPassword

	return nil
}

// Request sends an API request to the server and expects a API reply.
// If a negative IntInfo is received, an IRODSError is returned.
func (c *conn) Request(ctx context.Context, apiNumber msg.APINumber, request, response any) error {
	return c.RequestWithBuffers(ctx, apiNumber, request, response, nil, nil)
}

// Request sends an API request to the server and expects a API reply,
// with possible request and response buffers.
// If a negative IntInfo is received, an IRODSError is returned.
func (c *conn) RequestWithBuffers(ctx context.Context, apiNumber msg.APINumber, request, response any, requestBuf, responseBuf []byte) error {
	c.doRequest.Lock()

	defer c.doRequest.Unlock()

	if err := ctx.Err(); err != nil {
		return err
	}

	if err := msg.Write(c.transport, request, requestBuf, c.protocol, "RODS_API_REQ", int32(apiNumber)); err != nil {
		c.transportErrors++

		return err
	}

	m := msg.Message{
		Bin: responseBuf,
	}

	if err := m.Read(c.transport); err != nil {
		c.transportErrors++

		return err
	}

	if expectedMsgType := "RODS_API_REPLY"; m.Header.Type != expectedMsgType {
		return fmt.Errorf("%w: expected %s, got %s", msg.ErrUnexpectedMessage, expectedMsgType, m.Header.Type)
	}

	// The api call RM_COLL_AN is a special case, an extended version of irods returns the payload
	// only if we request it using a special code. However it is still optional, so it is possible that
	// the server returns a zero IntInfo and an empty response, but this is fine as UnmarshalXML will
	// not complain in this case if the message length is zero.
	if apiNumber == msg.RM_COLL_AN && m.Header.IntInfo == msg.SYS_SVR_TO_CLI_COLL_STAT {
		return c.handleCollStat(response, responseBuf)
	}

	if m.Header.IntInfo < 0 {
		if msg.ErrorCode(m.Header.IntInfo) == msg.CAT_SQL_ERR {
			c.sqlErrors++
		}

		return &msg.IRODSError{
			Code:    msg.ErrorCode(m.Header.IntInfo),
			Message: c.buildError(m),
		}
	}

	return msg.Unmarshal(m, c.protocol, response)
}

func (c *conn) API() *api.API {
	return &api.API{
		Username: c.env.Username,
		Zone:     c.env.Zone,
		Connect: func(ctx context.Context) (api.Conn, error) {
			return &dummyCloser{c}, nil
		},
		DefaultResource: c.env.DefaultResource,
	}
}

type dummyCloser struct {
	*conn
}

func (d *dummyCloser) Close() error {
	return nil
}

func (c *conn) handleCollStat(response any, responseBuf []byte) error {
	// Send special code
	replyBuffer := make([]byte, 4)
	binary.BigEndian.PutUint32(replyBuffer, uint32(msg.SYS_CLI_TO_SVR_COLL_STAT_REPLY))

	if _, err := c.transport.Write(replyBuffer); err != nil {
		c.transportErrors++

		return err
	}

	m := msg.Message{
		Bin: responseBuf,
	}

	if err := m.Read(c.transport); err != nil {
		c.transportErrors++

		return err
	}

	if expectedMsgType := "RODS_API_REPLY"; m.Header.Type != expectedMsgType {
		return fmt.Errorf("%w: expected %s, got %s", msg.ErrUnexpectedMessage, expectedMsgType, m.Header.Type)
	}

	if m.Header.IntInfo < 0 {
		if msg.ErrorCode(m.Header.IntInfo) == msg.CAT_SQL_ERR {
			c.sqlErrors++
		}

		return &msg.IRODSError{
			Code:    msg.ErrorCode(m.Header.IntInfo),
			Message: c.buildError(m),
		}
	}

	return msg.Unmarshal(m, c.protocol, response)
}

func (c *conn) buildError(m msg.Message) string {
	if m.Header.ErrorLen == 0 {
		return string(m.Body.Message)
	}

	var rodsErr msg.ErrorResponse

	if c.protocol == msg.Native {
		if err := msg.DecodeC(m.Body.Error, &rodsErr); err != nil {
			return string(m.Body.Error)
		}
	} else {
		payload, err := msg.PostprocessXML(m.Body.Error)
		if err != nil {
			return string(m.Body.Error)
		}

		if xml.Unmarshal(payload, &rodsErr) != nil {
			return string(m.Body.Error)
		}
	}

	var msgs []string

	for _, msg := range rodsErr.Errors {
		msgs = append(msgs, msg.Message)
	}

	return strings.Join(msgs, "; ")
}

func (c *conn) Close() error {
	c.doClose.Lock()
	defer c.doClose.Unlock()

	if c.closed {
		return c.closeErr
	}

	for _, handler := range c.closeHandlers {
		if handler == nil {
			continue
		}

		if err := handler(); err != nil {
			c.closeErr = multierr.Append(c.closeErr, err)
		}
	}

	c.doRequest.Lock()
	defer c.doRequest.Unlock()

	c.closeErr = multierr.Append(c.closeErr, msg.Write(c.transport, msg.EmptyResponse{}, nil, c.protocol, "RODS_DISCONNECT", 0))

	if tlsConn, ok := c.transport.(*tls.Conn); ok {
		c.closeErr = multierr.Append(c.closeErr, tlsConn.CloseWrite())
		c.closeErr = multierr.Append(c.closeErr, tlsConn.SetReadDeadline(time.Now().Add(100*time.Millisecond)))

		if _, err := io.Copy(io.Discard, tlsConn); err != nil {
			c.closeErr = multierr.Append(c.closeErr, err)
		}
	}

	c.closeErr = multierr.Append(c.closeErr, c.Conn().Close())
	c.closed = true

	return c.closeErr
}

func (c *conn) RegisterCloseHandler(handler func() error) context.CancelFunc {
	c.doClose.Lock()
	defer c.doClose.Unlock()

	c.closeHandlers = append(c.closeHandlers, handler)

	i := len(c.closeHandlers) - 1

	return func() {
		c.doClose.Lock()
		defer c.doClose.Unlock()

		c.closeHandlers[i] = nil
	}
}

func (c *conn) CloseOnCancel(ctx context.Context) context.CancelFunc {
	done := make(chan struct{})

	go func() {
		select {
		case <-ctx.Done():
			c.Close()
		case <-done:
		}
	}()

	return func() {
		close(done)
	}
}
