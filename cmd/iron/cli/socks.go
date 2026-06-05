package cli

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"

	"github.com/kuleuven/iron"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/proxy"
)

// SOCKSProxyEnv is the environment variable consulted by FileLoader (and
// WrapLoaderWithSOCKS5) to configure an outbound SOCKS5 proxy for iRODS
// connections. The value can either be a bare host:port pair or a fully
// qualified socks5:// (or socks5h://) URL with optional user:password
// credentials.
const SOCKSProxyEnv = "SOCKS_PROXY"

// WrapLoaderWithSOCKS5 returns a Loader that routes connections through the
// given SOCKS5 proxy. If proxyURL is empty, loader is returned unchanged.
// The proxyURL accepts the same forms as the SOCKS_PROXY environment
// variable: a bare host:port (socks5:// is assumed) or a full
// socks5://[user:pass@]host:port URL.
func WrapLoaderWithSOCKS5(loader Loader, proxyURL string) Loader {
	if proxyURL == "" {
		return loader
	}

	return func(ctx context.Context, zone string) (iron.Env, iron.DialFunc, error) {
		env, _, err := loader(ctx, zone)
		if err != nil {
			return env, nil, err
		}

		wrapped, werr := wrapDialFuncWithSOCKS5(proxyURL)
		if werr != nil {
			return env, nil, werr
		}

		logrus.Infof("routing iRODS connections via SOCKS proxy %s", proxyURL)

		return env, wrapped, nil
	}
}

// wrapDialFuncWithSOCKS5 returns a DialFunc that routes connections through
// the SOCKS5 proxy described by proxyURL.
func wrapDialFuncWithSOCKS5(proxyURL string) (iron.DialFunc, error) {
	u, err := parseSOCKSProxy(proxyURL)
	if err != nil {
		return nil, err
	}

	return func(ctx context.Context, env iron.Env, _ string) (net.Conn, error) {
		// Build a forward dialer that respects env.DialTimeout and the
		// caller's context, so the SOCKS handshake honours the same
		// constraints as a direct connection.
		forward := &net.Dialer{
			Timeout: env.DialTimeout,
		}

		dialer, err := proxy.FromURL(u, forward)
		if err != nil {
			return nil, fmt.Errorf("invalid SOCKS proxy %q: %w", proxyURL, err)
		}

		addr := net.JoinHostPort(env.Host, fmt.Sprintf("%d", env.Port))

		if cd, ok := dialer.(proxy.ContextDialer); ok {
			return cd.DialContext(ctx, "tcp", addr)
		}

		return dialer.Dial("tcp", addr)
	}, nil
}

// parseSOCKSProxy parses a SOCKS proxy specification. It accepts either a
// fully qualified URL (e.g. socks5://user:pass@host:1080) or a bare
// host:port string, in which case socks5:// is assumed.
func parseSOCKSProxy(s string) (*url.URL, error) {
	if !strings.Contains(s, "://") {
		s = "socks5://" + s
	}

	u, err := url.Parse(s)
	if err != nil {
		return nil, fmt.Errorf("invalid SOCKS proxy %q: %w", s, err)
	}

	switch u.Scheme {
	case "socks5", "socks5h":
	default:
		return nil, fmt.Errorf("unsupported SOCKS proxy scheme %q", u.Scheme)
	}

	if u.Host == "" {
		return nil, fmt.Errorf("SOCKS proxy %q is missing host", s)
	}

	return u, nil
}

// socksProxyFromEnv returns the SOCKS proxy configured via the SOCKS_PROXY
// environment variable, or an empty string if unset.
func socksProxyFromEnv() string {
	return os.Getenv(SOCKSProxyEnv)
}
