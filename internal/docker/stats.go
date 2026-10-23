package docker

import (
	"context"
	"net/url"
)

// Stats holds a single resource-usage snapshot of a running container.
type Stats struct {
	CPUPercent    float64
	MemUsage      uint64
	MemLimit      uint64
	MemPercent    float64
	NetRxBytes    uint64
	NetTxBytes    uint64
	BlkReadBytes  uint64
	BlkWriteBytes uint64
	PIDs          uint64
	Throttled     uint64 // number of CPU throttled periods
}

type apiStats struct {
	CPUStats struct {
		CPUUsage struct {
			TotalUsage  uint64   `json:"total_usage"`
			PerCPUUsage []uint64 `json:"percpu_usage"`
		} `json:"cpu_usage"`
		SystemCPUUsage uint64 `json:"system_cpu_usage"`
		OnlineCPUs     uint32 `json:"online_cpus"`
		ThrottlingData struct {
			ThrottledPeriods uint64 `json:"throttled_periods"`
		} `json:"throttling_data"`
	} `json:"cpu_stats"`
	PreCPUStats struct {
		CPUUsage struct {
			TotalUsage uint64 `json:"total_usage"`
		} `json:"cpu_usage"`
		SystemCPUUsage uint64 `json:"system_cpu_usage"`
	} `json:"precpu_stats"`
	MemoryStats struct {
		Usage uint64            `json:"usage"`
		Limit uint64            `json:"limit"`
		Stats map[string]uint64 `json:"stats"`
	} `json:"memory_stats"`
	Networks map[string]struct {
		RxBytes uint64 `json:"rx_bytes"`
		TxBytes uint64 `json:"tx_bytes"`
	} `json:"networks"`
	BlkioStats struct {
		IoServiceBytesRecursive []struct {
			Op    string `json:"op"`
			Value uint64 `json:"value"`
		} `json:"io_service_bytes_recursive"`
	} `json:"blkio_stats"`
	PidsStats struct {
		Current uint64 `json:"current"`
	} `json:"pids_stats"`
}

// ContainerStats fetches a one-shot stats snapshot (stream=false).
func (c *Client) ContainerStats(ctx context.Context, idOrName string) (Stats, error) {
	var raw apiStats
	path := "/containers/" + url.PathEscape(idOrName) + "/stats?stream=false"
	if err := c.getJSON(ctx, path, &raw); err != nil {
		return Stats{}, err
	}
	return computeStats(raw), nil
}

func computeStats(r apiStats) Stats {
	var s Stats
	s.PIDs = r.PidsStats.Current
	s.Throttled = r.CPUStats.ThrottlingData.ThrottledPeriods

	// Memory usage excluding page cache (matches `docker stats` CLI calculation).
	usage := r.MemoryStats.Usage
	if inactiveFile, ok := r.MemoryStats.Stats["inactive_file"]; ok && usage > inactiveFile {
		usage -= inactiveFile
	} else if cache, ok := r.MemoryStats.Stats["cache"]; ok && usage > cache {
		usage -= cache
	}
	s.MemUsage = usage
	s.MemLimit = r.MemoryStats.Limit
	if s.MemLimit > 0 {
		s.MemPercent = (float64(s.MemUsage) / float64(s.MemLimit)) * 100.0
	}

	// CPU percentage delta.
	cpuDelta := float64(r.CPUStats.CPUUsage.TotalUsage) - float64(r.PreCPUStats.CPUUsage.TotalUsage)
	sysDelta := float64(r.CPUStats.SystemCPUUsage) - float64(r.PreCPUStats.SystemCPUUsage)
	cpus := float64(r.CPUStats.OnlineCPUs)
	if cpus == 0 {
		cpus = float64(len(r.CPUStats.CPUUsage.PerCPUUsage))
	}
	if cpus == 0 {
		cpus = 1
	}
	if sysDelta > 0 && cpuDelta > 0 {
		s.CPUPercent = (cpuDelta / sysDelta) * cpus * 100.0
	}

	for _, n := range r.Networks {
		s.NetRxBytes += n.RxBytes
		s.NetTxBytes += n.TxBytes
	}
	for _, b := range r.BlkioStats.IoServiceBytesRecursive {
		switch b.Op {
		case "read", "Read", "READ":
			s.BlkReadBytes += b.Value
		case "write", "Write", "WRITE":
			s.BlkWriteBytes += b.Value
		}
	}
	return s
}
