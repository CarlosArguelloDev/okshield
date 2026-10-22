package summarizer

import (
	"fmt"
	"sort"
	"time"

	"github.com/CarlosArguelloDev/okshield/internal/docker"
)

// Thresholds for resource anomaly detection.
const (
	HighCPUThreshold = 85.0 // %
	HighMemThreshold = 85.0 // %
)

// ContainerInput aggregates the signals of a single container for the
// deterministic correlator.
type ContainerInput struct {
	Detail docker.ContainerDetail
	Stats  docker.Stats
	Logs   []string
}

// ContainerPulse is the synthesized summary for one container.
type ContainerPulse struct {
	Name         string
	Image        string
	State        string
	Healthy      bool
	Anomalies    []string     // deterministic state/resource anomalies
	TopClusters  []LogCluster // top deduplicated log templates
	ErrorCount   int          // total ERROR log lines
	WarnCount    int          // total WARN log lines
	TotalLines   int          // total raw log lines analyzed
	CPUPercent   float64
	MemPercent   float64
	RestartCount int
	ExitCode     int
	OOMKilled    bool
}

// Report is the executive pulse report returned by /whatsup (/pulse).
type Report struct {
	GeneratedAt time.Time
	Total       int
	Running     int
	Degraded    int
	Failed      int
	Containers  []ContainerPulse
}

// Analyze correlates container states, resource spikes and deduplicated log
// clusters into a compact, deterministic Report.
func Analyze(inputs []ContainerInput, maxClusters int) Report {
	if maxClusters <= 0 {
		maxClusters = 5
	}
	rep := Report{
		GeneratedAt: time.Now(),
		Total:       len(inputs),
		Containers:  make([]ContainerPulse, 0, len(inputs)),
	}

	for _, in := range inputs {
		d := in.Detail
		st := in.Stats
		clusters := ClusterLogs(in.Logs)

		errLines, warnLines := 0, 0
		for _, cl := range clusters {
			switch cl.Severity {
			case SevError:
				errLines += cl.Count
			case SevWarn:
				warnLines += cl.Count
			}
		}

		var anomalies []string
		if d.OOMKilled {
			anomalies = append(anomalies, "OOMKilled detectado por el kernel (memoria agotada)")
		}
		if !d.Running && d.ExitCode != 0 {
			anomalies = append(anomalies, fmt.Sprintf("Proceso terminó con ExitCode=%d", d.ExitCode))
		}
		if d.Restarting {
			anomalies = append(anomalies, "Contenedor en bucle de reinicio (Restarting)")
		}
		if d.RestartCount > 0 {
			anomalies = append(anomalies, fmt.Sprintf("%d reinicios acumulados", d.RestartCount))
		}
		if d.Health == "unhealthy" {
			anomalies = append(anomalies, "Healthcheck en estado UNHEALTHY")
		}
		if st.CPUPercent >= HighCPUThreshold {
			anomalies = append(anomalies, fmt.Sprintf("Pico de CPU: %.1f%%", st.CPUPercent))
		}
		if st.MemPercent >= HighMemThreshold {
			anomalies = append(anomalies, fmt.Sprintf("Presión de RAM: %.1f%%", st.MemPercent))
		}
		if st.Throttled > 0 {
			anomalies = append(anomalies, fmt.Sprintf("CPU throttling activo (%d periodos)", st.Throttled))
		}

		top := clusters
		if len(top) > maxClusters {
			top = top[:maxClusters]
		}

		failed := !d.Running && (d.ExitCode != 0 || d.OOMKilled || d.Dead || d.State == "exited")
		degraded := !failed && (len(anomalies) > 0 || errLines > 0)

		switch {
		case failed:
			rep.Failed++
		case degraded:
			rep.Degraded++
		default:
			if d.Running {
				rep.Running++
			}
		}

		rep.Containers = append(rep.Containers, ContainerPulse{
			Name:         d.Name,
			Image:        d.Image,
			State:        d.State,
			Healthy:      !failed && !degraded,
			Anomalies:    anomalies,
			TopClusters:  top,
			ErrorCount:   errLines,
			WarnCount:    warnLines,
			TotalLines:   len(in.Logs),
			CPUPercent:   st.CPUPercent,
			MemPercent:   st.MemPercent,
			RestartCount: d.RestartCount,
			ExitCode:     d.ExitCode,
			OOMKilled:    d.OOMKilled,
		})
	}

	// Sort unhealthy/anomalous containers first so the operator sees issues at the top.
	sort.Slice(rep.Containers, func(a, b int) bool {
		ca, cb := rep.Containers[a], rep.Containers[b]
		if ca.Healthy != cb.Healthy {
			return !ca.Healthy
		}
		if len(ca.Anomalies) != len(cb.Anomalies) {
			return len(ca.Anomalies) > len(cb.Anomalies)
		}
		if ca.ErrorCount != cb.ErrorCount {
			return ca.ErrorCount > cb.ErrorCount
		}
		return ca.Name < cb.Name
	})

	return rep
}
