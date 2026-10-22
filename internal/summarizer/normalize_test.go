package summarizer

import (
	"fmt"
	"testing"
)

func TestNormalizeLine(t *testing.T) {
	tests := []struct {
		in       string
		wantSev  string
		wantTmpl string
	}{
		{
			"2026-10-22T09:15:02Z [ERROR] Connection refused to 10.0.0.15:5432 after 350ms",
			SevError,
			"Connection refused to <IP>:<PORT> after <NUM>",
		},
		{
			"2026-10-22 09:15:02.111 WARN retry attempt=3 req=550e8400-e29b-41d4-a716-446655440000",
			SevWarn,
			"retry attempt=<NUM> req=<UUID>",
		},
		{
			"INFO open /var/lib/postgresql/data/pg_hba.conf block 0x1a2b3c4d",
			SevInfo,
			"open <PATH> block <HEX>",
		},
	}
	for _, tt := range tests {
		sev, tmpl := NormalizeLine(tt.in)
		if sev != tt.wantSev || tmpl != tt.wantTmpl {
			t.Errorf("NormalizeLine(%q) = (%q, %q), want (%q, %q)",
				tt.in, sev, tmpl, tt.wantSev, tt.wantTmpl)
		}
	}
}

func TestClusterLogsDeduplicatesThousandsOfLines(t *testing.T) {
	lines := make([]string, 0, 1005)
	for i := 0; i < 1000; i++ {
		lines = append(lines, fmt.Sprintf("2026-10-22T09:00:%02dZ [ERROR] Connection refused to 10.0.1.%d:5432", i%60, i%254))
	}
	for i := 0; i < 5; i++ {
		lines = append(lines, fmt.Sprintf("2026-10-22T09:01:%02dZ [WARN] slow query took %dms", i, 100+i))
	}

	clusters := ClusterLogs(lines)
	if len(clusters) != 2 {
		t.Fatalf("len(clusters) = %d, want 2", len(clusters))
	}
	if clusters[0].Severity != SevError || clusters[0].Count != 1000 || clusters[0].Template != "Connection refused to <IP>:<PORT>" {
		t.Errorf("clusters[0] = %+v", clusters[0])
	}
	if clusters[1].Severity != SevWarn || clusters[1].Count != 5 {
		t.Errorf("clusters[1] = %+v", clusters[1])
	}
}

func BenchmarkNormalizeLine(b *testing.B) {
	line := "2026-10-22T09:15:02Z [ERROR] Connection refused to 10.0.0.15:5432 req=550e8400-e29b-41d4-a716-446655440000 after 350ms"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = NormalizeLine(line)
	}
}
