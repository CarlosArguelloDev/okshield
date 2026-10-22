// Package docker is a minimal, allocation-conscious client for the Docker
// Engine API. It deliberately avoids the full moby SDK to keep the binary and
// resident memory small on SBCs such as the Raspberry Pi.
package docker

import (
	"fmt"
	"net/url"
	"runtime"
	"strings"
)

// Endpoint describes how to reach the Docker daemon.
type Endpoint struct {
	Scheme string // "unix", "tcp" or "npipe"
	Addr   string // socket path, host:port or pipe path
}

func (e Endpoint) String() string { return e.Scheme + "://" + e.Addr }

// DefaultHost returns the platform default daemon address.
func DefaultHost() string {
	if runtime.GOOS == "windows" {
		return "npipe:////./pipe/docker_engine"
	}
	return "unix:///var/run/docker.sock"
}

// ParseHost parses a DOCKER_HOST-style value. An empty string resolves to
// DefaultHost(). TLS (https / DOCKER_TLS_VERIFY) is not supported yet.
func ParseHost(host string) (Endpoint, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		host = DefaultHost()
	}
	scheme, rest, ok := strings.Cut(host, "://")
	if !ok {
		return Endpoint{}, fmt.Errorf("docker: invalid host %q: missing scheme", host)
	}
	switch scheme {
	case "unix":
		if rest == "" {
			return Endpoint{}, fmt.Errorf("docker: invalid host %q: empty socket path", host)
		}
		return Endpoint{Scheme: "unix", Addr: rest}, nil
	case "npipe":
		// npipe:////./pipe/docker_engine -> \\.\pipe\docker_engine
		p := strings.ReplaceAll(strings.TrimPrefix(rest, "//"), "/", `\`)
		if !strings.HasPrefix(p, `\\`) {
			p = `\\` + strings.TrimLeft(p, `\`)
		}
		return Endpoint{Scheme: "npipe", Addr: p}, nil
	case "tcp", "http":
		u, err := url.Parse("tcp://" + rest)
		if err != nil || u.Host == "" {
			return Endpoint{}, fmt.Errorf("docker: invalid host %q", host)
		}
		addr := u.Host
		if u.Port() == "" {
			addr += ":2375"
		}
		return Endpoint{Scheme: "tcp", Addr: addr}, nil
	default:
		return Endpoint{}, fmt.Errorf("docker: unsupported scheme %q", scheme)
	}
}
