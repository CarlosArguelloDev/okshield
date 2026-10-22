package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// Client talks to the Docker Engine API over unix socket, TCP or named pipe.
// It is safe for concurrent use.
type Client struct {
	endpoint Endpoint
	http     *http.Client
	base     string
}

// NewClient builds a client for a DOCKER_HOST-style address ("" = default).
// No connection is made until the first request.
func NewClient(host string) (*Client, error) {
	ep, err := ParseHost(host)
	if err != nil {
		return nil, err
	}
	var d net.Dialer
	tr := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			switch ep.Scheme {
			case "unix":
				return d.DialContext(ctx, "unix", ep.Addr)
			case "npipe":
				return dialPipe(ctx, ep.Addr)
			default:
				return d.DialContext(ctx, "tcp", ep.Addr)
			}
		},
		// Keep the pool tiny: one TUI, a handful of concurrent requests.
		MaxIdleConns:        2,
		MaxIdleConnsPerHost: 2,
		IdleConnTimeout:     30 * time.Second,
		DisableCompression:  true,
	}
	return &Client{
		endpoint: ep,
		http:     &http.Client{Transport: tr},
		// Host is ignored for unix/npipe; unversioned paths use the daemon's
		// current API version (Docker 29 rejects API < 1.44).
		base: "http://docker",
	}, nil
}

// Endpoint returns the resolved daemon endpoint.
func (c *Client) Endpoint() Endpoint { return c.endpoint }

// Close releases idle connections.
func (c *Client) Close() { c.http.CloseIdleConnections() }

// APIError is returned for non-2xx responses.
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("docker api: %d %s", e.Status, e.Message)
}

// IsNotFound reports whether err is a 404 from the daemon.
func IsNotFound(err error) bool {
	var ae *APIError
	return errors.As(err, &ae) && ae.Status == http.StatusNotFound
}

// do performs a GET request and returns the open response body on success.
// The caller must close it.
func (c *Client) do(ctx context.Context, path string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("docker: %s: %w", c.endpoint, err)
	}
	if resp.StatusCode/100 != 2 {
		defer resp.Body.Close()
		var body struct {
			Message string `json:"message"`
		}
		_ = json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&body)
		return nil, &APIError{Status: resp.StatusCode, Message: body.Message}
	}
	return resp.Body, nil
}

// getJSON performs a GET request and decodes the JSON body into out.
func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	body, err := c.do(ctx, path)
	if err != nil {
		return err
	}
	defer body.Close()
	if err := json.NewDecoder(body).Decode(out); err != nil {
		return fmt.Errorf("docker: decode %s: %w", path, err)
	}
	return nil
}

// Ping checks daemon reachability.
func (c *Client) Ping(ctx context.Context) error {
	body, err := c.do(ctx, "/_ping")
	if err != nil {
		return err
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(body, 64))
	return body.Close()
}

// Version returns the daemon server version (e.g. "29.0.1").
func (c *Client) Version(ctx context.Context) (string, error) {
	var v struct {
		Version string `json:"Version"`
	}
	if err := c.getJSON(ctx, "/version", &v); err != nil {
		return "", err
	}
	return v.Version, nil
}
