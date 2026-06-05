package cli

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strings"
	"testing"

	"github.com/kuleuven/iron"
	"golang.org/x/net/proxy"
)

func TestParseSOCKSProxy(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantHost  string
		wantUser  string
		wantPass  string
		wantErr   bool
		errSubstr string
	}{
		{
			name:     "bare host:port",
			input:    "127.0.0.1:1080",
			wantHost: "127.0.0.1:1080",
		},
		{
			name:     "socks5 url",
			input:    "socks5://proxy.example.com:1080",
			wantHost: "proxy.example.com:1080",
		},
		{
			name:     "socks5h url with credentials",
			input:    "socks5h://alice:secret@proxy.example.com:1080",
			wantHost: "proxy.example.com:1080",
			wantUser: "alice",
			wantPass: "secret",
		},
		{
			name:      "unsupported scheme",
			input:     "http://proxy.example.com:8080",
			wantErr:   true,
			errSubstr: "unsupported SOCKS proxy scheme",
		},
		{
			name:      "missing host",
			input:     "socks5://",
			wantErr:   true,
			errSubstr: "missing host",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := parseSOCKSProxy(tt.input)
			if tt.wantErr {
				assertParseSOCKSError(t, err, tt.errSubstr)

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			assertParsedSOCKSURL(t, u, tt.wantHost, tt.wantUser, tt.wantPass)
		})
	}
}

func assertParseSOCKSError(t *testing.T, err error, substr string) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if substr != "" && !strings.Contains(err.Error(), substr) {
		t.Fatalf("error %q does not contain %q", err.Error(), substr)
	}
}

func assertParsedSOCKSURL(t *testing.T, u *url.URL, wantHost, wantUser, wantPass string) {
	t.Helper()

	if u.Host != wantHost {
		t.Errorf("host = %q, want %q", u.Host, wantHost)
	}

	if u.User.Username() != wantUser {
		t.Errorf("user = %q, want %q", u.User.Username(), wantUser)
	}

	if pw, _ := u.User.Password(); pw != wantPass {
		t.Errorf("password = %q, want %q", pw, wantPass)
	}
}

func TestWrapLoaderWithSOCKS5_EmptyProxyReturnsLoader(t *testing.T) {
	called := false
	base := Loader(func(_ context.Context, _ string) (iron.Env, iron.DialFunc, error) {
		called = true

		return iron.Env{Host: "irods.example.com", Port: 1247}, iron.DefaultDialFunc, nil
	})

	wrapped := WrapLoaderWithSOCKS5(base, "")

	_, _, err := wrapped(t.Context(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !called {
		t.Fatalf("base loader was not invoked")
	}
}

func TestWrapLoaderWithSOCKS5_PropagatesLoaderError(t *testing.T) {
	want := errors.New("boom")
	base := Loader(func(_ context.Context, _ string) (iron.Env, iron.DialFunc, error) {
		return iron.Env{}, nil, want
	})

	wrapped := WrapLoaderWithSOCKS5(base, "127.0.0.1:1080")

	_, _, err := wrapped(t.Context(), "")
	if !errors.Is(err, want) {
		t.Fatalf("got error %v, want %v", err, want)
	}
}

func TestWrapLoaderWithSOCKS5_InvalidProxy(t *testing.T) {
	base := Loader(func(_ context.Context, _ string) (iron.Env, iron.DialFunc, error) {
		return iron.Env{Host: "irods.example.com", Port: 1247}, iron.DefaultDialFunc, nil
	})

	wrapped := WrapLoaderWithSOCKS5(base, "http://proxy.example.com:8080")

	_, dialer, err := wrapped(t.Context(), "")
	if err == nil {
		t.Fatalf("expected error for unsupported scheme, got dialer=%v", dialer)
	}
}

func TestWrapLoaderWithSOCKS5_RoutesThroughProxy(t *testing.T) {
	// Spin up a tiny TCP listener that records the connection and hangs up.
	// We only need to assert that the dialer attempts to talk to the proxy
	// address rather than the iRODS host directly.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	defer ln.Close()

	gotConn := make(chan struct{}, 1)

	go func() {
		conn, aerr := ln.Accept()
		if aerr != nil {
			return
		}

		gotConn <- struct{}{}

		conn.Close()
	}()

	base := Loader(func(_ context.Context, _ string) (iron.Env, iron.DialFunc, error) {
		// Use a host that would fail to resolve/connect if the proxy were
		// bypassed, ensuring any successful connection went via the proxy.
		return iron.Env{Host: "irods.invalid", Port: 1247}, iron.DefaultDialFunc, nil
	})

	wrapped := WrapLoaderWithSOCKS5(base, "socks5://"+ln.Addr().String())

	_, dialer, err := wrapped(t.Context(), "")
	if err != nil {
		t.Fatalf("wrapped loader: %v", err)
	}

	// The proxy listener will hang up immediately, so DialContext will return
	// an error — but gotConn firing proves the SOCKS path was taken. Run the
	// dial in a goroutine so we don't block on the (failing) SOCKS handshake.
	go func() {
		_, _ = dialer(t.Context(), iron.Env{Host: "irods.invalid", Port: 1247}, "test")
	}()

	select {
	case <-gotConn:
	case <-t.Context().Done():
		t.Fatalf("proxy never received a connection: %v", t.Context().Err())
	}

	// Sanity-check that proxy.FromURL still resolves our URL — this guards
	// against the dependency being accidentally dropped.
	u, perr := url.Parse("socks5://" + ln.Addr().String())
	if perr != nil {
		t.Fatalf("url.Parse: %v", perr)
	}

	if _, ferr := proxy.FromURL(u, proxy.Direct); ferr != nil {
		t.Fatalf("proxy.FromURL: %v", ferr)
	}
}
