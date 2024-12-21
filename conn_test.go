package iron

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"io"
	"net"
	"runtime/debug"
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

func TestConnNative(t *testing.T) {
	ctx := context.Background()
	transport, server := connPipe()

	msg.Write(server, msg.ClientServerNegotiation{
		Result: "CS_NEG_DONT_CARE",
	}, "RODS_CS_NEG_T", 0)

	msg.Write(server, msg.Version{}, "RODS_VERSION", 0)

	msg.Write(server, msg.AuthChallenge{
		Challenge: base64.StdEncoding.EncodeToString([]byte("testChallengetestChallengetestChallengetestChallengetestChallenge")),
	}, "RODS_API_REPLY", 0)

	msg.Write(server, msg.AuthResponse{}, "RODS_API_REPLY", 0)

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

func TestConnPamPassword(t *testing.T) {
	ctx := context.Background()
	transport, server := connPipe()

	assert := func(args ...any) {
		if err := args[len(args)-1]; err != nil {
			debug.PrintStack()
			t.Fatal(err)
		}
	}

	go func() {
		assert(msg.Read(server, &msg.StartupPack{}, "RODS_CONNECT"))
		assert(msg.Write(server, msg.ClientServerNegotiation{
			Result: "CS_NEQ_REQUIRE",
		}, "RODS_CS_NEG_T", 0))
		assert(msg.Read(server, &msg.ClientServerNegotiation{}, "RODS_CS_NEG_T"))
		assert(msg.Write(server, msg.Version{}, "RODS_VERSION", 0))

		// Switch to TLS
		cert, err := tls.X509KeyPair(certPem, keyPem)
		assert(err)

		serverTLS := tls.Server(server, &tls.Config{
			MinVersion:   tls.VersionTLS12,
			Certificates: []tls.Certificate{cert},
		})

		assert((&msg.Header{}).Read(serverTLS))
		assert(msg.Read(serverTLS, &msg.SSLSharedSecret{}, "SHARED_SECRET"))
		assert(msg.Read(serverTLS, &msg.PamAuthRequest{}, "RODS_API_REQ"))
		assert(msg.Write(serverTLS, msg.PamAuthResponse{
			GeneratedPassword: "testNativePassword",
		}, "RODS_API_REPLY", 0))
		assert(msg.Read(serverTLS, &msg.AuthRequest{}, "RODS_API_REQ"))
		assert(msg.Write(serverTLS, msg.AuthChallenge{
			Challenge: base64.StdEncoding.EncodeToString([]byte("testChallengetestChallengetestChallengetestChallengetestChallenge")),
		}, "RODS_API_REPLY", 0))
		assert(msg.Read(serverTLS, &msg.AuthChallengeResponse{}, "RODS_API_REQ"))
		assert(msg.Write(serverTLS, msg.AuthResponse{}, "RODS_API_REPLY", 0))
	}()

	env := Env{
		Host:                          "localhost",
		Port:                          1247,
		Zone:                          "testZone",
		Username:                      "testUser",
		Password:                      "testPassword",
		AuthScheme:                    "pam_password",
		ClientServerNegotiationPolicy: "CS_NEG_DONT_CARE",
		SSLVerifyServer:               "none",
	}

	env.ApplyDefaults()

	_, err := NewConn(ctx, transport, env, "test")
	if err != nil {
		t.Fatal(err)
	}
}
