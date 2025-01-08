package iron

import (
	"context"
	"crypto/md5" //nolint:gosec
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"gitea.icts.kuleuven.be/coz/iron/api"
	"gitea.icts.kuleuven.be/coz/iron/msg"
	"github.com/hashicorp/go-rootcerts"
	"go.uber.org/multierr"

	"github.com/sirupsen/logrus"
)

type Conn interface {
	// Env returns the connection environment
	Env() Env

	// Conn returns the underlying net.Conn
	Conn() net.Conn

	// Request sends an API request for the given API number and expects a response.
	// Both request and response should represent a type such as in `msg/types.go`.
	// The request and response will be marshaled and unmarshaled automatically.
	// If a negative IntInfo is returned, an appropriate error will be returned.
	Request(ctx context.Context, apiNumber msg.APINumber, request, response any) error

	// RequestWithBuffers behaves as Request, with provided buffers for the request
	// and response binary data. Both requestBuf and responseBuf could be nil.
	RequestWithBuffers(ctx context.Context, apiNumber msg.APINumber, request, response any, requestBuf, responseBuf []byte) error

	// API returns an API for the given resource.
	API() *api.API

	// Close closes the connection.
	Close() error
}

type conn struct {
	transport       net.Conn
	env             *Env
	option          string
	protocol        msg.Protocol
	UseTLS          bool
	Version         *msg.Version
	ClientSignature string
	NativePassword  string // Only used for non-native authentication
	closeOnce       sync.Once
	doRequest       sync.Mutex
}

// Dialer is used to connect to an IRODS server.
var Dialer = net.Dialer{
	Timeout: time.Minute,
}

// Dial connects to an IRODS server and creates a new connection.
// The caller is responsible for closing the connection when it is no longer needed.
func Dial(ctx context.Context, env Env, clientName string) (Conn, error) {
	return dial(ctx, env, clientName, msg.XML)
}

func dial(ctx context.Context, env Env, clientName string, protocol msg.Protocol) (*conn, error) {
	conn, err := Dialer.DialContext(ctx, "tcp", net.JoinHostPort(env.Host, strconv.FormatInt(int64(env.Port), 10)))
	if err != nil {
		return nil, err
	}

	return newConn(ctx, conn, env, clientName, protocol)
}

var HandshakeTimeout = time.Minute

// NewConn initializes a new Conn instance with the provided network connection and environment settings.
// It performs a handshake as part of the initialization process and returns the constructed Conn instance.
// Returns an error if the handshake fails.
func NewConn(ctx context.Context, transport net.Conn, env Env, clientName string) (Conn, error) {
	return newConn(ctx, transport, env, clientName, msg.XML)
}

func newConn(ctx context.Context, transport net.Conn, env Env, clientName string, protocol msg.Protocol) (*conn, error) {
	c := &conn{
		transport: transport,
		env:       &env,
		option:    clientName,
		protocol:  protocol,
	}

	// Make sure TLS is required when not using native authentication
	if c.env.AuthScheme != native {
		if c.env.ClientServerNegotiation != "request_server_negotiation" {
			return nil, ErrTLSRequired
		}

		if c.env.ClientServerNegotiationPolicy == ClientServerRefuseTLS {
			return nil, ErrTLSRequired
		}

		c.env.ClientServerNegotiationPolicy = ClientServerRequireTLS
	}

	ctx, cancel := context.WithTimeout(ctx, HandshakeTimeout)

	defer cancel()

	return c, c.Handshake(ctx)
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

// Handshake performs a handshake with the IRODS server.
func (c *conn) Handshake(ctx context.Context) error {
	if err := c.startup(ctx); err != nil {
		return err
	}

	return c.authenticate(ctx)
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
		Option:         fmt.Sprintf("%s;%s", c.option, c.env.ClientServerNegotiation),
	}

	if err := msg.Write(c.transport, pack, nil, msg.XML, "RODS_CONNECT", 0); err != nil {
		return err
	}

	if c.env.ClientServerNegotiation == "request_server_negotiation" {
		if err := c.handshakeNegotiation(); err != nil {
			return err
		}
	}

	version := msg.Version{}

	if _, err := msg.Read(c.transport, &version, nil, msg.XML, "RODS_VERSION"); err != nil {
		return err
	}

	if !checkVersion(version) {
		return fmt.Errorf("%w: server version %v", ErrUnsupportedVersion, version.ReleaseVersion)
	}

	c.Version = &version

	if !c.UseTLS {
		return nil
	}

	return c.handshakeTLS()
}

// checkVersion returns true if the server version is greater than or equal to 4.3.2
func checkVersion(version msg.Version) bool {
	parts := strings.Split(version.ReleaseVersion[4:], ".")

	if len(parts) != 3 {
		return false
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return false
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return false
	}

	release, err := strconv.Atoi(parts[2])
	if err != nil {
		return false
	}

	return major > 4 || (major == 4 && minor > 3) || (major == 4 && minor == 3 && release > 1)
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
		c.UseTLS = true
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

func (c *conn) authenticate(ctx context.Context) error {
	if c.env.AuthScheme == pam {
		if err := c.authenticatePAM(ctx); err != nil {
			return err
		}
	} else if c.env.AuthScheme != native {
		return fmt.Errorf("%w: authentication scheme %s", ErrNotImplemented, c.env.AuthScheme)
	}

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
	c.ClientSignature = hex.EncodeToString(challengeBytes[:min(16, len(challengeBytes))])

	// Create challenge response
	myPassword := c.env.Password

	if c.env.AuthScheme != native {
		myPassword = c.NativePassword
	}

	response := msg.AuthChallengeResponse{
		Response: GenerateAuthResponse(challengeBytes, myPassword),
		Username: c.env.ProxyUsername,
	}

	logrus.Debugf("Responding %s %s ", response.Response, response.Username)

	return c.Request(ctx, msg.AUTH_RESPONSE_AN, response, &msg.AuthResponse{})
}

func (c *conn) authenticatePAM(ctx context.Context) error {
	request := msg.PamAuthRequest{
		Username: c.env.ProxyUsername,
		Password: c.env.Password,
		TTL:      c.env.PamTTL,
	}

	response := msg.PamAuthResponse{}

	if err := c.Request(ctx, msg.PAM_AUTH_REQUEST_AN, request, &response); err != nil {
		return err
	}

	c.NativePassword = response.GeneratedPassword

	return nil
}

const (
	maxPasswordLength int = 50
	challengeLen      int = 64
	authResponseLen   int = 16
)

// GenerateAuthResponse generates an authentication response using the given
// challenge and password. The response is the MD5 hash of the first 64 bytes
// of the challenge and the padded password (padded to maxPasswordLength).
// The first 16 bytes of the hash are then base64 encoded to produce the
// final response string.
func GenerateAuthResponse(challenge []byte, password string) string {
	paddedPassword := make([]byte, maxPasswordLength)
	copy(paddedPassword, password)

	m := md5.New() //nolint:gosec
	m.Write(challenge[:64])
	m.Write(paddedPassword)
	encodedPassword := m.Sum(nil)

	// replace 0x00 to 0x01
	for idx := 0; idx < len(encodedPassword); idx++ {
		if encodedPassword[idx] == 0 {
			encodedPassword[idx] = 1
		}
	}

	return base64.StdEncoding.EncodeToString(encodedPassword[:authResponseLen])
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
		return err
	}

	m := msg.Message{
		Bin: responseBuf,
	}

	if err := m.Read(c.transport); err != nil {
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
		err := &msg.IRODSError{
			Code:    m.Header.IntInfo,
			Message: string(m.Body.Error),
		}

		if m.Header.ErrorLen == 0 {
			err.Message = string(m.Body.Message)
		}

		return err
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

func (c *conn) handleCollStat(response any, responseBuf []byte) error {
	// Send special code
	replyBuffer := make([]byte, 4)
	binary.BigEndian.PutUint32(replyBuffer, uint32(msg.SYS_CLI_TO_SVR_COLL_STAT_REPLY))

	if _, err := c.transport.Write(replyBuffer); err != nil {
		return err
	}

	m := msg.Message{
		Bin: responseBuf,
	}

	if err := m.Read(c.transport); err != nil {
		return err
	}

	if expectedMsgType := "RODS_API_REPLY"; m.Header.Type != expectedMsgType {
		return fmt.Errorf("%w: expected %s, got %s", msg.ErrUnexpectedMessage, expectedMsgType, m.Header.Type)
	}

	if m.Header.IntInfo < 0 {
		err := &msg.IRODSError{
			Code:    m.Header.IntInfo,
			Message: string(m.Body.Error),
		}

		if m.Header.ErrorLen == 0 {
			err.Message = string(m.Body.Message)
		}

		return err
	}

	return msg.Unmarshal(m, c.protocol, response)
}

func (c *conn) Close() error {
	var err error

	c.closeOnce.Do(func() {
		c.doRequest.Lock()

		defer c.doRequest.Unlock()

		err = msg.Write(c.transport, msg.EmptyResponse{}, nil, c.protocol, "RODS_DISCONNECT", 0)
	})

	err = multierr.Append(err, c.Conn().Close())

	return err
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
