package iron

import (
	"crypto/tls"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/acomagu/bufpipe"
	"github.com/kuleuven/iron/msg"
)

type mockConn struct {
	io.Reader
	io.Writer
	version string
}

func (c *mockConn) Close() error {
	return nil
}

func (c *mockConn) ServerVersion() string {
	return c.version
}

func (c *mockConn) ClientSignature() string {
	return "signature"
}

func (c *mockConn) NativePassword() string {
	return "password"
}

func (c *mockConn) LocalAddr() net.Addr {
	return nil
}

func (c *mockConn) RemoteAddr() net.Addr {
	return nil
}

func (c *mockConn) SetDeadline(t time.Time) error {
	return nil
}

func (c *mockConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (c *mockConn) SetWriteDeadline(t time.Time) error {
	return nil
}

func connPipe(version string) (net.Conn, net.Conn) {
	r, w := bufpipe.New(nil)
	R, W := bufpipe.New(nil)

	return &mockConn{r, W, version}, &mockConn{R, w, version}
}

const mockVersion = "4.3.2"

const releaseVersion = "rods" + mockVersion

const mockVersionNew = "5.0.0"

const releaseVersionNew = "rods" + mockVersionNew

func TestConnNative(t *testing.T) {
	ctx := t.Context()
	transport, server := connPipe(mockVersion)

	msg.Write(server, msg.ClientServerNegotiation{
		Result: "CS_NEG_DONT_CARE",
	}, nil, msg.XML, "RODS_CS_NEG_T", 0)

	msg.Write(server, msg.Version{
		ReleaseVersion: releaseVersion,
	}, nil, msg.XML, "RODS_VERSION", 0)

	msg.Write(server, msg.AuthChallenge{
		Challenge: base64.StdEncoding.EncodeToString([]byte("testChallengetestChallengetestChallengetestChallengetestChallenge")),
	}, nil, msg.XML, "RODS_API_REPLY", 0)

	msg.Write(server, msg.AuthResponse{}, nil, msg.XML, "RODS_API_REPLY", 0)

	env := Env{
		Host:                          "localhost",
		Port:                          1247,
		Zone:                          "testZone",
		Username:                      "testUser",
		Password:                      "testPassword",
		AuthScheme:                    "native",
		ClientServerNegotiationPolicy: "CS_NEG_REFUSE",
	}

	env.ApplyDefaults()

	conn, err := NewConn(ctx, transport, env, "test")
	if err != nil {
		t.Fatal(err)
	}

	if conn.ServerVersion() != mockVersion {
		t.Errorf("bad server version: %s", conn.ServerVersion())
	}

	err = conn.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func TestConnNativeNew(t *testing.T) {
	ctx := t.Context()
	transport, server := connPipe(mockVersionNew)

	msg.Write(server, msg.ClientServerNegotiation{
		Result: "CS_NEG_DONT_CARE",
	}, nil, msg.XML, "RODS_CS_NEG_T", 0)

	msg.Write(server, msg.Version{
		ReleaseVersion: releaseVersionNew,
	}, nil, msg.XML, "RODS_VERSION", 0)

	msg.Write(server, msg.AuthPluginResponse{
		NextOperation: "auth_agent_auth_request",
		RequestResult: base64.StdEncoding.EncodeToString([]byte("testChallengetestChallengetestChallengetestChallengetestChallenge")),
	}, nil, msg.XML, "RODS_API_REPLY", 0)

	msg.Write(server, msg.AuthPluginResponse{
		NextOperation: "auth_agent_auth_response",
	}, nil, msg.XML, "RODS_API_REPLY", 0)

	env := Env{
		Host:                          "localhost",
		Port:                          1247,
		Zone:                          "testZone",
		Username:                      "testUser",
		Password:                      "testPassword",
		AuthScheme:                    "native",
		ClientServerNegotiationPolicy: "CS_NEG_REFUSE",
	}

	env.ApplyDefaults()

	conn, err := NewConn(ctx, transport, env, "test")
	if err != nil {
		t.Fatal(err)
	}

	if conn.ServerVersion() != mockVersionNew {
		t.Errorf("bad server version: %s", conn.ServerVersion())
	}

	err = conn.Close()
	if err != nil {
		t.Fatal(err)
	}
}

var (
	certPem = []byte(`-----BEGIN CERTIFICATE-----
MIIBhTCCASugAwIBAgIQIRi6zePL6mKjOipn+dNuaTAKBggqhkjOPQQDAjASMRAw
DgYDVQQKEwdBY21lIENvMB4XDTE3MTAyMDE5NDMwNloXDTE4MTAyMDE5NDMwNlow
EjEQMA4GA1UEChMHQWNtZSBDbzBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABD0d
7VNhbWvZLWPuj/RtHFjvtJBEwOkhbN/BnnE8rnZR8+sbwnc/KhCk3FhnpHZnQz7B
5aETbbIgmuvewdjvSBSjYzBhMA4GA1UdDwEB/wQEAwICpDATBgNVHSUEDDAKBggr
BgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MCkGA1UdEQQiMCCCDmxvY2FsaG9zdDo1
NDUzgg4xMjcuMC4wLjE6NTQ1MzAKBggqhkjOPQQDAgNIADBFAiEA2zpJEPQyz6/l
Wf86aX6PepsntZv2GYlA5UpabfT2EZICICpJ5h/iI+i341gBmLiAFQOyTDT+/wQc
6MF9+Yw1Yy0t
-----END CERTIFICATE-----`)
	keyPem = []byte(`-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIIrYSSNQFaA2Hwf1duRSxKtLYX5CB04fSeQ6tF1aY/PuoAoGCCqGSM49
AwEHoUQDQgAEPR3tU2Fta9ktY+6P9G0cWO+0kETA6SFs38GecTyudlHz6xvCdz8q
EKTcWGekdmdDPsHloRNtsiCa697B2O9IFA==
-----END EC PRIVATE KEY-----`)
)

func pamResponses(server net.Conn) {
	assert := func(args ...any) {
		if err := args[len(args)-1]; err != nil {
			panic(err)
		}
	}

	assert(msg.Read(server, &msg.StartupPack{}, nil, msg.XML, "RODS_CONNECT"))
	assert(msg.Write(server, msg.ClientServerNegotiation{
		Result: "CS_NEQ_REQUIRE",
	}, nil, msg.XML, "RODS_CS_NEG_T", 0))
	assert(msg.Read(server, &msg.ClientServerNegotiation{}, nil, msg.XML, "RODS_CS_NEG_T"))
	assert(msg.Write(server, msg.Version{
		ReleaseVersion: releaseVersion,
	}, nil, msg.XML, "RODS_VERSION", 0))

	// Switch to TLS
	cert, err := tls.X509KeyPair(certPem, keyPem)
	assert(err)

	serverTLS := tls.Server(server, &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{cert},
	})

	defer serverTLS.Close()

	assert((&msg.Header{}).Read(serverTLS))
	assert(msg.Read(serverTLS, &msg.SSLSharedSecret{}, nil, msg.XML, "SHARED_SECRET"))
	assert(msg.Read(serverTLS, &msg.PamAuthRequest{}, nil, msg.XML, "RODS_API_REQ"))
	assert(msg.Write(serverTLS, msg.PamAuthResponse{
		GeneratedPassword: "testNativePassword",
	}, nil, msg.XML, "RODS_API_REPLY", 0))
	assert(msg.Read(serverTLS, &msg.AuthRequest{}, nil, msg.XML, "RODS_API_REQ"))
	assert(msg.Write(serverTLS, msg.AuthChallenge{
		Challenge: base64.StdEncoding.EncodeToString([]byte("testChallengetestChallengetestChallengetestChallengetestChallenge")),
	}, nil, msg.XML, "RODS_API_REPLY", 0))
	assert(msg.Read(serverTLS, &msg.AuthChallengeResponse{}, nil, msg.XML, "RODS_API_REQ"))
	assert(msg.Write(serverTLS, msg.AuthResponse{}, nil, msg.XML, "RODS_API_REPLY", 0))
}

func pamResponsesNew(server net.Conn) {
	assert := func(args ...any) {
		if err := args[len(args)-1]; err != nil {
			panic(err)
		}
	}

	assert(msg.Read(server, &msg.StartupPack{}, nil, msg.XML, "RODS_CONNECT"))
	assert(msg.Write(server, msg.ClientServerNegotiation{
		Result: "CS_NEQ_REQUIRE",
	}, nil, msg.XML, "RODS_CS_NEG_T", 0))
	assert(msg.Read(server, &msg.ClientServerNegotiation{}, nil, msg.XML, "RODS_CS_NEG_T"))
	assert(msg.Write(server, msg.Version{
		ReleaseVersion: releaseVersionNew,
	}, nil, msg.XML, "RODS_VERSION", 0))

	// Switch to TLS
	cert, err := tls.X509KeyPair(certPem, keyPem)
	assert(err)

	serverTLS := tls.Server(server, &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{cert},
	})

	defer serverTLS.Close()

	assert((&msg.Header{}).Read(serverTLS))
	assert(msg.Read(serverTLS, &msg.SSLSharedSecret{}, nil, msg.XML, "SHARED_SECRET"))
	assert(msg.Read(serverTLS, &msg.AuthPluginRequest{}, nil, msg.XML, "RODS_API_REQ"))
	assert(msg.Write(serverTLS, msg.AuthPluginResponse{
		RequestResult: "testNativePassword",
	}, nil, msg.XML, "RODS_API_REPLY", 0))
	assert(msg.Read(serverTLS, &msg.AuthPluginRequest{}, nil, msg.XML, "RODS_API_REQ"))
	assert(msg.Write(serverTLS, msg.AuthPluginResponse{
		RequestResult: base64.StdEncoding.EncodeToString([]byte("testChallengetestChallengetestChallengetestChallengetestChallenge")),
	}, nil, msg.XML, "RODS_API_REPLY", 0))
	assert(msg.Read(serverTLS, &msg.AuthPluginRequest{}, nil, msg.XML, "RODS_API_REQ"))
	assert(msg.Write(serverTLS, msg.AuthPluginResponse{}, nil, msg.XML, "RODS_API_REPLY", 0))
}

func TestConnPamPassword(t *testing.T) {
	ctx := t.Context()
	transport, server := connPipe(mockVersion)

	go pamResponses(server)

	env := Env{
		Zone:            "testZone",
		Username:        "testUser",
		Password:        "testPassword",
		AuthScheme:      "pam_password",
		SSLVerifyServer: "none",
	}

	env.ApplyDefaults()

	conn, err := NewConn(ctx, transport, env, "test")
	if err != nil {
		t.Fatal(err)
	}

	err = conn.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func TestConnPamPasswordTLS(t *testing.T) {
	ctx := t.Context()
	transport, server := connPipe(mockVersion)

	go pamResponses(server)

	f, err := os.CreateTemp(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}

	defer os.Remove(f.Name())

	if _, err = f.Write(certPem); err != nil {
		t.Fatal(err)
	}

	if err = f.Close(); err != nil {
		t.Fatal(err)
	}

	env := Env{
		SSLServerName:        "localhost",
		Zone:                 "testZone",
		Username:             "testUser",
		Password:             "testPassword",
		AuthScheme:           "pam_password",
		SSLVerifyServer:      "host",
		SSLCACertificateFile: f.Name(),
	}

	env.ApplyDefaults()

	tlsTime = func() time.Time {
		return time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC)
	}

	conn, err := NewConn(ctx, transport, env, "test")
	if err != nil {
		t.Fatal(err)
	}

	err = conn.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func TestConnPamPasswordTLS2(t *testing.T) {
	ctx := t.Context()
	transport, server := connPipe(mockVersion)

	go pamResponses(server)

	f, err := os.CreateTemp(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}

	defer os.Remove(f.Name())

	if _, err = f.Write(certPem); err != nil {
		t.Fatal(err)
	}

	if err = f.Close(); err != nil {
		t.Fatal(err)
	}

	env := Env{
		SSLServerName:        "localhost:5453",
		Zone:                 "testZone",
		Username:             "testUser",
		Password:             "testPassword",
		AuthScheme:           "pam_password",
		SSLVerifyServer:      "cert",
		SSLCACertificateFile: f.Name(),
	}

	env.ApplyDefaults()

	tlsTime = func() time.Time {
		return time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC)
	}

	conn, err := NewConn(ctx, transport, env, "test")
	if err != nil {
		t.Fatal(err)
	}

	err = conn.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func TestConnPamPasswordTLS3(t *testing.T) {
	ctx := t.Context()
	transport, server := connPipe(mockVersionNew)

	go pamResponsesNew(server)

	f, err := os.CreateTemp(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}

	defer os.Remove(f.Name())

	if _, err = f.Write(certPem); err != nil {
		t.Fatal(err)
	}

	if err = f.Close(); err != nil {
		t.Fatal(err)
	}

	env := Env{
		SSLServerName:        "localhost:5453",
		Zone:                 "testZone",
		Username:             "testUser",
		Password:             "testPassword",
		AuthScheme:           "pam_password",
		SSLVerifyServer:      "cert",
		SSLCACertificateFile: f.Name(),
	}

	env.ApplyDefaults()

	tlsTime = func() time.Time {
		return time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC)
	}

	conn, err := NewConn(ctx, transport, env, "test")
	if err != nil {
		t.Fatal(err)
	}

	err = conn.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func pamResponsesInteractive(server net.Conn) {
	assert := func(args ...any) {
		if err := args[len(args)-1]; err != nil {
			panic(err)
		}
	}

	assert(msg.Read(server, &msg.StartupPack{}, nil, msg.XML, "RODS_CONNECT"))
	assert(msg.Write(server, msg.ClientServerNegotiation{
		Result: "CS_NEQ_REQUIRE",
	}, nil, msg.XML, "RODS_CS_NEG_T", 0))
	assert(msg.Read(server, &msg.ClientServerNegotiation{}, nil, msg.XML, "RODS_CS_NEG_T"))
	assert(msg.Write(server, msg.Version{
		ReleaseVersion: releaseVersionNew,
	}, nil, msg.XML, "RODS_VERSION", 0))

	// Switch to TLS
	cert, err := tls.X509KeyPair(certPem, keyPem)
	assert(err)

	serverTLS := tls.Server(server, &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{cert},
	})

	defer serverTLS.Close()

	assert((&msg.Header{}).Read(serverTLS))
	assert(msg.Read(serverTLS, &msg.SSLSharedSecret{}, nil, msg.XML, "SHARED_SECRET"))
	assert(msg.Read(serverTLS, &msg.AuthPluginRequest{}, nil, msg.XML, "RODS_API_REQ"))
	assert(msg.Write(serverTLS, msg.AuthPluginResponse{
		NextOperation: "next",
		Message: struct {
			Prompt      string           "json:\"prompt,omitempty\""
			Retrieve    string           "json:\"retrieve,omitempty\""
			DefaultPath string           "json:\"default_path,omitempty\""
			Patch       []map[string]any "json:\"patch,omitempty\""
		}{
			Prompt: "Hello",
			Patch: []map[string]any{
				{
					"op":    "replace",
					"path":  "/bla",
					"value": "testValue",
				},
			},
		},
	}, nil, msg.XML, "RODS_API_REPLY", 0))
	assert(msg.Read(serverTLS, &msg.AuthPluginRequest{}, nil, msg.XML, "RODS_API_REQ"))
	assert(msg.Write(serverTLS, msg.AuthPluginResponse{
		NextOperation: "waiting",
		Message: struct {
			Prompt      string           "json:\"prompt,omitempty\""
			Retrieve    string           "json:\"retrieve,omitempty\""
			DefaultPath string           "json:\"default_path,omitempty\""
			Patch       []map[string]any "json:\"patch,omitempty\""
		}{
			Retrieve: "/bla",
		},
	}, nil, msg.XML, "RODS_API_REPLY", 0))
	assert(msg.Read(serverTLS, &msg.AuthPluginRequest{}, nil, msg.XML, "RODS_API_REQ"))
	assert(msg.Write(serverTLS, msg.AuthPluginResponse{
		NextOperation: "authenticated",
		RequestResult: "testNativePassword",
	}, nil, msg.XML, "RODS_API_REPLY", 0))
	assert(msg.Read(serverTLS, &msg.AuthPluginRequest{}, nil, msg.XML, "RODS_API_REQ"))
	assert(msg.Write(serverTLS, msg.AuthPluginResponse{
		RequestResult: base64.StdEncoding.EncodeToString([]byte("testChallengetestChallengetestChallengetestChallengetestChallenge")),
	}, nil, msg.XML, "RODS_API_REPLY", 0))
	assert(msg.Read(serverTLS, &msg.AuthPluginRequest{}, nil, msg.XML, "RODS_API_REQ"))
	assert(msg.Write(serverTLS, msg.AuthPluginResponse{}, nil, msg.XML, "RODS_API_REPLY", 0))
}

func TestConnPamInteractiveTLS(t *testing.T) {
	ctx := t.Context()
	transport, server := connPipe(mockVersionNew)

	go pamResponsesInteractive(server)

	f, err := os.CreateTemp(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}

	defer os.Remove(f.Name())

	if _, err = f.Write(certPem); err != nil {
		t.Fatal(err)
	}

	if err = f.Close(); err != nil {
		t.Fatal(err)
	}

	env := Env{
		SSLServerName:        "localhost:5453",
		Zone:                 "testZone",
		Username:             "testUser",
		AuthScheme:           "pam_interactive",
		SSLVerifyServer:      "cert",
		SSLCACertificateFile: f.Name(),
	}

	env.ApplyDefaults()

	tlsTime = func() time.Time {
		return time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC)
	}

	conn, err := NewConn(ctx, transport, env, "test")
	if err != nil {
		t.Fatal(err)
	}

	err = conn.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func TestDialer(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			t.Error(err)
		}

		// Consume startup message
		_, err = msg.Read(conn, &msg.StartupPack{}, nil, msg.XML, "RODS_CONNECT")
		if err != nil {
			t.Error(err)
		}

		conn.Close()
	}()

	tcpAddr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("expected TCP address, got %T", listener.Addr())
	}

	env := Env{Host: "127.0.0.1", Port: tcpAddr.Port}

	env.ApplyDefaults()

	_, err = Dial(t.Context(), env, "test")
	if err != io.EOF {
		t.Fatalf("expected EOF, got %v", err)
	}
}

func TestTLSRequired1(t *testing.T) {
	env := Env{
		AuthScheme:                    "pam_password",
		ClientServerNegotiationPolicy: "CS_NEG_REFUSE",
	}

	env.ApplyDefaults()

	_, err := NewConn(t.Context(), nil, env, "test")
	if err != ErrTLSRequired {
		t.Fatalf("expected ErrTLSRequired, got %v", err)
	}
}

func TestTLSRequired2(t *testing.T) {
	env := Env{
		AuthScheme:              "pam_password",
		ClientServerNegotiation: "dont_negotiate",
	}

	env.ApplyDefaults()

	_, err := NewConn(t.Context(), nil, env, "test")
	if err != ErrTLSRequired {
		t.Fatalf("expected ErrTLSRequired, got %v", err)
	}
}

func TestOldVersion(t *testing.T) {
	ctx := t.Context()
	transport, server := connPipe(mockVersion)

	msg.Write(server, msg.ClientServerNegotiation{
		Result: "CS_NEG_DONT_CARE",
	}, nil, msg.XML, "RODS_CS_NEG_T", 0)

	msg.Write(server, msg.Version{
		ReleaseVersion: "rods4.2.9",
	}, nil, msg.XML, "RODS_VERSION", 0)

	env := Env{
		Host:                          "localhost",
		Port:                          1247,
		Zone:                          "testZone",
		Username:                      "testUser",
		Password:                      "testPassword",
		AuthScheme:                    "native",
		ClientServerNegotiationPolicy: "CS_NEG_REFUSE",
	}

	env.ApplyDefaults()

	_, err := NewConn(ctx, transport, env, "test")
	if !errors.Is(err, ErrUnsupportedVersion) {
		t.Fatalf("expected ErrUnsupportedVersion, got %v", err)
	}
}

func TestRequest(t *testing.T) { //nolint:funlen
	ctx := t.Context()
	transport, server := connPipe(mockVersion)

	msg.Write(server, msg.Version{
		ReleaseVersion: releaseVersion,
	}, nil, msg.XML, "RODS_VERSION", 0)

	msg.Write(server, msg.AuthChallenge{
		Challenge: base64.StdEncoding.EncodeToString([]byte("testChallengetestChallengetestChallengetestChallengetestChallenge")),
	}, nil, msg.XML, "RODS_API_REPLY", 0)

	msg.Write(server, msg.AuthResponse{}, nil, msg.XML, "RODS_API_REPLY", 0)

	msg.Write(server, msg.EmptyResponse{}, nil, msg.XML, "RODS_API_REPLY", msg.SYS_SVR_TO_CLI_COLL_STAT)

	msg.Write(server, msg.CollectionOperationStat{}, nil, msg.XML, "RODS_API_REPLY", 0)

	env := Env{
		Host:                    "localhost",
		Port:                    1247,
		Zone:                    "testZone",
		Username:                "testUser",
		Password:                "testPassword",
		AuthScheme:              "native",
		ClientServerNegotiation: "dont_negotiate",
	}

	env.ApplyDefaults()

	conn, err := NewConn(ctx, transport, env, "test")
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(conn.Env(), env) {
		t.Error(err)
	}

	var called int

	conn.RegisterCloseHandler(func() error {
		called++

		return nil
	})

	err = conn.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = conn.Request(ctx, msg.RM_COLL_AN, msg.CreateCollectionRequest{
		Name: "testColl",
	}, &msg.CollectionOperationStat{})
	if err != nil {
		t.Fatalf("expected error, got nil")
	}

	if called != 1 {
		t.Fatalf("expected 1, got %d", called)
	}
}
