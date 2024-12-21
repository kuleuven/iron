package iron

import (
	"context"
	"encoding/base64"
	"io"
	"net"
	"testing"

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
		msg.Read(conn, &msg.StartupPack{}, "RODS_CONNECT")

		conn.Close()
	}()

	tcpAddr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("expected TCP address, got %T", listener.Addr())
	}

	env := Env{Host: "127.0.0.1", Port: tcpAddr.Port}

	env.ApplyDefaults()

	client, err := New(env, "test", 1)
	if err != nil {
		t.Fatal(err)
	}

	defer client.Close()

	_, err = client.Connect(context.Background())
	if err != io.EOF {
		t.Fatalf("expected EOF, got %v", err)
	}

	_, err = client.Connect(context.Background())
	if err != io.EOF {
		t.Fatalf("expected EOF, got %v", err)
	}
}

func TestClientNative(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			panic(err)
		}

		msg.Read(conn, &msg.StartupPack{}, "RODS_CONNECT")
		msg.Write(conn, msg.Version{}, "RODS_VERSION", 0)
		msg.Read(conn, &msg.AuthRequest{}, "RODS_API_REQ")
		msg.Write(conn, msg.AuthChallenge{
			Challenge: base64.StdEncoding.EncodeToString([]byte("testChallengetestChallengetestChallengetestChallengetestChallenge")),
		}, "RODS_API_REPLY", 0)
		msg.Read(conn, &msg.AuthChallengeResponse{}, "RODS_API_REQ")
		msg.Write(conn, msg.AuthResponse{}, "RODS_API_REPLY", 0)

		conn.Close()
	}()

	tcpAddr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("expected TCP address, got %T", listener.Addr())
	}

	env := Env{Host: "127.0.0.1", Port: tcpAddr.Port, ClientServerNegotiation: "no_negotiation"}

	env.ApplyDefaults()

	client, err := New(env, "test", 1)
	if err != nil {
		t.Fatal(err)
	}

	defer client.Close()

	conn, err := client.Connect(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})

	go func() {
		conn, err := client.Connect(context.Background())
		if err != nil {
			panic(err)
		}

		conn.Close()

		close(done)
	}()

	conn.Close()

	<-done
}
