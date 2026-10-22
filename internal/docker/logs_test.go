package docker

import (
	"bytes"
	"context"
	"encoding/binary"
	"net/http"
	"reflect"
	"testing"
)

func muxFrame(stream byte, payload string) []byte {
	var b bytes.Buffer
	var hdr [8]byte
	hdr[0] = stream
	binary.BigEndian.PutUint32(hdr[4:8], uint32(len(payload)))
	b.Write(hdr[:])
	b.WriteString(payload)
	return b.Bytes()
}

func TestContainerLogsMultiplexed(t *testing.T) {
	var stream bytes.Buffer
	stream.Write(muxFrame(1, "2026-10-22 INFO starting server on :8080\n"))
	stream.Write(muxFrame(2, "2026-10-22 ERROR connection refused to 10.0.0.5:5432\n"))

	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/containers/api/logs" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Write(stream.Bytes())
	}))

	got, err := c.ContainerLogs(context.Background(), "api", 100)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"2026-10-22 INFO starting server on :8080",
		"2026-10-22 ERROR connection refused to 10.0.0.5:5432",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestContainerLogsRawTTYAndTailCap(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("line1\nline2\nline3\n"))
	}))
	got, err := c.ContainerLogs(context.Background(), "tty", 2)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"line2", "line3"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
