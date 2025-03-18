package iron

import (
	"context"
	"encoding/base64"
	"io"
	"net"
	"reflect"
	"testing"
	"time"

	"gitea.icts.kuleuven.be/coz/iron/msg"
)

func TestClient(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			panic(err)
		}

		// Consume startup message
		msg.Read(conn, &msg.StartupPack{}, nil, msg.XML, "RODS_CONNECT")

		conn.Close()
	}()

	tcpAddr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("expected TCP address, got %T", listener.Addr())
	}

	env := Env{Host: "127.0.0.1", Port: tcpAddr.Port}

	env.ApplyDefaults()

	client, err := New(context.Background(), env, Option{
		ClientName:                "test",
		DeferConnectionToFirstUse: true,
		EnvCallback:               func() (Env, time.Time, error) { return env, time.Time{}, nil },
		DiscardConnectionAge:      time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}

	defer client.Close()

	if !reflect.DeepEqual(client.Env(), env) {
		t.Error("expected environment settings to match")
	}

	_, err = client.Connect()
	if err != io.EOF {
		t.Fatalf("expected EOF, got %v", err)
	}

	_, err = client.Connect()
	if err != io.EOF {
		t.Fatalf("expected EOF, got %v", err)
	}
}

func TestClientNative(t *testing.T) { //nolint:funlen
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

	env := Env{Host: "127.0.0.1", Port: tcpAddr.Port, ClientServerNegotiation: "no_negotiation"}

	env.ApplyDefaults()

	client, err := New(context.Background(), env, Option{ClientName: "test"})
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		err = client.Close()
		if err != nil {
			t.Fatal(err)
		}
	}()

	conn, err := client.Connect()
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})

	go func() {
		conn, err := client.Connect()
		if err != nil {
			panic(err)
		}

		conn.Close()
		close(done)
	}()

	conn.Close()

	<-done
}
