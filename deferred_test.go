package iron

import (
	"context"
	"encoding/base64"
	"net"
	"reflect"
	"testing"

	"gitea.icts.kuleuven.be/coz/iron/msg"
)

func TestDeferredNative(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			panic(err)
		}

		msg.Read(conn, &msg.StartupPack{}, nil, "RODS_CONNECT")
		msg.Write(conn, msg.Version{ReleaseVersion: "rods4.3.2"}, nil, "RODS_VERSION", 0)
		msg.Read(conn, &msg.AuthRequest{}, nil, "RODS_API_REQ")
		msg.Write(conn, msg.AuthChallenge{
			Challenge: base64.StdEncoding.EncodeToString([]byte("testChallengetestChallengetestChallengetestChallengetestChallenge")),
		}, nil, "RODS_API_REPLY", 0)
		msg.Read(conn, &msg.AuthChallengeResponse{}, nil, "RODS_API_REQ")
		msg.Write(conn, msg.AuthResponse{}, nil, "RODS_API_REPLY", 0)
		msg.Read(conn, &msg.QueryRequest{}, nil, "RODS_API_REQ")
		msg.Write(conn, msg.EmptyResponse{}, nil, "RODS_API_REPLY", 0)
		msg.Read(conn, msg.EmptyResponse{}, nil, "RODS_DISCONNECT")
		conn.Close()
	}()

	tcpAddr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("expected TCP address, got %T", listener.Addr())
	}

	env := Env{Host: "127.0.0.1", Port: tcpAddr.Port, ClientServerNegotiation: "no_negotiation"}

	env.ApplyDefaults()

	conn, err := Dial(context.Background(), env, Option{
		ClientName:        "test",
		ConnectAtFirstUse: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(conn.Env(), Env{}) {
		t.Error(err)
	}

	err = conn.Request(context.Background(), 702, msg.QueryRequest{}, &msg.EmptyResponse{})
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
}
