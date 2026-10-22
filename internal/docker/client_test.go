package docker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// handlerTransport serves requests in-process, so tests need no sockets.
type handlerTransport struct{ h http.Handler }

func (ht handlerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	rec := httptest.NewRecorder()
	ht.h.ServeHTTP(rec, r)
	return rec.Result(), nil
}

// newTestClient returns a client backed by an in-memory fake Engine API.
func newTestClient(t *testing.T, h http.Handler) *Client {
	t.Helper()
	c, err := NewClient("tcp://fake:2375")
	if err != nil {
		t.Fatal(err)
	}
	c.http.Transport = handlerTransport{h}
	return c
}

func TestPingAndVersion(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/_ping", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("OK")) })
	mux.HandleFunc("/version", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"Version":"29.9.0","ApiVersion":"1.52"}`))
	})
	c := newTestClient(t, mux)
	ctx := context.Background()
	if err := c.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}
	v, err := c.Version(ctx)
	if err != nil || v != "29.9.0" {
		t.Fatalf("Version = %q, %v", v, err)
	}
}

func TestAPIErrorNotFound(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"No such container: x"}`))
	}))
	err := c.getJSON(context.Background(), "/containers/x/json", &struct{}{})
	if !IsNotFound(err) {
		t.Fatalf("IsNotFound(%v) = false", err)
	}
	if !strings.Contains(err.Error(), "No such container") {
		t.Errorf("error message lost: %v", err)
	}
}
