package telemetry

import (
	"path/filepath"
	"testing"
	"time"
)

func TestStoreBatchFlushAndPrune(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.duckdb")
	st := NewStore(dbPath, time.Hour)
	defer st.Close()

	now := time.Now()
	// Insert 500 records to trigger an immediate batch flush via sync.Pool.
	for i := 0; i < BatchMaxRecords; i++ {
		st.Append(LogRecord{
			Timestamp: now.Add(-10 * time.Minute),
			Source:    "svc",
			Severity:  "INFO",
			Body:      "hello",
		})
	}

	stats := st.Stats("127.0.0.1:4317", "127.0.0.1:4318", true)
	if stats.BatchesFlushed != 1 || stats.StoredRows != BatchMaxRecords {
		t.Fatalf("BatchesFlushed=%d StoredRows=%d", stats.BatchesFlushed, stats.StoredRows)
	}
	if stats.MemoryLimit != "64MB" || stats.Threads != 2 {
		t.Errorf("unexpected DuckDB limits: %+v", stats)
	}

	deleted := st.Prune(now)
	if deleted != BatchMaxRecords {
		t.Errorf("Prune deleted %d, want %d", deleted, BatchMaxRecords)
	}
	if after := st.Stats("", "", false).StoredRows; after != 0 {
		t.Errorf("StoredRows after prune = %d, want 0", after)
	}
}
