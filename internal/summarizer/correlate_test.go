package summarizer

import (
	"testing"

	"github.com/CarlosArguelloDev/okshield/internal/docker"
)

func TestAnalyzeCorrelatesStateStatsAndLogs(t *testing.T) {
	inputs := []ContainerInput{
		{
			Detail: docker.ContainerDetail{
				Container: docker.Container{Name: "healthy-web", Image: "nginx:alpine", State: "running"},
				Running:   true,
			},
			Stats: docker.Stats{CPUPercent: 3.5, MemPercent: 12.0},
			Logs:  []string{"2026-10-22T09:00:00Z INFO GET /health 200"},
		},
		{
			Detail: docker.ContainerDetail{
				Container:    docker.Container{Name: "oom-worker", Image: "worker:1", State: "exited"},
				Running:      false,
				OOMKilled:    true,
				ExitCode:     137,
				RestartCount: 3,
			},
			Stats: docker.Stats{MemPercent: 98.4},
			Logs: []string{
				"2026-10-22T09:01:00Z ERROR out of memory allocating 67108864 bytes",
				"2026-10-22T09:01:01Z ERROR out of memory allocating 134217728 bytes",
			},
		},
	}

	rep := Analyze(inputs, 5)
	if rep.Total != 2 || rep.Running != 1 || rep.Failed != 1 {
		t.Fatalf("counts: total=%d running=%d failed=%d", rep.Total, rep.Running, rep.Failed)
	}
	top := rep.Containers[0]
	if top.Name != "oom-worker" || top.Healthy {
		t.Fatalf("expected oom-worker first and unhealthy, got %+v", top)
	}
	if len(top.Anomalies) < 3 {
		t.Errorf("expected >=3 anomalies (OOM, ExitCode, Restarts), got %v", top.Anomalies)
	}
	if len(top.TopClusters) != 1 || top.TopClusters[0].Count != 2 {
		t.Errorf("expected 2 OOM lines clustered into 1 template, got %+v", top.TopClusters)
	}
}
