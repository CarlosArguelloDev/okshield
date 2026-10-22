package telemetry

import (
	"fmt"
	"testing"
	"time"
)

func TestRingBufferWrapAround(t *testing.T) {
	rb := NewRingBuffer(3)
	for i := 1; i <= 5; i++ {
		rb.Push(LogRecord{
			Timestamp: time.Unix(int64(i), 0),
			Source:    "api",
			Severity:  "INFO",
			Body:      fmt.Sprintf("msg-%d", i),
		})
	}
	if rb.Len() != 3 || rb.Cap() != 3 || rb.TotalPushed() != 5 {
		t.Fatalf("len=%d cap=%d total=%d", rb.Len(), rb.Cap(), rb.TotalPushed())
	}
	snap := rb.Snapshot()
	want := []string{"msg-3", "msg-4", "msg-5"}
	for i, w := range want {
		if snap[i].Body != w {
			t.Errorf("snap[%d] = %q, want %q", i, snap[i].Body, w)
		}
	}
}

func TestRingBufferDefaultCap(t *testing.T) {
	rb := NewRingBuffer(0)
	if rb.Cap() != DefaultRingCapacity {
		t.Errorf("Cap() = %d, want %d", rb.Cap(), DefaultRingCapacity)
	}
	if rb.Snapshot() != nil {
		t.Errorf("empty Snapshot should be nil")
	}
}

func BenchmarkRingBufferPush(b *testing.B) {
	rb := NewRingBuffer(DefaultRingCapacity)
	rec := LogRecord{Timestamp: time.Now(), Source: "api", Severity: "ERROR", Body: "connection refused"}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rb.Push(rec)
	}
}
