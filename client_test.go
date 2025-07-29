package iron

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/kuleuven/iron/msg"
	"github.com/kuleuven/iron/transfer"
	"golang.org/x/sync/errgroup"
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

	tcpAddr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("expected TCP address, got %T", listener.Addr())
	}

	var wg errgroup.Group

	wg.Go(func() error {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}

		wg.Go(func() error {
			return runDialog(conn, []Dialog{
				{
					msg.AUTH_REQUEST_AN,
					&msg.AuthRequest{},
					msg.AuthChallenge{
						Challenge: base64.StdEncoding.EncodeToString([]byte("testChallengetestChallengetestChallengetestChallengetestChallenge")),
					},
				},
				{
					msg.AUTH_RESPONSE_AN,
					&msg.AuthChallengeResponse{},
					msg.AuthResponse{},
				},
			})
		})

		return nil
	})

	wg.Go(func() error {
		defer listener.Close()

		env := Env{Host: "127.0.0.1", Port: tcpAddr.Port, ClientServerNegotiation: "no_negotiation"}

		env.ApplyDefaults()

		client, err := New(context.Background(), env, Option{ClientName: "test", MaxConns: 1})
		if err != nil {
			return err
		}

		defer client.Close()

		conn, err := client.Connect()
		if err != nil {
			return err
		}

		_, err = client.TryConnect()
		if err != ErrNoConnectionsAvailable {
			return fmt.Errorf("expected error %v, got %v", ErrNoConnectionsAvailable, err)
		}

		done := make(chan struct{})

		wg.Go(func() error {
			defer close(done)

			conn1, err := client.Connect()
			if err != nil {
				return err
			}

			return conn1.Close()
		})

		conn.Close()

		<-done

		return client.Close()
	})

	if err := wg.Wait(); err != nil {
		t.Fatal(err)
	}
}

func runDialog(conn net.Conn, dialog []Dialog) error {
	defer conn.Close()

	_, err := msg.Read(conn, &msg.StartupPack{}, nil, msg.XML, "RODS_CONNECT")
	if err != nil {
		return err
	}

	err = msg.Write(conn, msg.Version{
		ReleaseVersion: "rods4.3.2",
	}, nil, msg.XML, "RODS_VERSION", 0)
	if err != nil {
		return err
	}

	for _, d := range dialog {
		intInfo, err := msg.Read(conn, d.Request, nil, msg.XML, "RODS_API_REQ")
		if err != nil {
			fmt.Print(err)
			return err
		}

		if intInfo != int32(d.APINumber) {
			return fmt.Errorf("unexpected API number: expected %d, got %d", d.APINumber, intInfo)
		}

		err = msg.Write(conn, d.Response, nil, msg.XML, "RODS_API_REPLY", 0)
		if err != nil {
			fmt.Print(err)
			return err
		}
	}

	_, err = msg.Read(conn, &msg.EmptyResponse{}, nil, msg.XML, "RODS_DISCONNECT")

	return err
}

type Dialog struct {
	msg.APINumber
	Request, Response any
}

func TestClientUpload(t *testing.T) { //nolint:funlen
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	tcpAddr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("expected TCP address, got %T", listener.Addr())
	}

	var wg errgroup.Group

	wg.Go(func() error {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}

		wg.Go(func() error {
			return runDialog(conn, []Dialog{
				{
					msg.AUTH_REQUEST_AN,
					&msg.AuthRequest{},
					msg.AuthChallenge{
						Challenge: base64.StdEncoding.EncodeToString([]byte("testChallengetestChallengetestChallengetestChallengetestChallenge")),
					},
				},
				{
					msg.AUTH_RESPONSE_AN,
					&msg.AuthChallengeResponse{},
					msg.AuthResponse{},
				},
				{
					msg.DATA_OBJ_OPEN_AN,
					&msg.DataObjectRequest{},
					msg.FileDescriptor(1),
				},
				{
					msg.GET_FILE_DESCRIPTOR_INFO_APN,
					&msg.GetDescriptorInfoRequest{},
					msg.GetDescriptorInfoResponse{
						DataObjectInfo: map[string]any{
							"replica_number":     1,
							"resource_hierarchy": "blub",
						},
					},
				},
				{
					msg.DATA_OBJ_WRITE_AN,
					&msg.OpenedDataObjectRequest{},
					msg.EmptyResponse{},
				},
				{
					msg.DATA_OBJ_WRITE_AN,
					&msg.OpenedDataObjectRequest{},
					msg.EmptyResponse{},
				},
				{
					msg.DATA_OBJ_CLOSE_AN,
					&msg.OpenedDataObjectRequest{},
					msg.EmptyResponse{},
				},
			})
		})

		conn2, err := listener.Accept()
		if err != nil {
			return err
		}

		wg.Go(func() error {
			return runDialog(conn2, []Dialog{
				{
					msg.AUTH_REQUEST_AN,
					&msg.AuthRequest{},
					msg.AuthChallenge{
						Challenge: base64.StdEncoding.EncodeToString([]byte("testChallengetestChallengetestChallengetestChallengetestChallenge")),
					},
				},
				{
					msg.AUTH_RESPONSE_AN,
					&msg.AuthChallengeResponse{},
					msg.AuthResponse{},
				},
				{
					msg.DATA_OBJ_OPEN_AN,
					&msg.DataObjectRequest{},
					msg.FileDescriptor(1),
				},
				{
					msg.DATA_OBJ_LSEEK_AN,
					&msg.OpenedDataObjectRequest{},
					msg.SeekResponse{Offset: 200},
				},
				{
					msg.DATA_OBJ_WRITE_AN,
					&msg.OpenedDataObjectRequest{},
					msg.EmptyResponse{},
				},
				{
					msg.DATA_OBJ_WRITE_AN,
					&msg.OpenedDataObjectRequest{},
					msg.EmptyResponse{},
				},
				{
					msg.REPLICA_CLOSE_APN,
					&msg.CloseDataObjectReplicaRequest{},
					msg.EmptyResponse{},
				},
			})
		})

		return nil
	})

	wg.Go(func() error {
		defer listener.Close()

		env := Env{Host: "127.0.0.1", Port: tcpAddr.Port, ClientServerNegotiation: "no_negotiation"}

		env.ApplyDefaults()

		client, err := New(context.Background(), env, Option{ClientName: "test", MaxConns: 2})
		if err != nil {
			return err
		}

		defer client.Close()

		f, err := os.CreateTemp(t.TempDir(), "test")
		if err != nil {
			return err
		}

		defer os.Remove(f.Name())

		_, err = f.Write(bytes.Repeat([]byte("test"), 100))
		if err != nil {
			return err
		}

		if err = f.Close(); err != nil {
			return err
		}

		transfer.BufferSize = 100
		transfer.MinimumRangeSize = 200

		return client.Upload(context.Background(), f.Name(), "test", Options{
			//	SyncModTime: true,
			Threads: 2,
		})
	})

	if err := wg.Wait(); err != nil {
		t.Fatal(err)
	}
}

func TestClientDownload(t *testing.T) { //nolint:funlen
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	tcpAddr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("expected TCP address, got %T", listener.Addr())
	}

	var wg errgroup.Group

	wg.Go(func() error {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}

		wg.Go(func() error {
			return runDialog(conn, []Dialog{
				{
					msg.AUTH_REQUEST_AN,
					&msg.AuthRequest{},
					msg.AuthChallenge{
						Challenge: base64.StdEncoding.EncodeToString([]byte("testChallengetestChallengetestChallengetestChallengetestChallenge")),
					},
				},
				{
					msg.AUTH_RESPONSE_AN,
					&msg.AuthChallengeResponse{},
					msg.AuthResponse{},
				},
				{
					msg.DATA_OBJ_OPEN_AN,
					&msg.DataObjectRequest{},
					msg.FileDescriptor(1),
				},
				{
					msg.DATA_OBJ_LSEEK_AN,
					&msg.OpenedDataObjectRequest{},
					msg.SeekResponse{},
				},
				{
					msg.DATA_OBJ_READ_AN,
					&msg.OpenedDataObjectRequest{},
					msg.EmptyResponse{},
				},
				{
					msg.DATA_OBJ_CLOSE_AN,
					&msg.OpenedDataObjectRequest{},
					msg.EmptyResponse{},
				},
			})
		})

		return nil
	})

	wg.Go(func() error {
		defer listener.Close()

		env := Env{Host: "127.0.0.1", Port: tcpAddr.Port, ClientServerNegotiation: "no_negotiation"}

		env.ApplyDefaults()

		client, err := New(context.Background(), env, Option{ClientName: "test", MaxConns: 2})
		if err != nil {
			return err
		}

		defer client.Close()

		f, err := os.CreateTemp(t.TempDir(), "test")
		if err != nil {
			return err
		}

		defer os.Remove(f.Name())

		if err = f.Close(); err != nil {
			return err
		}

		transfer.BufferSize = 100
		transfer.MinimumRangeSize = 200

		return client.Download(context.Background(), f.Name(), "test", Options{
			//	SyncModTime: true,
			Threads: 1,
		})
	})

	if err := wg.Wait(); err != nil {
		t.Fatal(err)
	}
}

func TestClientVerify(t *testing.T) { //nolint:funlen
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	tcpAddr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("expected TCP address, got %T", listener.Addr())
	}

	var wg errgroup.Group

	wg.Go(func() error {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}

		wg.Go(func() error {
			return runDialog(conn, []Dialog{
				{
					msg.AUTH_REQUEST_AN,
					&msg.AuthRequest{},
					msg.AuthChallenge{
						Challenge: base64.StdEncoding.EncodeToString([]byte("testChallengetestChallengetestChallengetestChallengetestChallenge")),
					},
				},
				{
					msg.AUTH_RESPONSE_AN,
					&msg.AuthChallengeResponse{},
					msg.AuthResponse{},
				},
				{
					msg.DATA_OBJ_CHKSUM_AN,
					&msg.DataObjectRequest{},
					&msg.Checksum{
						Checksum: "sha2:jMuGXraweIxVs1RAFTHRM8Nbk/mrfSZwERQ3YzMHvy8=",
					},
				},
			})
		})

		return nil
	})

	wg.Go(func() error {
		defer listener.Close()

		env := Env{Host: "127.0.0.1", Port: tcpAddr.Port, ClientServerNegotiation: "no_negotiation"}

		env.ApplyDefaults()

		client, err := New(context.Background(), env, Option{ClientName: "test", MaxConns: 2})
		if err != nil {
			return err
		}

		defer client.Close()

		f, err := os.CreateTemp(t.TempDir(), "test")
		if err != nil {
			return err
		}

		defer os.Remove(f.Name())

		_, err = f.Write(bytes.Repeat([]byte("test"), 100))
		if err != nil {
			return err
		}

		if err = f.Close(); err != nil {
			return err
		}

		return client.Verify(context.Background(), f.Name(), "test")
	})

	if err := wg.Wait(); err != nil {
		t.Fatal(err)
	}
}
