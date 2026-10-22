package diagnose

import (
	"testing"

	"github.com/CarlosArguelloDev/okshield/internal/docker"
	"github.com/CarlosArguelloDev/okshield/internal/summarizer"
)

func TestClassifyAllCategories(t *testing.T) {
	tests := []struct {
		name      string
		in        summarizer.ContainerInput
		wantCat   Category
		wantLabel string
	}{
		{
			name: "oom killed",
			in: summarizer.ContainerInput{
				Detail: docker.ContainerDetail{
					Container: docker.Container{Name: "mem-hog", Image: "app:1", State: "exited"},
					OOMKilled: true,
					ExitCode:  137,
				},
			},
			wantCat:   CatMemoryExhaustion,
			wantLabel: "OOMKilled",
		},
		{
			name: "volume permission error",
			in: summarizer.ContainerInput{
				Detail: docker.ContainerDetail{
					Container: docker.Container{
						Name:   "pg",
						Image:  "postgres:16",
						State:  "exited",
						Mounts: []docker.Mount{{Type: "volume", Name: "pgdata", Destination: "/var/lib/postgresql/data", RW: false}},
					},
					ExitCode: 1,
				},
				Logs: []string{"2026-10-22T09:00:00Z ERROR initdb: could not access directory /var/lib/postgresql/data: Permission denied"},
			},
			wantCat:   CatStoragePermission,
			wantLabel: "VolumePermissionError",
		},
		{
			name: "db connection refused",
			in: summarizer.ContainerInput{
				Detail: docker.ContainerDetail{
					Container: docker.Container{Name: "api", Image: "api:1", State: "running"},
					Running:   true,
				},
				Logs: []string{"2026-10-22T09:00:00Z ERROR dial tcp 172.18.0.3:5432: connect: connection refused"},
			},
			wantCat:   CatDependencyTimeout,
			wantLabel: "DBConnectionRefused",
		},
		{
			name: "network timeout",
			in: summarizer.ContainerInput{
				Detail: docker.ContainerDetail{
					Container: docker.Container{Name: "scraper", Image: "scraper:1", State: "running"},
					Running:   true,
				},
				Logs: []string{"2026-10-22T09:00:00Z ERROR Get https://example.com: dial tcp: lookup example.com: i/o timeout"},
			},
			wantCat:   CatNetworkUnreach,
			wantLabel: "NetworkTimeout",
		},
		{
			name: "cpu throttling",
			in: summarizer.ContainerInput{
				Detail: docker.ContainerDetail{
					Container: docker.Container{Name: "encoder", Image: "ffmpeg:1", State: "running"},
					Running:   true,
				},
				Stats: docker.Stats{CPUPercent: 95.2, Throttled: 42},
			},
			wantCat:   CatCPUThrottling,
			wantLabel: "CPUThrottling",
		},
		{
			name: "crash loop on config panic",
			in: summarizer.ContainerInput{
				Detail: docker.ContainerDetail{
					Container:    docker.Container{Name: "bad-cfg", Image: "svc:1", State: "restarting"},
					Restarting:   true,
					RestartCount: 5,
					ExitCode:     2,
				},
				Logs: []string{"panic: missing ENV variable JWT_SECRET"},
			},
			wantCat:   CatConfigPanic,
			wantLabel: "CrashLoop",
		},
		{
			name: "healthy container",
			in: summarizer.ContainerInput{
				Detail: docker.ContainerDetail{
					Container: docker.Container{Name: "nginx", Image: "nginx:alpine", State: "running", Networks: []docker.NetAttachment{{Network: "bridge", IP: "172.17.0.2"}}},
					Running:   true,
				},
				Stats: docker.Stats{CPUPercent: 1.2, MemPercent: 8.5},
				Logs:  []string{"2026-10-22T09:00:00Z INFO listening on :80"},
			},
			wantCat:   CatHealthy,
			wantLabel: "Healthy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify(tt.in)
			if got.Category != tt.wantCat || got.FaultLabel != tt.wantLabel {
				t.Errorf("Classify(%s) = (%s, %s), want (%s, %s)",
					tt.name, got.Category, got.FaultLabel, tt.wantCat, tt.wantLabel)
			}
			if len(got.RootCauseTree) == 0 || len(got.Runbook) == 0 {
				t.Errorf("Classify(%s) missing tree or runbook", tt.name)
			}
		})
	}
}
