package iron

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"os"
	"reflect"
	"testing"
	"time"

	"gitea.icts.kuleuven.be/coz/iron/msg"
	"github.com/acomagu/bufpipe"
)

type mockConn struct {
	io.Reader
	io.Writer
}

func (c *mockConn) Close() error {
	return nil
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

func connPipe() (net.Conn, net.Conn) {
	r, w := bufpipe.New(nil)
	R, W := bufpipe.New(nil)

	return &mockConn{r, W}, &mockConn{R, w}
}

const releaseVer = "rods4.3.2"

func TestConnNative(t *testing.T) {
	ctx := context.Background()
	transport, server := connPipe()

	msg.Write(server, msg.ClientServerNegotiation{
		Result: "CS_NEG_DONT_CARE",
	}, nil, "RODS_CS_NEG_T", 0)

	msg.Write(server, msg.Version{
		ReleaseVersion: releaseVer,
	}, nil, "RODS_VERSION", 0)

	msg.Write(server, msg.AuthChallenge{
		Challenge: base64.StdEncoding.EncodeToString([]byte("testChallengetestChallengetestChallengetestChallengetestChallenge")),
	}, nil, "RODS_API_REPLY", 0)

	msg.Write(server, msg.AuthResponse{}, nil, "RODS_API_REPLY", 0)

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

	assert(msg.Read(server, &msg.StartupPack{}, nil, "RODS_CONNECT"))
	assert(msg.Write(server, msg.ClientServerNegotiation{
		Result: "CS_NEQ_REQUIRE",
	}, nil, "RODS_CS_NEG_T", 0))
	assert(msg.Read(server, &msg.ClientServerNegotiation{}, nil, "RODS_CS_NEG_T"))
	assert(msg.Write(server, msg.Version{
		ReleaseVersion: releaseVer,
	}, nil, "RODS_VERSION", 0))

	// Switch to TLS
	cert, err := tls.X509KeyPair(certPem, keyPem)
	assert(err)

	serverTLS := tls.Server(server, &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{cert},
	})

	assert((&msg.Header{}).Read(serverTLS))
	assert(msg.Read(serverTLS, &msg.SSLSharedSecret{}, nil, "SHARED_SECRET"))
	assert(msg.Read(serverTLS, &msg.PamAuthRequest{}, nil, "RODS_API_REQ"))
	assert(msg.Write(serverTLS, msg.PamAuthResponse{
		GeneratedPassword: "testNativePassword",
	}, nil, "RODS_API_REPLY", 0))
	assert(msg.Read(serverTLS, &msg.AuthRequest{}, nil, "RODS_API_REQ"))
	assert(msg.Write(serverTLS, msg.AuthChallenge{
		Challenge: base64.StdEncoding.EncodeToString([]byte("testChallengetestChallengetestChallengetestChallengetestChallenge")),
	}, nil, "RODS_API_REPLY", 0))
	assert(msg.Read(serverTLS, &msg.AuthChallengeResponse{}, nil, "RODS_API_REQ"))
	assert(msg.Write(serverTLS, msg.AuthResponse{}, nil, "RODS_API_REPLY", 0))
}

func TestConnPamPassword(t *testing.T) {
	ctx := context.Background()
	transport, server := connPipe()

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
	ctx := context.Background()
	transport, server := connPipe()

	go pamResponses(server)

	f, err := os.CreateTemp("", "")
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
	ctx := context.Background()
	transport, server := connPipe()

	go pamResponses(server)

	f, err := os.CreateTemp("", "")
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
		_, err = msg.Read(conn, &msg.StartupPack{}, nil, "RODS_CONNECT")
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

	_, err = Dial(context.Background(), env, Option{ClientName: "test"})
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

	_, err := NewConn(context.Background(), nil, env, "test")
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

	_, err := NewConn(context.Background(), nil, env, "test")
	if err != ErrTLSRequired {
		t.Fatalf("expected ErrTLSRequired, got %v", err)
	}
}

func TestOldVersion(t *testing.T) {
	ctx := context.Background()
	transport, server := connPipe()

	msg.Write(server, msg.ClientServerNegotiation{
		Result: "CS_NEG_DONT_CARE",
	}, nil, "RODS_CS_NEG_T", 0)

	msg.Write(server, msg.Version{
		ReleaseVersion: "rods4.2.9",
	}, nil, "RODS_VERSION", 0)

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

func TestRequest(t *testing.T) {
	ctx := context.Background()
	transport, server := connPipe()

	msg.Write(server, msg.Version{
		ReleaseVersion: releaseVer,
	}, nil, "RODS_VERSION", 0)

	msg.Write(server, msg.AuthChallenge{
		Challenge: base64.StdEncoding.EncodeToString([]byte("testChallengetestChallengetestChallengetestChallengetestChallenge")),
	}, nil, "RODS_API_REPLY", 0)

	msg.Write(server, msg.AuthResponse{}, nil, "RODS_API_REPLY", 0)

	msg.Write(server, msg.EmptyResponse{}, nil, "RODS_API_REPLY", msg.SYS_SVR_TO_CLI_COLL_STAT)

	msg.Write(server, msg.CollectionOperationStat{}, nil, "RODS_API_REPLY", 0)

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
}
