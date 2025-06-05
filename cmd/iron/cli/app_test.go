package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net"
	"os"
	"testing"

	"github.com/kuleuven/iron"
	"github.com/kuleuven/iron/msg"
)

func writeConfig(env iron.Env) (string, error) {
	f, err := os.CreateTemp("", "")
	if err != nil {
		return "", err
	}

	defer f.Close()

	err = json.NewEncoder(f).Encode(env)
	if err != nil {
		os.Remove(f.Name())

		return "", err
	}

	return f.Name(), nil
}

func TestNew(t *testing.T) {
	app := New(context.Background())

	cmd := app.Command()

	if cmd == nil {
		t.Fatal("expected command")
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			panic(err)
		}

		msg.Read(conn, &msg.StartupPack{}, nil, msg.XML, "RODS_CONNECT")
		msg.Write(conn, msg.Version{
			ReleaseVersion: "rods4.3.2",
		}, nil, msg.XML, "RODS_VERSION", 0)
		msg.Read(conn, &msg.AuthRequest{}, nil, msg.XML, "RODS_API_REQ")
		msg.Write(conn, msg.AuthChallenge{
			Challenge: base64.StdEncoding.EncodeToString([]byte("testChallengetestChallengetestChallengetestChallengetestChallenge")),
		}, nil, msg.XML, "RODS_API_REPLY", 0)
		msg.Read(conn, &msg.AuthChallengeResponse{}, nil, msg.XML, "RODS_API_REQ")
		msg.Write(conn, msg.AuthResponse{}, nil, msg.XML, "RODS_API_REPLY", 0)
		msg.Read(conn, msg.EmptyResponse{}, nil, msg.XML, "RODS_DISCONNECT")
		conn.Close()
	}()

	tcpAddr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("expected TCP address, got %T", listener.Addr())
	}

	app.envfile, err = writeConfig(iron.Env{Host: "127.0.0.1", Port: tcpAddr.Port, ClientServerNegotiation: "no_negotiation"})
	if err != nil {
		t.Fatal(err)
	}

	defer os.Remove(app.envfile)

	defer app.Close()

	if err := cmd.PersistentPreRunE(cmd, nil); err != nil {
		t.Fatal(err)
	}
}
