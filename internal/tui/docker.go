package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/CarlosArguelloDev/okshield/internal/diagnose"
	"github.com/CarlosArguelloDev/okshield/internal/docker"
	"github.com/CarlosArguelloDev/okshield/internal/summarizer"
	"github.com/CarlosArguelloDev/okshield/internal/telemetry"
)

// DockerAPI is the subset of the Docker client used by the TUI. It allows
// tests to inject fakes.
type DockerAPI interface {
	ListContainers(ctx context.Context) ([]docker.Container, error)
	ListNetworks(ctx context.Context) ([]docker.Network, error)
	ListVolumes(ctx context.Context) ([]docker.Volume, error)
	InspectContainer(ctx context.Context, idOrName string) (docker.ContainerDetail, error)
}

// TelemetryAPI optionally exposes ContainerStats and ContainerLogs when the
// underlying client supports them.
type TelemetryAPI interface {
	ContainerStats(ctx context.Context, idOrName string) (docker.Stats, error)
	ContainerLogs(ctx context.Context, idOrName string, tail int) ([]string, error)
}

const (
	refreshInterval = 3 * time.Second
	metricsInterval = 1 * time.Second
	requestTimeout  = 3 * time.Second
)

// resourcesMsg carries a full infrastructure snapshot.
type resourcesMsg struct {
	containers []docker.Container
	networks   []docker.Network
	volumes    []docker.Volume
	err        error
}

// detailMsg carries the inspect, stats and logs result for the selected container.
type detailMsg struct {
	id       string
	detail   docker.ContainerDetail
	stats    docker.Stats
	logs     []string
	clusters []summarizer.LogCluster
	err      error
}

// pulseMsg carries the deterministic /whatsup report across all containers.
type pulseMsg struct {
	report summarizer.Report
	err    error
}

// diagnoseMsg carries the decision-model diagnosis for a container.
type diagnoseMsg struct {
	diag diagnose.Diagnosis
	err  error
}

// pruneMsg reports how many warm-store telemetry rows were pruned.
type pruneMsg struct {
	removed int
}

// refreshTickMsg triggers the next periodic snapshot.
type refreshTickMsg struct{}

// metricsTickMsg triggers the asynchronous 1.5s live metrics sample.
type metricsTickMsg struct{}

// metricsSampleMsg carries a live stats sample for a container.
type metricsSampleMsg struct {
	id    string
	stats docker.Stats
	at    time.Time
	err   error
}

// fetchResources lists containers, networks and volumes off the UI goroutine.
func fetchResources(parent context.Context, api DockerAPI) tea.Cmd {
	if api == nil {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(parent, requestTimeout)
		defer cancel()
		var m resourcesMsg
		if m.containers, m.err = api.ListContainers(ctx); m.err != nil {
			return m
		}
		if m.networks, m.err = api.ListNetworks(ctx); m.err != nil {
			return m
		}
		m.volumes, m.err = api.ListVolumes(ctx)
		return m
	}
}

// fetchDetail inspects a single container and, if supported, fetches its live
// stats and tail logs off the UI goroutine.
func fetchDetail(parent context.Context, api DockerAPI, store *telemetry.Store, id string) tea.Cmd {
	if api == nil || id == "" {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(parent, requestTimeout)
		defer cancel()
		d, err := api.InspectContainer(ctx, id)
		msg := detailMsg{id: id, detail: d, err: err}
		if err != nil {
			return msg
		}
		if tel, ok := api.(TelemetryAPI); ok {
			if d.Running {
				msg.stats, _ = tel.ContainerStats(ctx, id)
			}
			if lines, lerr := tel.ContainerLogs(ctx, id, docker.MaxLogLines); lerr == nil {
				msg.logs = lines
				msg.clusters = summarizer.ClusterLogs(lines)
				if store != nil && len(lines) > 0 {
					last := lines[len(lines)-1]
					sev, _ := summarizer.NormalizeLine(last)
					store.Append(telemetry.LogRecord{
						Timestamp: time.Now(),
						Source:    d.Name,
						Severity:  sev,
						Body:      last,
					})
				}
			}
		}
		return msg
	}
}

// runPulse executes the deterministic /whatsup engine across all containers.
func runPulse(parent context.Context, api DockerAPI, containers []docker.Container) tea.Cmd {
	if api == nil {
		return nil
	}
	list := append([]docker.Container(nil), containers...)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(parent, 6*time.Second)
		defer cancel()
		if len(list) == 0 {
			var err error
			list, err = api.ListContainers(ctx)
			if err != nil {
				return pulseMsg{err: err}
			}
		}
		inputs := make([]summarizer.ContainerInput, 0, len(list))
		tel, hasTel := api.(TelemetryAPI)
		for _, c := range list {
			d, err := api.InspectContainer(ctx, c.ID)
			if err != nil {
				d = docker.ContainerDetail{Container: c, ExitCode: c.ExitCode(), Running: c.State == "running"}
			}
			var st docker.Stats
			var logs []string
			if hasTel {
				if d.Running {
					st, _ = tel.ContainerStats(ctx, c.ID)
				}
				logs, _ = tel.ContainerLogs(ctx, c.ID, 200)
			}
			inputs = append(inputs, summarizer.ContainerInput{
				Detail: d,
				Stats:  st,
				Logs:   logs,
			})
		}
		return pulseMsg{report: summarizer.Analyze(inputs, 5)}
	}
}

// runDiagnose executes the decision-model classifier for a target container.
func runDiagnose(parent context.Context, api DockerAPI, target string) tea.Cmd {
	if api == nil || target == "" {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(parent, requestTimeout)
		defer cancel()
		d, err := api.InspectContainer(ctx, target)
		if err != nil {
			return diagnoseMsg{err: err}
		}
		var st docker.Stats
		var logs []string
		if tel, ok := api.(TelemetryAPI); ok {
			if d.Running {
				st, _ = tel.ContainerStats(ctx, target)
			}
			logs, _ = tel.ContainerLogs(ctx, target, docker.MaxLogLines)
		}
		diag := diagnose.Classify(summarizer.ContainerInput{
			Detail: d,
			Stats:  st,
			Logs:   logs,
		})
		return diagnoseMsg{diag: diag}
	}
}

func scheduleRefresh() tea.Cmd {
	return tea.Tick(refreshInterval, func(time.Time) tea.Msg { return refreshTickMsg{} })
}

func scheduleMetrics() tea.Cmd {
	return tea.Tick(metricsInterval, func(time.Time) tea.Msg { return metricsTickMsg{} })
}

// fetchMetrics collects a non-blocking live stats sample for a container.
func fetchMetrics(parent context.Context, api DockerAPI, id string) tea.Cmd {
	tel, ok := api.(TelemetryAPI)
	if !ok || id == "" {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(parent, requestTimeout)
		defer cancel()
		st, err := tel.ContainerStats(ctx, id)
		return metricsSampleMsg{id: id, stats: st, at: time.Now(), err: err}
	}
}

// containerHealth maps Docker state to the status glyph classification.
func containerHealth(c docker.Container) Health {
	switch c.State {
	case "running":
		return HealthOK
	case "restarting", "paused", "created":
		return HealthDegraded
	default: // exited, dead, removing
		return HealthError
	}
}
