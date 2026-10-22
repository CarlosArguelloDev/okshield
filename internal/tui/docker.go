package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/CarlosArguelloDev/okshield/internal/docker"
)

// DockerAPI is the subset of the Docker client used by the TUI. It allows
// tests to inject fakes.
type DockerAPI interface {
	ListContainers(ctx context.Context) ([]docker.Container, error)
	ListNetworks(ctx context.Context) ([]docker.Network, error)
	ListVolumes(ctx context.Context) ([]docker.Volume, error)
	InspectContainer(ctx context.Context, idOrName string) (docker.ContainerDetail, error)
}

const (
	refreshInterval = 3 * time.Second
	requestTimeout  = 3 * time.Second
)

// resourcesMsg carries a full infrastructure snapshot.
type resourcesMsg struct {
	containers []docker.Container
	networks   []docker.Network
	volumes    []docker.Volume
	err        error
}

// detailMsg carries the inspect result for the selected container.
type detailMsg struct {
	id     string
	detail docker.ContainerDetail
	err    error
}

// refreshTickMsg triggers the next periodic snapshot.
type refreshTickMsg struct{}

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

// fetchDetail inspects a single container off the UI goroutine.
func fetchDetail(parent context.Context, api DockerAPI, id string) tea.Cmd {
	if api == nil || id == "" {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(parent, requestTimeout)
		defer cancel()
		d, err := api.InspectContainer(ctx, id)
		return detailMsg{id: id, detail: d, err: err}
	}
}

func scheduleRefresh() tea.Cmd {
	return tea.Tick(refreshInterval, func(time.Time) tea.Msg { return refreshTickMsg{} })
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
