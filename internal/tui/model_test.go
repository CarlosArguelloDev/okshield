package tui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/CarlosArguelloDev/okshield/internal/docker"
)

func sized(t *testing.T) Model {
	t.Helper()
	m, _ := New(Options{}).Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	return m.(Model)
}

func key(m Model, k string) Model {
	var msg tea.KeyMsg
	switch k {
	case "enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	case "tab":
		msg = tea.KeyMsg{Type: tea.KeyTab}
	case "esc":
		msg = tea.KeyMsg{Type: tea.KeyEsc}
	default:
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
	}
	out, _ := m.Update(msg)
	return out.(Model)
}

func TestKeybindingsSwitchViewsAndTabs(t *testing.T) {
	tests := []struct {
		keys     []string
		wantView View
		wantTab  Tab
	}{
		{[]string{"2"}, ViewNetworks, TabSummary},
		{[]string{"3"}, ViewVolumes, TabSummary},
		{[]string{"3", "1"}, ViewContainers, TabSummary},
		{[]string{"d"}, ViewContainers, TabDiagnose},
		{[]string{"d", "?"}, ViewContainers, TabSummary},
		{[]string{"tab", "tab"}, ViewContainers, TabLogs},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.keys, ","), func(t *testing.T) {
			m := sized(t)
			for _, k := range tt.keys {
				m = key(m, k)
			}
			if m.view != tt.wantView || m.tab != tt.wantTab {
				t.Errorf("view=%v tab=%v, want view=%v tab=%v", m.view, m.tab, tt.wantView, tt.wantTab)
			}
		})
	}
}

func TestCommandBarDispatch(t *testing.T) {
	m := sized(t)
	m = key(m, "/") // focus command bar, prefilled with "/" and opens help modal
	if m.focus != FocusCommand {
		t.Fatalf("focus = %v, want FocusCommand", m.focus)
	}
	if !strings.Contains(m.View(), "Comandos Disponibles") || !strings.Contains(m.View(), "Descripción y Detalles") {
		t.Errorf("pressing / should open command help modal with available commands and descriptions")
	}
	m = key(m, "volumes")
	m = key(m, "enter")
	if m.view != ViewVolumes {
		t.Errorf("view = %v, want Volumes", m.view)
	}
	if m.focus == FocusCommand {
		t.Errorf("command bar should lose focus after enter")
	}

	m = key(m, "/")
	m = key(m, "help")
	m = key(m, "enter")
	if !m.showHelp {
		t.Errorf("/help should open overlay")
	}
	m = key(m, "x")
	if m.showHelp {
		t.Errorf("any key should close overlay")
	}
}

func TestViewFitsTerminal(t *testing.T) {
	m := sized(t)
	out := m.View()
	if h := lipgloss.Height(out); h > 30 {
		t.Errorf("view height %d exceeds terminal height 30", h)
	}
	for i, line := range strings.Split(out, "\n") {
		if w := lipgloss.Width(line); w > 120 {
			t.Errorf("line %d width %d exceeds terminal width 120", i, w)
		}
	}
}

func TestHumanBytes(t *testing.T) {
	tests := []struct {
		in   uint64
		want string
	}{
		{0, "0B"},
		{512, "512B"},
		{2048, "2.0KB"},
		{5 * 1024 * 1024, "5.0MB"},
	}
	for _, tt := range tests {
		if got := humanBytes(tt.in); got != tt.want {
			t.Errorf("humanBytes(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

type fakeDocker struct {
	containers []docker.Container
	networks   []docker.Network
	volumes    []docker.Volume
	detail     docker.ContainerDetail
}

func (f fakeDocker) ListContainers(context.Context) ([]docker.Container, error) {
	return f.containers, nil
}
func (f fakeDocker) ListNetworks(context.Context) ([]docker.Network, error) {
	return f.networks, nil
}
func (f fakeDocker) ListVolumes(context.Context) ([]docker.Volume, error) {
	return f.volumes, nil
}
func (f fakeDocker) InspectContainer(context.Context, string) (docker.ContainerDetail, error) {
	return f.detail, nil
}

func TestResourcesAndDetailsRender(t *testing.T) {
	fd := fakeDocker{
		containers: []docker.Container{
			{
				ID:     "11111111111122222222",
				Name:   "api",
				Image:  "api:v1",
				State:  "running",
				Status: "Up 10 minutes",
				Ports:  []docker.Port{{IP: "0.0.0.0", Private: 8080, Public: 8080, Proto: "tcp"}},
				Networks: []docker.NetAttachment{
					{Network: "bridge", IP: "172.17.0.2"},
				},
				Mounts: []docker.Mount{
					{Type: "volume", Name: "apidata", Destination: "/data", RW: true},
				},
			},
			{
				ID:     "33333333333344444444",
				Name:   "worker",
				Image:  "worker:v1",
				State:  "exited",
				Status: "Exited (137) 1 minute ago",
			},
		},
		networks: []docker.Network{
			{ID: "n1", Name: "bridge", Driver: "bridge", Scope: "local", Subnets: []docker.Subnet{{CIDR: "172.17.0.0/16", Gateway: "172.17.0.1"}}},
		},
		volumes: []docker.Volume{
			{Name: "apidata", Driver: "local", Mountpoint: "/var/lib/docker/volumes/apidata/_data", Scope: "local"},
		},
		detail: docker.ContainerDetail{
			Container:    docker.Container{ID: "11111111111122222222", Name: "api"},
			Running:      true,
			RestartCount: 2,
			MemoryLimit:  128 << 20,
			NanoCPUs:     1e9,
		},
	}

	m, _ := New(Options{Docker: fd}).Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	model := m.(Model)

	res := fetchResources(context.Background(), fd)()
	m, _ = model.Update(res)
	model = m.(Model)

	pmsg := runPulse(context.Background(), fd, fd.containers)()
	m, _ = model.Update(pmsg)
	model = m.(Model)
	// Row 0 is "General" (all containers): summary must show both "api" and "worker".
	if !strings.Contains(model.View(), "Pulso Determinista") || !strings.Contains(model.View(), "General") {
		t.Errorf("summary view missing pulse header or General option")
	}

	// Move cursor down to row 1 ("api") and fetch its detail.
	model = key(model, "j")
	det := fetchDetail(context.Background(), fd, nil, "11111111111122222222")()
	m, _ = model.Update(det)
	model = m.(Model)

	dmsg := runDiagnose(context.Background(), fd, "11111111111122222222")()
	m, _ = model.Update(dmsg)
	model = m.(Model)
	model = key(model, "d")
	if !strings.Contains(model.View(), "Diagnóstico:") {
		t.Errorf("diagnose view missing header")
	}
	model = key(model, "?") // back to TabSummary

	model = key(model, "tab") // switch to TabDetails
	view := model.View()
	for _, want := range []string{"api", "worker", "8080", "172.17.0.2", "apidata", "128.0MB"} {
		if !strings.Contains(view, want) {
			t.Errorf("container details view missing %q", want)
		}
	}

	model = key(model, "2") // switch to Networks
	view = model.View()
	if !strings.Contains(view, "172.17.0.0/16") {
		t.Errorf("network details view missing subnet")
	}

	model = key(model, "3") // switch to Volumes
	view = model.View()
	if !strings.Contains(view, "/var/lib/docker/volumes/apidata/_data") {
		t.Errorf("volume details view missing mountpoint")
	}
}
