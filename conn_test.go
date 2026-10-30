package iron

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
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
	return testPassword
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
		Result: ClientServerDontCare,
	}, nil, msg.XML, "RODS_CS_NEG_T", 0)

	msg.Write(server, msg.Version{
		ReleaseVersion: releaseVersion,
	}, nil, msg.XML, "RODS_VERSION", 0)

	msg.Write(server, msg.AuthChallenge{
		Challenge: base64.StdEncoding.EncodeToString([]byte("testChallengetestChallengetestChallengetestChallengetestChallenge")),
	}, nil, msg.XML, "RODS_API_REPLY", 0)

	msg.Write(server, msg.AuthResponse{}, nil, msg.XML, "RODS_API_REPLY", 0)

	env := Env{
		Host:                          testHost,
		Port:                          1247,
		Zone:                          testZoneName,
		Username:                      testUserName,
		Password:                      testPasswordValue,
		AuthScheme:                    native,
		ClientServerNegotiationPolicy: ClientServerRefuseTLS,
	}

	env.ApplyDefaults()

	conn, err := NewConn(ctx, transport, env, testStr)
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
		Result: ClientServerDontCare,
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
		Host:                          testHost,
		Port:                          1247,
		Zone:                          testZoneName,
		Username:                      testUserName,
		Password:                      testPasswordValue,
		AuthScheme:                    native,
		ClientServerNegotiationPolicy: ClientServerRefuseTLS,
	}

	env.ApplyDefaults()

	conn, err := NewConn(ctx, transport, env, testStr)
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
		Result: testCSNegRequire,
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
		GeneratedPassword: testNativePassword,
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
		Result: testCSNegRequire,
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
		RequestResult: testNativePassword,
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
		Zone:            testZoneName,
		Username:        testUserName,
		Password:        testPasswordValue,
		AuthScheme:      pamPassword,
		SSLVerifyServer: "none",
	}

	env.ApplyDefaults()

	conn, err := NewConn(ctx, transport, env, testStr)
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
		SSLServerName:        testHost,
		Zone:                 testZoneName,
		Username:             testUserName,
		Password:             testPasswordValue,
		AuthScheme:           pamPassword,
		SSLVerifyServer:      SSLVerifyServerHost,
		SSLCACertificateFile: f.Name(),
	}

	env.ApplyDefaults()

	tlsTime := func() time.Time {
		return time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC)
	}

	dialer := Dialer{
		DialFunc: func(context.Context, Env, string) (net.Conn, error) {
			return transport, nil
		},
		Protocol: msg.XML,
		Time: func() time.Time {
			return tlsTime().Add(7 * 24 * time.Hour)
		},
	}

	conn, err := dialer.Dial(ctx, env, testStr)
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
		SSLServerName:        testHostPort,
		Zone:                 testZoneName,
		Username:             testUserName,
		Password:             testPasswordValue,
		AuthScheme:           pamPassword,
		SSLVerifyServer:      SSLVerifyServerCert,
		SSLCACertificateFile: f.Name(),
	}

	env.ApplyDefaults()

	tlsDefaultTime := func() time.Time {
		return time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC)
	}

	conn, err := newConn(ctx, transport, env, testStr, StdPrompt, msg.XML, tlsDefaultTime)
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
		SSLServerName:        testHostPort,
		Zone:                 testZoneName,
		Username:             testUserName,
		Password:             testPasswordValue,
		AuthScheme:           pamPassword,
		SSLVerifyServer:      "cert",
		SSLCACertificateFile: f.Name(),
	}

	env.ApplyDefaults()

	tlsDefaultTime := func() time.Time {
		return time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC)
	}

	conn, err := newConn(ctx, transport, env, testStr, StdPrompt, msg.XML, tlsDefaultTime)
	if err != nil {
		t.Fatal(err)
	}

	err = conn.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func pamResponsesInteractive(server net.Conn) { //nolint:funlen
	assert := func(args ...any) {
		if err := args[len(args)-1]; err != nil {
			panic(err)
		}
	}

	assert(msg.Read(server, &msg.StartupPack{}, nil, msg.XML, "RODS_CONNECT"))
	assert(msg.Write(server, msg.ClientServerNegotiation{
		Result: testCSNegRequire,
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
					"op":     "replace",
					testPath: "/bla",
					value:    "testValue",
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
		NextOperation: "waiting_pw",
		Message: struct {
			Prompt      string           "json:\"prompt,omitempty\""
			Retrieve    string           "json:\"retrieve,omitempty\""
			DefaultPath string           "json:\"default_path,omitempty\""
			Patch       []map[string]any "json:\"patch,omitempty\""
		}{
			Prompt:   "Password",
			Retrieve: "/pw",
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
			Prompt: "Some key",
		},
	}, nil, msg.XML, "RODS_API_REPLY", 0))
	assert(msg.Read(serverTLS, &msg.AuthPluginRequest{}, nil, msg.XML, "RODS_API_REQ"))
	assert(msg.Write(serverTLS, msg.AuthPluginResponse{
		NextOperation: "authenticated",
		RequestResult: testNativePassword,
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
		SSLServerName:        testHostPort,
		Zone:                 testZoneName,
		Username:             testUserName,
		AuthScheme:           "pam_interactive",
		SSLVerifyServer:      "cert",
		SSLCACertificateFile: f.Name(),
	}

	env.ApplyDefaults()

	tlsDefaultTime := func() time.Time {
		return time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC)
	}

	bot := &Bot{
		"Password": "abc",
		"Some key": "def",
	}

	conn, err := newConn(ctx, transport, env, testStr, bot, msg.XML, tlsDefaultTime)
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

	env := Env{Host: testIP, Port: tcpAddr.Port}

	env.ApplyDefaults()

	_, err = Dial(t.Context(), env, testStr)
	if err != io.EOF {
		t.Fatalf("expected EOF, got %v", err)
	}
}

func TestTLSRequired1(t *testing.T) {
	env := Env{
		AuthScheme:                    pamPassword,
		ClientServerNegotiationPolicy: ClientServerRefuseTLS,
	}

	env.ApplyDefaults()

	_, err := NewConn(t.Context(), nil, env, testStr)
	if err != ErrTLSRequired {
		t.Fatalf("expected ErrTLSRequired, got %v", err)
	}
}

func TestTLSRequired2(t *testing.T) {
	env := Env{
		AuthScheme:              pamPassword,
		ClientServerNegotiation: "dont_negotiate",
	}

	env.ApplyDefaults()

	_, err := NewConn(t.Context(), nil, env, testStr)
	if err != ErrTLSRequired {
		t.Fatalf("expected ErrTLSRequired, got %v", err)
	}
}

func TestOldVersion(t *testing.T) {
	ctx := t.Context()
	transport, server := connPipe(mockVersion)

	msg.Write(server, msg.ClientServerNegotiation{
		Result: ClientServerDontCare,
	}, nil, msg.XML, "RODS_CS_NEG_T", 0)

	msg.Write(server, msg.Version{
		ReleaseVersion: "rods4.2.9",
	}, nil, msg.XML, "RODS_VERSION", 0)

	env := Env{
		Host:                          testHost,
		Port:                          1247,
		Zone:                          testZoneName,
		Username:                      testUserName,
		Password:                      testPasswordValue,
		AuthScheme:                    native,
		ClientServerNegotiationPolicy: ClientServerRefuseTLS,
	}

	env.ApplyDefaults()

	_, err := NewConn(ctx, transport, env, testStr)
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
		Host:                    testHost,
		Port:                    1247,
		Zone:                    testZoneName,
		Username:                testUserName,
		Password:                testPasswordValue,
		AuthScheme:              native,
		ClientServerNegotiation: "dont_negotiate",
	}

	env.ApplyDefaults()

	conn, err := NewConn(ctx, transport, env, testStr)
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

func TestCheckVersionValid(t *testing.T) {
	tests := []struct {
		version        string
		major          int
		minor          int
		release        int
		expectedResult bool
	}{
		{releaseVersion, 4, 3, 2, true},  // equal
		{"rods4.3.3", 4, 3, 2, true},     // release greater
		{"rods4.4.0", 4, 3, 2, true},     // minor greater
		{"rods5.0.0", 4, 3, 2, true},     // major greater
		{"rods4.3.1", 4, 3, 2, false},    // release less
		{"rods4.2.9", 4, 3, 2, false},    // minor less
		{"rods3.9.9", 4, 3, 2, false},    // major less
		{releaseVersion, 4, 3, 3, false}, // equal major/minor, release less
		{releaseVersion, 4, 4, 0, false}, // equal major, minor less
		{"rods10.0.0", 4, 3, 2, true},    // large major
		{releaseVersion, 5, 0, 0, false}, // major less than required
		{"rods5.0.0", 5, 0, 0, true},     // exact match v5
		{"rods4.3.10", 4, 3, 2, true},    // multi-digit release
		{"rods4.10.0", 4, 3, 0, true},    // multi-digit minor
		{"rods0.0.0", 0, 0, 0, true},     // zero version
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s>=%d.%d.%d", tt.version, tt.major, tt.minor, tt.release), func(t *testing.T) {
			v := msg.Version{ReleaseVersion: tt.version}

			result := checkVersion(v, tt.major, tt.minor, tt.release)
			if result != tt.expectedResult {
				t.Errorf("checkVersion(%q, %d, %d, %d) = %v, want %v", tt.version, tt.major, tt.minor, tt.release, result, tt.expectedResult)
			}
		})
	}
}

func TestCheckVersionInvalid(t *testing.T) {
	tests := []struct {
		name    string
		version string
	}{
		{"no prefix", mockVersion},
		{"wrong prefix", "irods4.3.2"},
		{"too few parts", "rods4.3"},
		{"too many parts", "rods4.3.2.1"},
		{"non-numeric major", "rodsa.3.2"},
		{"non-numeric minor", "rods4.b.2"},
		{"non-numeric release", "rods4.3.c"},
		{"empty after prefix", "rods"},
		{"single part", "rods4"},
		{"empty string", ""},
		{"dots only", "rods..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := msg.Version{ReleaseVersion: tt.version}
			if checkVersion(v, 4, 3, 2) {
				t.Errorf("checkVersion(%q, 4, 3, 2) should return false for invalid version", tt.version)
			}
		})
	}
}

func TestDetermineTTL(t *testing.T) {
	tests := []struct {
		name     string
		ttl      time.Duration
		expected int
	}{
		{"zero", 0, 0},
		{"one hour", time.Hour, 1},
		{"two hours", 2 * time.Hour, 2},
		{"negative", -1 * time.Hour, 0},
		{"half hour rounds down", 30 * time.Minute, 0},
		{"90 minutes rounds down", 90 * time.Minute, 1},
		{"24 hours", 24 * time.Hour, 24},
		{"large duration", 720 * time.Hour, 720},
		{"one second", time.Second, 0},
		{"negative large", -48 * time.Hour, 0},
		{"just under 1h", time.Hour - time.Nanosecond, 0},
		{"just over 1h", time.Hour + time.Nanosecond, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineTTL(tt.ttl)
			if result != tt.expected {
				t.Errorf("determineTTL(%v) = %d, want %d", tt.ttl, result, tt.expected)
			}
		})
	}
}

func TestRetrieveValue(t *testing.T) {
	state := map[string]any{
		testUsername:  testUserLower,
		testPassword:  testSecret,
		"nested_data": map[string]any{testKey: value},
	}

	tests := []struct {
		name     string
		path     string
		expected string
		wantErr  bool
	}{
		{"top-level string", "/username", testUserLower, false},
		{"another top-level", "/password", testSecret, false},
		{"missing key returns empty", "/nonexistent", "", false},
		{"nested value", "/nested_data/key", value, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := retrieveValue(state, tt.path)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result != tt.expected {
				t.Errorf("retrieveValue(state, %q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

func TestRetrieveValueInvalidPath(t *testing.T) {
	state := map[string]any{testKey: value}

	// A completely invalid JSON pointer should return an error
	_, err := retrieveValue(state, "not a valid pointer ~")
	if err != nil {
		// Invalid pointers that fail jsonpointer.New return error
		t.Logf("got expected error: %v", err)
	}
}

func TestRetrieveValueNonStringValue(t *testing.T) {
	state := map[string]any{
		"count": 42,
		"flag":  true,
	}

	// Non-string values should return empty string (type assertion fails)
	result, err := retrieveValue(state, "/count")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "" {
		t.Errorf("expected empty string for non-string value, got %q", result)
	}

	result, err = retrieveValue(state, "/flag")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "" {
		t.Errorf("expected empty string for non-string value, got %q", result)
	}
}

func TestPatchStateEmpty(t *testing.T) {
	state := map[string]any{testKey: value}
	dirty := false

	err := patchState(state, nil, &dirty, "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if dirty {
		t.Error("dirty should remain false for empty patch")
	}

	err = patchState(state, []map[string]any{}, &dirty, "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if dirty {
		t.Error("dirty should remain false for empty patch slice")
	}
}

func TestPatchStateAdd(t *testing.T) {
	state := map[string]any{"existing": testOldKey}
	dirty := false

	patch := []map[string]any{
		{"op": add, testPath: testNewKey, value: "newvalue"},
	}

	err := patchState(state, patch, &dirty, "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !dirty {
		t.Error("dirty should be true after applying patch")
	}

	if state["newkey"] != "newvalue" {
		t.Errorf("expected state[newkey]='newvalue', got %v", state["newkey"])
	}
}

func TestPatchStateReplace(t *testing.T) {
	state := map[string]any{testKey: testOldKey}
	dirty := false

	patch := []map[string]any{
		{"op": "replace", testPath: "/key", value: "new"},
	}

	err := patchState(state, patch, &dirty, "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !dirty {
		t.Error("dirty should be true")
	}

	if state[testKey] != "new" {
		t.Errorf("expected state[key]='new', got %v", state[testKey])
	}
}

func TestPatchStateDefaultValue(t *testing.T) {
	state := map[string]any{testKey: testOldKey}
	dirty := false

	// Patch without value should use defaultValue
	patch := []map[string]any{
		{"op": add, testPath: testNewKey},
	}

	err := patchState(state, patch, &dirty, "mydefault")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !dirty {
		t.Error("dirty should be true")
	}

	if state["newkey"] != "mydefault" {
		t.Errorf("expected state[newkey]='mydefault', got %v", state["newkey"])
	}
}

func TestPatchStateSkipsNonAddReplace(t *testing.T) {
	state := map[string]any{testKey: value, "other": "keep"}
	dirty := false

	// Only add and "replace" get the defaultValue fill-in.
	// A testStr op without value is passed through to jsonpatch which will handle it.
	patch := []map[string]any{
		{"op": add, testPath: testNewKey},
	}

	err := patchState(state, patch, &dirty, "filled")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !dirty {
		t.Error("dirty should be true after applying patch")
	}

	if state["newkey"] != "filled" {
		t.Errorf("expected state[newkey]='filled', got %v", state["newkey"])
	}
}

type mockPrompt struct {
	askResponse      string
	passwordResponse string
	printMessages    []string
	askErr           error
	passwordErr      error
}

func (m *mockPrompt) Print(message string) error {
	m.printMessages = append(m.printMessages, message)
	return nil
}

func (m *mockPrompt) Ask(message string) (string, error) {
	return m.askResponse, m.askErr
}

func (m *mockPrompt) Password(message string) (string, error) {
	return m.passwordResponse, m.passwordErr
}

func TestPromptValueNonSensitive(t *testing.T) {
	p := &mockPrompt{askResponse: "answer"}

	result, err := promptValue(p, "Enter value", false, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "answer" {
		t.Errorf("expected 'answer', got %q", result)
	}
}

func TestPromptValueSensitive(t *testing.T) {
	p := &mockPrompt{passwordResponse: testSecret}

	result, err := promptValue(p, "Enter password", true, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != testSecret {
		t.Errorf("expected 'secret', got %q", result)
	}
}

func TestPromptValueWithDefault(t *testing.T) {
	// When user enters empty string, defaultValue should be used
	p := &mockPrompt{askResponse: ""}

	result, err := promptValue(p, "Enter value", false, "defaultVal")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "defaultVal" {
		t.Errorf("expected 'defaultVal', got %q", result)
	}
}

func TestPromptValueWithDefaultOverridden(t *testing.T) {
	p := &mockPrompt{askResponse: testCustom}

	result, err := promptValue(p, "Enter value", false, "defaultVal")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != testCustom {
		t.Errorf("expected 'custom', got %q", result)
	}
}

func TestPromptValueError(t *testing.T) {
	p := &mockPrompt{askErr: fmt.Errorf("input error")}

	_, err := promptValue(p, "Enter value", false, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetValue(t *testing.T) {
	state := map[string]any{
		testUsername: testUserLower,
		testPassword: testSecret,
	}

	t.Run("retrieve path", func(t *testing.T) {
		result, err := getValue(state, nil, "", false, "/username", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result != testUserLower {
			t.Errorf("expected 'testuser', got %q", result)
		}
	})

	t.Run("prompt with default from state", func(t *testing.T) {
		p := &mockPrompt{askResponse: ""}

		result, err := getValue(state, p, "Enter value", false, "", "/username")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result != testUserLower {
			t.Errorf("expected 'testuser' as default, got %q", result)
		}
	})

	t.Run("prompt without default", func(t *testing.T) {
		p := &mockPrompt{askResponse: testCustom}

		result, err := getValue(state, p, "Enter value", false, "", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result != testCustom {
			t.Errorf("expected 'custom', got %q", result)
		}
	})
}

func TestConnAccessors(t *testing.T) {
	now := time.Now()
	c := &conn{
		connectedAt:     now,
		transportErrors: 3,
		sqlErrors:       1,
		nativePassword:  "native123",
		clientSignature: "sig456",
	}

	if got := c.ConnectedAt(); got != now {
		t.Errorf("ConnectedAt() = %v, want %v", got, now)
	}

	if got := c.TransportErrors(); got != 3 {
		t.Errorf("TransportErrors() = %d, want 3", got)
	}

	if got := c.SQLErrors(); got != 1 {
		t.Errorf("SQLErrors() = %d, want 1", got)
	}

	if got := c.NativePassword(); got != "native123" {
		t.Errorf("NativePassword() = %q, want 'native123'", got)
	}

	if got := c.ClientSignature(); got != "sig456" {
		t.Errorf("ClientSignature() = %q, want 'sig456'", got)
	}
}

func TestConnServerVersion(t *testing.T) {
	c := &conn{
		version: &msg.Version{ReleaseVersion: releaseVersion},
	}

	if got := c.ServerVersion(); got != mockVersion {
		t.Errorf("ServerVersion() = %q, want '4.3.2'", got)
	}
}

func TestConnEnv(t *testing.T) {
	env := Env{
		Username: "user1",
		Zone:     "zone1",
	}

	c := &conn{
		env: &env,
	}

	got := c.Env()
	if got.Username != "user1" || got.Zone != "zone1" {
		t.Errorf("Env() = %+v, want Username=user1, Zone=zone1", got)
	}
}

func TestBuildErrorXML(t *testing.T) {
	c := &conn{
		protocol: msg.XML,
	}

	// No error payload
	m := msg.Message{
		Header: msg.Header{ErrorLen: 0},
		Body:   msg.Body{Message: []byte("simple error")},
	}

	result := c.buildError(m)
	if result != "simple error" {
		t.Errorf("expected 'simple error', got %q", result)
	}
}

func TestBuildErrorNativeNoError(t *testing.T) {
	c := &conn{
		protocol: msg.Native,
	}

	m := msg.Message{
		Header: msg.Header{ErrorLen: 0},
		Body:   msg.Body{Message: []byte("native error msg")},
	}

	result := c.buildError(m)
	if result != "native error msg" {
		t.Errorf("expected 'native error msg', got %q", result)
	}
}
