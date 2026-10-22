// Package telemetry provides fixed-size in-memory ring buffers, embedded OTLP
// ingestion and batched historical storage in DuckDB.
package telemetry

import (
	"sync"
	"time"
)

// DefaultRingCapacity is the maximum number of log entries kept in RAM per
// buffer (AGENTS.md §5: max 500 renderable lines).
const DefaultRingCapacity = 500

// LogRecord is a compact telemetry log entry stored in the hot-path ring buffer.
type LogRecord struct {
	Timestamp time.Time
	Source    string // container name or OTLP service.name
	Severity  string // ERROR, WARN, INFO, DEBUG
	Body      string
}

// RingBuffer is a thread-safe, fixed-capacity circular buffer of LogRecords.
// Once initialized, Push performs zero slice allocations.
type RingBuffer struct {
	mu    sync.RWMutex
	buf   []LogRecord
	cap   int
	head  int // next write index
	count int
	total uint64 // total records ever pushed
}

// NewRingBuffer allocates a ring buffer with the given fixed capacity.
// If capacity <= 0, DefaultRingCapacity (500) is used.
func NewRingBuffer(capacity int) *RingBuffer {
	if capacity <= 0 {
		capacity = DefaultRingCapacity
	}
	return &RingBuffer{
		buf: make([]LogRecord, capacity),
		cap: capacity,
	}
}

// Push inserts a record into the ring buffer, overwriting the oldest entry
// when full.
func (r *RingBuffer) Push(rec LogRecord) {
	r.mu.Lock()
	r.buf[r.head] = rec
	r.head = (r.head + 1) % r.cap
	if r.count < r.cap {
		r.count++
	}
	r.total++
	r.mu.Unlock()
}

// Snapshot returns a chronological copy (oldest to newest) of the buffered records.
func (r *RingBuffer) Snapshot() []LogRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.count == 0 {
		return nil
	}
	out := make([]LogRecord, r.count)
	start := (r.head - r.count + r.cap) % r.cap
	for i := 0; i < r.count; i++ {
		out[i] = r.buf[(start+i)%r.cap]
	}
	return out
}

// Len returns the current number of stored records (<= Cap).
func (r *RingBuffer) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.count
}

// Cap returns the fixed capacity of the ring buffer.
func (r *RingBuffer) Cap() int { return r.cap }

// TotalPushed returns the cumulative number of records pushed since creation.
func (r *RingBuffer) TotalPushed() uint64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.total
}
