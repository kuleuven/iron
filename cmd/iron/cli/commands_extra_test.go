package cli

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/kuleuven/iron"
	"github.com/kuleuven/iron/api"
)

func TestLocalPathEndsWithSeparator(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/foo/", true},
		{"/foo", false},
		{"", false},
		{"plain", false},
	}

	for _, tt := range tests {
		if got := localPathEndsWithSeparator(tt.path); got != tt.want {
			t.Errorf("localPathEndsWithSeparator(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestHiddenColumnsSelection(t *testing.T) {
	defaults := []string{"a", "b", "c"}

	tests := []struct {
		name    string
		columns []string
		want    []string
	}{
		{"no changes", nil, []string{"a", "b", "c"}},
		{"add", []string{"+d"}, []string{"a", "b", "c", "d"}},
		{"remove", []string{"-b"}, []string{"a", "c"}},
		{"add and remove", []string{"-a", "+x"}, []string{"b", "c", "x"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := append([]string(nil), defaults...)
			got := hiddenColumnsSelection(tt.columns, input)

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("hiddenColumnsSelection(%v, %v) = %v, want %v", tt.columns, defaults, got, tt.want)
			}
		})
	}
}

type capturingPrinter struct {
	names []string
}

func (c *capturingPrinter) Setup(_, _, _ bool)              {}
func (c *capturingPrinter) Print(name string, _ api.Record) { c.names = append(c.names, name) }
func (c *capturingPrinter) Flush()                          {}

func TestFindFunc(t *testing.T) {
	captured := &capturingPrinter{}

	fn := findFunc(captured)

	if err := fn("/a", nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	boom := errors.New("boom")
	if fn("/x", nil, boom) != boom {
		t.Errorf("expected propagated error %v", boom)
	}

	if len(captured.names) != 1 || captured.names[0] != "/a" {
		t.Errorf("captured.names = %v, want [/a]", captured.names)
	}
}

func TestHostPort(t *testing.T) {
	tests := []struct {
		name string
		host string
		port int
		want string
	}{
		{name: "host and port", host: testExampleHost, port: 1247, want: testExampleHost + ":1247"},
		{name: "host without port", host: testExampleHost, want: testExampleHost},
		{name: "port without host", port: 1247, want: ""},
		{name: "empty", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hostPort(tt.host, tt.port); got != tt.want {
				t.Errorf("hostPort(%q, %d) = %q, want %q", tt.host, tt.port, got, tt.want)
			}
		})
	}
}

func TestWhoamiRows(t *testing.T) {
	app := testApp(t)

	client, err := iron.New(t.Context(), iron.Env{
		Zone:            testTempZone,
		ProxyUsername:   "proxy",
		ProxyZone:       "proxyZone",
		Host:            testExampleHost,
		Port:            1247,
		AuthScheme:      "native",
		DefaultResource: "demoResc",
	}, iron.Option{
		HandshakeFunc: func(context.Context) (iron.Conn, error) {
			return app.mockConn, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	defer client.Close()

	app.Client = client
	app.Workdir = "/remote/workdir"

	createdAt := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	rows := app.whoamiRows(&api.User{
		ID:        42,
		Name:      testAlice,
		Zone:      testTempZone,
		Type:      testRodsUser,
		CreatedAt: createdAt,
	}, []string{"public", "research"}, "/local/workdir")

	want := [][2]string{
		{"User", testAlice},
		{"Zone", testTempZone},
		{"ID", "42"},
		{"Type", testRodsUser},
		{"Proxy user", "proxy#proxyZone"},
		{"Host", testExampleHost + ":1247"},
		{"Authentication", "native"},
		{"Default resource", "demoResc"},
		{"Remote workdir", "/remote/workdir"},
		{"Local workdir", "/local/workdir"},
		{"Created", createdAt.Format(time.RFC3339)},
		{"Groups", "public, research"},
	}

	if !reflect.DeepEqual(rows, want) {
		t.Errorf("whoamiRows() = %#v, want %#v", rows, want)
	}
}
