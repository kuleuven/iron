package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
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

	for range 3 {
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

			msg.Read(conn, &msg.QueryRequest{}, nil, msg.XML, "RODS_API_REQ")
			msg.Write(conn, msg.QueryResponse{
				RowCount:       1,
				AttributeCount: 6,
				TotalRowCount:  1,
				ContinueIndex:  0,
				SQLResult: []msg.SQLResult{
					{AttributeIndex: 500, ResultLen: 1, Values: []string{"1"}},
					{AttributeIndex: 503, ResultLen: 1, Values: []string{"/testzone/coll"}},
					{AttributeIndex: 504, ResultLen: 1, Values: []string{"zone"}},
					{AttributeIndex: 508, ResultLen: 1, Values: []string{"10000"}},
					{AttributeIndex: 509, ResultLen: 1, Values: []string{"2024"}},
					{AttributeIndex: 506, ResultLen: 1, Values: []string{"1"}},
				},
			}, nil, msg.XML, "RODS_API_REPLY", 0)

			msg.Read(conn, msg.EmptyResponse{}, nil, msg.XML, "RODS_DISCONNECT")
			conn.Close()
		}()
	}

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
	cmd.SetContext(t.Context())

	defer app.Close()

	if err := cmd.PersistentPreRunE(cmd, nil); err != nil {
		t.Fatal(err)
	}

	// Alter Use so init() does not erase password
	for _, child := range cmd.Commands() {
		if strings.HasPrefix(child.Use, "auth ") {
			child.Use = "test-" + child.Use
		}
	}

	cmd.SetArgs([]string{"test-auth", "zone1"})

	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}

	cmd.SetArgs([]string{"test-auth", "zone2"})

	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestNewConfigStore(t *testing.T) { //nolint:funlen
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	for range 2 {
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
	}

	tcpAddr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("expected TCP address, got %T", listener.Addr())
	}

	f, err := os.CreateTemp(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}

	envfile := f.Name()

	err = f.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = WriteAuthFile(filepath.Join(filepath.Dir(envfile), ".irodsA"), "testPassword")
	if err != nil {
		t.Fatal(err)
	}

	app := New(t.Context(),
		WithConfigStore(FileStore(envfile, iron.Env{
			Port:                    tcpAddr.Port,
			ClientServerNegotiation: "no_negotiation",
		}), []string{"user name", "zone name", "host"}),
		WithLoader(FileLoader(envfile)),
		WithPasswordStore(FilePasswordStore(envfile)),
	)

	cmd := app.Command()
	cmd.SetContext(t.Context())

	if cmd == nil {
		t.Fatal("expected command")
	}

	defer app.Close()

	if err := app.ShellInit(cmd, nil); err != nil {
		t.Fatal(err)
	}

	// Alter Use so init() does not erase password
	for _, child := range cmd.Commands() {
		if strings.HasPrefix(child.Use, "auth ") {
			child.Use = "test-" + child.Use
		}
	}

	cmd.SetArgs([]string{"test-auth", "user", "zone", "127.0.0.1"})

	for range 2 {
		if err := cmd.ExecuteContext(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
}

type mockConn struct {
	*api.MockConn
}

func (c *mockConn) API() *api.API {
	return &api.API{
		Username: "testuser",
		Zone:     "testzone",
		Connect: func(context.Context) (api.Conn, error) {
			return c, nil
		},
		DefaultResource: "demoResc",
	}
}

func (c *mockConn) ConnectedAt() time.Time {
	return time.Now()
}

func (c *mockConn) Env() iron.Env {
	return iron.Env{
		Zone:            "testzone",
		DefaultResource: "demoResc",
	}
}

func (c *mockConn) SQLErrors() int {
	return 0
}

func (c *mockConn) Transport() net.Conn {
	return nil
}

func (c *mockConn) TransportErrors() int {
	return 0
}

func (c *mockConn) ServerVersion() string {
	return "rods5.0.1"
}

type mockApp struct {
	*mockConn
	*App
}

func testApp(t *testing.T) *mockApp {
	app := New(t.Context())

	testConn := &mockConn{
		MockConn: &api.MockConn{},
	}

	var err error

	app.Client, err = iron.New(t.Context(), iron.Env{
		Zone:            "testzone",
		DefaultResource: "demoResc",
	}, iron.Option{
		HandshakeFunc: func(ctx context.Context) (iron.Conn, error) {
			return testConn, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	WithName("test")(app)
	WithDefaultWorkdir("")(app)

	return &mockApp{
		mockConn: testConn,
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

func TestAutocompleteLocal(t *testing.T) {
	app := testApp(t)
	app.inShell = true

	dir := t.TempDir()

	if err := os.Mkdir(filepath.Join(dir, "test"), 0o700); err != nil {
		t.Fatal(err)
	}

	t.Chdir(dir)

	cmd := app.local().Commands()[1]
	cmd.SetContext(t.Context())

	opts, directive := app.CompleteArgs(cmd, []string{}, "te")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("expected default directive, got %d", directive)
	}

	if len(opts) != 1 {
		t.Fatalf("expected 1 options, got %v", opts)
	}

	if opts[0] != "test" {
		t.Fatalf("expected test, got %s", opts[0])
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
	msg.QueryResponse{
		RowCount:       4,
		AttributeCount: 15,
		TotalRowCount:  4,
		ContinueIndex:  0,
		SQLResult: []msg.SQLResult{
			{AttributeIndex: 401, ResultLen: 2, Values: []string{"4", "4", "5", "6"}},
			{AttributeIndex: 403, ResultLen: 2, Values: []string{"file1", "file1", "file2", "file3"}},
			{AttributeIndex: 402, ResultLen: 2, Values: slices.Repeat([]string{"1"}, 4)},
			{AttributeIndex: 406, ResultLen: 2, Values: slices.Repeat([]string{"generic"}, 4)},
			{AttributeIndex: 404, ResultLen: 2, Values: []string{"0", "1", "2", "3"}},
			{AttributeIndex: 407, ResultLen: 2, Values: []string{"1024000", "1024000", "100", "1024000"}},
			{AttributeIndex: 411, ResultLen: 2, Values: slices.Repeat([]string{"rods"}, 4)},
			{AttributeIndex: 412, ResultLen: 2, Values: slices.Repeat([]string{"zone"}, 4)},
			{AttributeIndex: 415, ResultLen: 2, Values: []string{"checksum", "checksum", "", ""}},
			{AttributeIndex: 413, ResultLen: 2, Values: []string{"2", "4", "0", "1"}},
			{AttributeIndex: 409, ResultLen: 2, Values: []string{"resc1", "resc2", "resc1", "resc2"}},
			{AttributeIndex: 410, ResultLen: 2, Values: []string{"/path1", "/path2", "/path3", "/path4"}},
			{AttributeIndex: 422, ResultLen: 2, Values: []string{"demoResc;resc1", "demoResc;resc2", "demoResc;resc1", "demoResc;resc2"}},
			{AttributeIndex: 419, ResultLen: 2, Values: slices.Repeat([]string{"10000"}, 4)},
			{AttributeIndex: 420, ResultLen: 2, Values: slices.Repeat([]string{"10000"}, 4)},
		},
	},
}

func TestAutocomplete2(t *testing.T) {
	app := testApp(t)

	app.AddResponses(responses)

	cmd := app.mkdir()
	cmd.SetContext(t.Context())

	opts, directive := app.CompleteArgs(cmd, []string{}, "/testzone/hom")
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

func TestXOpen(t *testing.T) {
	app := testApp(t)

	cmd := app.Command()
	cmd.SetContext(t.Context())

	cmd.SetArgs([]string{"x-open", "iron://version"})

	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
}
