package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kuleuven/iron"
	"github.com/kuleuven/iron/api"
	"github.com/kuleuven/iron/msg"
	"github.com/kuleuven/iron/scramble"
	"github.com/spf13/cobra"
)

func writeConfig(t *testing.T, env iron.Env) (string, error) {
	dir := t.TempDir()

	f, err := os.CreateTemp(dir, "")
	if err != nil {
		return "", err
	}

	defer f.Close()

	err = json.NewEncoder(f).Encode(env)
	if err != nil {
		os.Remove(f.Name())

		return "", err
	}

	fi, err := f.Stat()
	if err != nil {
		return "", err
	}

	scrambledPassword := scramble.EncodeIrodsA(env.Password, uid(fi), time.Now())

	return f.Name(), os.WriteFile(filepath.Join(filepath.Dir(f.Name()), ".irodsA"), scrambledPassword, 0o600)
}

func TestNew(t *testing.T) { //nolint:funlen
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
			ReleaseVersion: "rods5.0.1",
		}, nil, msg.XML, "RODS_VERSION", 0)
		msg.Read(conn, &msg.AuthPluginRequest{}, nil, msg.XML, "RODS_API_REQ")
		msg.Write(conn, msg.AuthPluginResponse{
			RequestResult: base64.StdEncoding.EncodeToString([]byte("testChallengetestChallengetestChallengetestChallengetestChallenge")),
		}, nil, msg.XML, "RODS_API_REPLY", 0)
		msg.Read(conn, &msg.AuthPluginRequest{}, nil, msg.XML, "RODS_API_REQ")
		msg.Write(conn, msg.AuthPluginResponse{}, nil, msg.XML, "RODS_API_REPLY", 0)
		msg.Read(conn, msg.EmptyResponse{}, nil, msg.XML, "RODS_DISCONNECT")
		conn.Close()
	}()

	tcpAddr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("expected TCP address, got %T", listener.Addr())
	}

	envfile, err := writeConfig(t, iron.Env{
		Host:                    "127.0.0.1",
		Port:                    tcpAddr.Port,
		ClientServerNegotiation: "no_negotiation",
	})
	if err != nil {
		t.Fatal(err)
	}

	defer os.Remove(envfile)

	err = WriteAuthFile(filepath.Join(filepath.Dir(envfile), ".irodsA"), "testPassword")
	if err != nil {
		t.Fatal(err)
	}

	app := New(t.Context(), WithLoader(FileLoader(envfile)), WithDefaultWorkdirFromFile(envfile), WithPasswordStore(FilePasswordStore(envfile)))

	cmd := app.Command()

	if cmd == nil {
		t.Fatal("expected command")
	}

	defer app.Close()

	if err := cmd.PersistentPreRunE(cmd, nil); err != nil {
		t.Fatal(err)
	}

	if err := app.cd().RunE(cmd, []string{"authenticate"}); err != nil {
		t.Fatal(err)
	}

	if err := app.cd().RunE(cmd, []string{"test"}); err != nil {
		t.Fatal(err)
	}
}

type mockApp struct {
	*api.MockConn
	*App
}

func testApp(t *testing.T) *mockApp {
	app := New(t.Context())

	testConn := &api.MockConn{}

	app.Client = &iron.Client{
		API: &api.API{
			Username: "testuser",
			Zone:     "testzone",
			Connect: func(context.Context) (api.Conn, error) {
				return testConn, nil
			},
			DefaultResource: "demoResc",
		},
	}

	return &mockApp{
		MockConn: testConn,
		App:      app,
	}
}

func TestAutocomplete(t *testing.T) {
	app := testApp(t)

	opts, directive := app.CompleteArgs(app.mkdir(), []string{}, "/test")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("expected default directive, got %d", directive)
	}

	if len(opts) != 1 {
		t.Fatalf("expected 0 options, got %v", opts)
	}

	if opts[0] != "/testzone/" {
		t.Fatalf("expected /testzone/, got %s", opts[0])
	}
}

var responses = []any{
	msg.QueryResponse{
		RowCount:       1,
		AttributeCount: 6,
		TotalRowCount:  1,
		ContinueIndex:  0,
		SQLResult: []msg.SQLResult{
			{AttributeIndex: 500, ResultLen: 1, Values: []string{"1"}},
			{AttributeIndex: 503, ResultLen: 1, Values: []string{"/testzone"}},
			{AttributeIndex: 504, ResultLen: 1, Values: []string{"zone"}},
			{AttributeIndex: 508, ResultLen: 1, Values: []string{"10000"}},
			{AttributeIndex: 509, ResultLen: 1, Values: []string{"2024"}},
			{AttributeIndex: 506, ResultLen: 1, Values: []string{"1"}},
		},
	},
	msg.QueryResponse{
		RowCount:       2,
		AttributeCount: 7,
		TotalRowCount:  2,
		ContinueIndex:  0,
		SQLResult: []msg.SQLResult{
			{AttributeIndex: 500, ResultLen: 1, Values: []string{"2", "3"}},
			{AttributeIndex: 501, ResultLen: 1, Values: []string{"/testzone/a", "/testzone/home"}},
			{AttributeIndex: 503, ResultLen: 1, Values: []string{"rods", "user"}},
			{AttributeIndex: 504, ResultLen: 1, Values: []string{"zone", "zone"}},
			{AttributeIndex: 508, ResultLen: 1, Values: []string{"10000", "10000"}},
			{AttributeIndex: 509, ResultLen: 1, Values: []string{"2024", "2025"}},
			{AttributeIndex: 506, ResultLen: 1, Values: []string{"1", "0"}},
		},
	},
	msg.QueryResponse{},
}

func TestAutocomplete2(t *testing.T) {
	app := testApp(t)

	app.AddResponses(responses)

	opts, directive := app.CompleteArgs(app.mkdir(), []string{}, "/testzone/hom")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("expected default directive, got %d", directive)
	}

	if len(opts) != 1 {
		t.Fatalf("expected 0 options, got %v", opts)
	}

	if opts[0] != "/testzone/home/" {
		t.Fatalf("expected /testzone/home/, got %s", opts[0])
	}
}
