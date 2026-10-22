package docker

import (
	"context"
	"math"
	"net/http"
	"testing"
)

const sampleStatsJSON = `{
  "cpu_stats": {
    "cpu_usage": {"total_usage": 200000000, "percpu_usage": [100000000, 100000000]},
    "system_cpu_usage": 1000000000,
    "online_cpus": 4,
    "throttling_data": {"throttled_periods": 7}
  },
  "precpu_stats": {
    "cpu_usage": {"total_usage": 100000000},
    "system_cpu_usage": 600000000
  },
  "memory_stats": {
    "usage": 52428800,
    "limit": 104857600,
    "stats": {"inactive_file": 2428800}
  },
  "networks": {
    "eth0": {"rx_bytes": 1024, "tx_bytes": 2048},
    "eth1": {"rx_bytes": 512, "tx_bytes": 256}
  },
  "pids_stats": {"current": 12}
}`

func TestContainerStats(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/containers/api/stats" || r.URL.Query().Get("stream") != "false" {
			t.Errorf("unexpected stats URL: %s", r.URL)
		}
		w.Write([]byte(sampleStatsJSON))
	}))

	st, err := c.ContainerStats(context.Background(), "api")
	if err != nil {
		t.Fatal(err)
	}
	// cpuDelta = 100e6, sysDelta = 400e6, online_cpus = 4 -> (0.25)*4*100 = 100%
	if math.Abs(st.CPUPercent-100.0) > 0.01 {
		t.Errorf("CPUPercent = %.2f, want 100.0", st.CPUPercent)
	}
	if st.MemUsage != 50000000 {
		t.Errorf("MemUsage = %d, want 50000000", st.MemUsage)
	}
	if st.MemLimit != 104857600 || st.MemPercent <= 47.0 || st.MemPercent >= 48.0 {
		t.Errorf("MemLimit=%d MemPercent=%.2f", st.MemLimit, st.MemPercent)
	}
	if st.NetRxBytes != 1536 || st.NetTxBytes != 2304 {
		t.Errorf("Net rx=%d tx=%d", st.NetRxBytes, st.NetTxBytes)
	}
	if st.PIDs != 12 || st.Throttled != 7 {
		t.Errorf("PIDs=%d Throttled=%d", st.PIDs, st.Throttled)
	}
}
