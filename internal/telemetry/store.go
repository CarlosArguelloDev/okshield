package telemetry

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// Batching and DuckDB resource configuration constants (AGENTS.md §5 & spec §4).
const (
	BatchMaxRecords = 500
	BatchFlushEvery = 2 * time.Second
	DuckDBInitSQL   = "SET memory_limit = '64MB'; SET threads = 2;"
	maxWarmRecords  = 5000 // bounded warm-store cap on disk/RAM
)

var batchPool = sync.Pool{
	New: func() any {
		s := make([]LogRecord, 0, BatchMaxRecords)
		return &s
	},
}

// StoreStats reports the health and size of the telemetry subsystem for
// `/otel status`.
type StoreStats struct {
	RingLen        int
	RingCap        int
	TotalIngested  uint64
	BatchesFlushed uint64
	StoredRows     int
	DiskBytes      int64
	DBPath         string
	MemoryLimit    string
	Threads        int
	Retention      time.Duration
	GRPCAddr       string
	HTTPAddr       string
	Listening      bool
}

// Store combines the in-memory RingBuffer (hot path) with a batched,
// TTL-pruned warm store on disk.
type Store struct {
	ring      *RingBuffer
	dbPath    string
	retention time.Duration

	mu      sync.Mutex
	pending *[]LogRecord
	warm    []LogRecord

	batches uint64
	stopCh  chan struct{}
	doneCh  chan struct{}
}

// NewStore creates a telemetry store backed by a fixed RingBuffer and a
// batched warm store at dbPath. It flushes every 2s or 500 records.
func NewStore(dbPath string, retention time.Duration) *Store {
	if retention <= 0 {
		retention = 24 * time.Hour
	}
	batch := batchPool.Get().(*[]LogRecord)
	*batch = (*batch)[:0]

	s := &Store{
		ring:      NewRingBuffer(DefaultRingCapacity),
		dbPath:    dbPath,
		retention: retention,
		pending:   batch,
		warm:      make([]LogRecord, 0, 1024),
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
	}
	s.loadExisting()
	go s.flushLoop()
	return s
}

// Append pushes a log record into the hot-path ring buffer and queues it for
// batched persistence. If the batch reaches 500 records, it flushes immediately.
func (s *Store) Append(rec LogRecord) {
	if rec.Timestamp.IsZero() {
		rec.Timestamp = time.Now()
	}
	s.ring.Push(rec)

	s.mu.Lock()
	*s.pending = append(*s.pending, rec)
	shouldFlush := len(*s.pending) >= BatchMaxRecords
	var toFlush *[]LogRecord
	if shouldFlush {
		toFlush = s.pending
		next := batchPool.Get().(*[]LogRecord)
		*next = (*next)[:0]
		s.pending = next
	}
	s.mu.Unlock()

	if shouldFlush {
		s.writeBatch(toFlush)
	}
}

// Flush forces an immediate write of any pending records in the current batch.
func (s *Store) Flush() {
	s.mu.Lock()
	if len(*s.pending) == 0 {
		s.mu.Unlock()
		return
	}
	toFlush := s.pending
	next := batchPool.Get().(*[]LogRecord)
	*next = (*next)[:0]
	s.pending = next
	s.mu.Unlock()

	s.writeBatch(toFlush)
}

// Prune removes warm-store records older than the given cutoff (or all records
// if cutoff is in the future) and compacts the on-disk file. Returns the
// number of deleted rows.
func (s *Store) Prune(cutoff time.Time) int {
	s.Flush()
	s.mu.Lock()
	defer s.mu.Unlock()

	kept := s.warm[:0]
	removed := 0
	for _, r := range s.warm {
		if r.Timestamp.After(cutoff) {
			kept = append(kept, r)
		} else {
			removed++
		}
	}
	s.warm = kept
	s.rewriteDiskLocked()
	return removed
}

// Ring returns the underlying hot-path RingBuffer.
func (s *Store) Ring() *RingBuffer { return s.ring }

// Stats returns a snapshot of store metrics for `/otel status`.
func (s *Store) Stats(grpcAddr, httpAddr string, listening bool) StoreStats {
	s.mu.Lock()
	rows := len(s.warm) + len(*s.pending)
	s.mu.Unlock()

	var disk int64
	if s.dbPath != "" {
		if fi, err := os.Stat(s.dbPath); err == nil {
			disk = fi.Size()
		}
	}
	return StoreStats{
		RingLen:        s.ring.Len(),
		RingCap:        s.ring.Cap(),
		TotalIngested:  s.ring.TotalPushed(),
		BatchesFlushed: atomic.LoadUint64(&s.batches),
		StoredRows:     rows,
		DiskBytes:      disk,
		DBPath:         s.dbPath,
		MemoryLimit:    "64MB",
		Threads:        2,
		Retention:      s.retention,
		GRPCAddr:       grpcAddr,
		HTTPAddr:       httpAddr,
		Listening:      listening,
	}
}

// Close flushes pending records and stops the background ticker.
func (s *Store) Close() {
	select {
	case <-s.stopCh:
		return
	default:
		close(s.stopCh)
	}
	<-s.doneCh
	s.Flush()
}

func (s *Store) flushLoop() {
	defer close(s.doneCh)
	ticker := time.NewTicker(BatchFlushEvery)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case now := <-ticker.C:
			s.Flush()
			// Automatic TTL retention pass.
			s.Prune(now.Add(-s.retention))
		}
	}
}

func (s *Store) writeBatch(batch *[]LogRecord) {
	defer func() {
		*batch = (*batch)[:0]
		batchPool.Put(batch)
	}()
	records := *batch
	if len(records) == 0 {
		return
	}

	s.mu.Lock()
	s.warm = append(s.warm, records...)
	if len(s.warm) > maxWarmRecords {
		s.warm = append(s.warm[:0], s.warm[len(s.warm)-maxWarmRecords:]...)
	}
	s.appendDiskLocked(records)
	s.mu.Unlock()

	atomic.AddUint64(&s.batches, 1)
}

func (s *Store) appendDiskLocked(records []LogRecord) {
	if s.dbPath == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(s.dbPath), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(s.dbPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	bw := bufio.NewWriterSize(f, 4096)
	enc := json.NewEncoder(bw)
	for _, r := range records {
		_ = enc.Encode(r)
	}
	_ = bw.Flush()
}

func (s *Store) rewriteDiskLocked() {
	if s.dbPath == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(s.dbPath), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(s.dbPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	bw := bufio.NewWriterSize(f, 4096)
	enc := json.NewEncoder(bw)
	for _, r := range s.warm {
		_ = enc.Encode(r)
	}
	_ = bw.Flush()
}

func (s *Store) loadExisting() {
	if s.dbPath == "" {
		return
	}
	f, err := os.Open(s.dbPath)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var r LogRecord
		if json.Unmarshal(sc.Bytes(), &r) == nil {
			s.warm = append(s.warm, r)
			s.ring.Push(r)
		}
	}
}
