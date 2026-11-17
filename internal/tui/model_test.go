package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/CarlosArguelloDev/okshield/internal/docker"
	"github.com/CarlosArguelloDev/okshield/internal/summarizer"
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
	case "left":
		msg = tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		msg = tea.KeyMsg{Type: tea.KeyRight}
	default:
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
	}
	out, _ := m.Update(msg)
	return out.(Model)
}

func TestKeybindingsSwitchViewsAndTabs(t *testing.T) {
	tests := []struct {
		keys      []string
		wantView  View
		wantTab   Tab
		wantFocus Focus
	}{
		{[]string{"2"}, ViewNetworks, TabSummary, FocusSidebar},
		{[]string{"3"}, ViewVolumes, TabSummary, FocusSidebar},
		{[]string{"4"}, ViewImages, TabSummary, FocusSidebar},
		{[]string{"3", "1"}, ViewContainers, TabSummary, FocusSidebar},
		{[]string{"right"}, ViewContainers, TabMetrics, FocusSidebar},
		{[]string{"right", "right"}, ViewContainers, TabDetails, FocusSidebar},
		{[]string{"right", "right", "right"}, ViewContainers, TabLogs, FocusSidebar},
		{[]string{"right", "left"}, ViewContainers, TabSummary, FocusSidebar},
		{[]string{"tab"}, ViewContainers, TabSummary, FocusMain},
		{[]string{"tab", "tab"}, ViewContainers, TabSummary, FocusProcesses},
		{[]string{"tab", "tab", "tab"}, ViewContainers, TabSummary, FocusMetadata},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.keys, ","), func(t *testing.T) {
			m := sized(t)
			for _, k := range tt.keys {
				m = key(m, k)
			}
			if m.view != tt.wantView || m.tab != tt.wantTab || m.focus != tt.wantFocus {
				t.Errorf("view=%v tab=%v focus=%v, want view=%v tab=%v focus=%v",
					m.view, m.tab, m.focus, tt.wantView, tt.wantTab, tt.wantFocus)
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
	if !strings.Contains(m.View(), "Available Commands") || !strings.Contains(m.View(), "Description & Details") {
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
	images     []docker.Image
	detail     docker.ContainerDetail
	actions    *[]string
}

func (f fakeDocker) record(op string) {
	if f.actions != nil {
		*f.actions = append(*f.actions, op)
	}
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
func (f fakeDocker) ListImages(context.Context) ([]docker.Image, error) {
	return f.images, nil
}
func (f fakeDocker) InspectContainer(context.Context, string) (docker.ContainerDetail, error) {
	return f.detail, nil
}
func (f fakeDocker) StartContainer(_ context.Context, id string) error {
	f.record("start:" + id)
	return nil
}
func (f fakeDocker) StopContainer(_ context.Context, id string) error {
	f.record("stop:" + id)
	return nil
}
func (f fakeDocker) RestartContainer(_ context.Context, id string) error {
	f.record("restart:" + id)
	return nil
}
func (f fakeDocker) PauseContainer(_ context.Context, id string) error {
	f.record("pause:" + id)
	return nil
}
func (f fakeDocker) UnpauseContainer(_ context.Context, id string) error {
	f.record("unpause:" + id)
	return nil
}
func (f fakeDocker) RemoveContainer(_ context.Context, id string, force bool) error {
	f.record("rm:" + id)
	return nil
}
func (f fakeDocker) DisconnectNetwork(_ context.Context, netID, cID string) error {
	f.record("disconnect:" + netID + ":" + cID)
	return nil
}
func (f fakeDocker) PruneNetworks(context.Context) error {
	f.record("prune-net")
	return nil
}
func (f fakeDocker) RemoveVolume(_ context.Context, name string) error {
	f.record("rm-vol:" + name)
	return nil
}
func (f fakeDocker) PullImage(_ context.Context, ref string) error {
	f.record("pull:" + ref)
	return nil
}
func (f fakeDocker) RemoveImage(_ context.Context, id string) error {
	f.record("rm-img:" + id)
	return nil
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
		images: []docker.Image{
			{ID: "sha256:abcdef1234567890", RepoTags: []string{"api:v1"}, Size: 64 << 20},
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
	if !strings.Contains(model.View(), "Deterministic Pulse") || !strings.Contains(model.View(), "General") {
		t.Errorf("summary view missing pulse header or General option")
	}

	// Move cursor down to row 1 ("api") and fetch its detail.
	model = key(model, "j")
	det := fetchDetail(context.Background(), fd, "11111111111122222222")()
	m, _ = model.Update(det)
	model = m.(Model)

	model = key(model, "right") // switch to TabMetrics using right arrow
	m, _ = model.Update(metricsSampleMsg{
		id: "11111111111122222222",
		stats: docker.Stats{
			CPUPercent: 42.5,
			MemUsage:   32 << 20,
			MemLimit:   128 << 20,
			MemPercent: 25.0,
			NetRxBytes: 4096,
			NetTxBytes: 2048,
		},
	})
	model = m.(Model)
	metricsView := model.View()
	for _, want := range []string{"CPU Usage", "Memory Usage", "Net RX/TX", "Disk I/O", "128.0MB"} {
		if !strings.Contains(metricsView, want) {
			t.Errorf("container metrics view missing %q", want)
		}
	}
	model = key(model, "m") // switch active chart metric to Memory %
	if model.metricKind != MetricMem {
		t.Errorf("metricKind = %v, want MetricMem", model.metricKind)
	}

	model = key(model, "right") // switch to TabDetails
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

	model = key(model, "4") // switch to Images
	view = model.View()
	if !strings.Contains(view, "api:v1") || !strings.Contains(view, "64.0MB") {
		t.Errorf("image details view missing tag or size")
	}
}

func TestMouseInteraction(t *testing.T) {
	fd := fakeDocker{
		containers: []docker.Container{
			{ID: "11111111111122222222", Name: "api", Image: "api:v1", State: "running", Status: "Up 10 minutes"},
			{ID: "33333333333344444444", Name: "worker", Image: "worker:v1", State: "exited", Status: "Exited (1) 1 minute ago"},
		},
	}

	m, _ := New(Options{Docker: fd}).Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	model := m.(Model)
	res := fetchResources(context.Background(), fd)()
	m, _ = model.Update(res)
	model = m.(Model)

	waitZone := func(id string) *zone.ZoneInfo {
		t.Helper()
		_ = model.View()
		for deadline := time.Now().Add(300 * time.Millisecond); time.Now().Before(deadline); {
			if z := zone.Get(id); !z.IsZero() {
				return z
			}
			time.Sleep(2 * time.Millisecond)
		}
		t.Fatalf("zone %q was not registered", id)
		return nil
	}

	clickZone := func(id string) Model {
		z := waitZone(id)
		out, _ := model.Update(tea.MouseMsg{
			X:      z.StartX,
			Y:      z.StartY,
			Action: tea.MouseActionPress,
			Button: tea.MouseButtonLeft,
		})
		return out.(Model)
	}

	// 1. Click Metrics tab (tab-1).
	model = clickZone("tab-1")
	if model.tab != TabMetrics {
		t.Errorf("after clicking tab-1: tab = %v, want TabMetrics", model.tab)
	}

	// 2. Click a metric selector pill (metric-2 -> Net RX/TX).
	model = clickZone("metric-2")
	if model.metricKind != MetricNet {
		t.Errorf("after clicking metric-2: metricKind = %v, want MetricNet", model.metricKind)
	}

	// 3. Click container row ("api").
	model = clickZone("container-11111111111122222222")
	if model.cursor != 1 {
		t.Errorf("after clicking container row: cursor = %d, want 1", model.cursor)
	}

	// 4. Mouse wheel down on Browser pane selects next item ("worker").
	bz := waitZone(zonePaneBrowser)
	out, _ := model.Update(tea.MouseMsg{
		X:      bz.StartX + 1,
		Y:      bz.StartY + 1,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonWheelDown,
	})
	model = out.(Model)
	if model.cursor != 2 {
		t.Errorf("after wheel down in browser: cursor = %d, want 2", model.cursor)
	}

	// 5. Click action-whatsup switches back to TabSummary.
	model = clickZone("action-whatsup")
	if model.tab != TabSummary {
		t.Errorf("after clicking action-whatsup: tab = %v, want TabSummary", model.tab)
	}
}

func TestCopyToClipboardZones(t *testing.T) {
	if got := formatOSC52("hello"); got != "\x1b]52;c;aGVsbG8=\x07" {
		t.Fatalf("formatOSC52 = %q, want %q", got, "\x1b]52;c;aGVsbG8=\x07")
	}

	var ops []string
	fd := fakeDocker{
		actions: &ops,
		containers: []docker.Container{
			{
				ID:     "11111111111122222222",
				Name:   "api",
				Image:  "api:v1",
				State:  "running",
				Status: "Up 10 minutes",
				Networks: []docker.NetAttachment{
					{Network: "bridge", IP: "172.17.0.2"},
				},
				Mounts: []docker.Mount{
					{Type: "volume", Name: "apidata", Destination: "/data", RW: true},
				},
			},
		},
	}

	m, _ := New(Options{Docker: fd}).Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	model := m.(Model)
	res := fetchResources(context.Background(), fd)()
	m, _ = model.Update(res)
	model = m.(Model)
	model = key(model, "j") // select "api"
	model.logs = []string{"2026-11-16T00:00:00Z INFO ready", "2026-11-16T00:00:01Z ERROR failed"}
	model.clusters = []summarizer.LogCluster{
		{Severity: summarizer.SevError, Template: "failed", Count: 1, Sample: model.logs[1]},
	}

	waitZone := func(id string) *zone.ZoneInfo {
		t.Helper()
		_ = model.View()
		for deadline := time.Now().Add(300 * time.Millisecond); time.Now().Before(deadline); {
			if z := zone.Get(id); !z.IsZero() {
				return z
			}
			time.Sleep(2 * time.Millisecond)
		}
		t.Fatalf("zone %q was not registered", id)
		return nil
	}

	clickZone := func(id string) tea.Cmd {
		z := waitZone(id)
		out, cmd := model.Update(tea.MouseMsg{
			X:      z.StartX,
			Y:      z.StartY,
			Action: tea.MouseActionPress,
			Button: tea.MouseButtonLeft,
		})
		model = out.(Model)
		return cmd
	}

	clickAndRunCmd := func(id string) tea.Msg {
		cmd := clickZone(id)
		if cmd == nil {
			t.Fatalf("clicking %q returned nil cmd", id)
		}
		return cmd()
	}

	// 1. Details tab copy icons ("copy-meta-d-name" & "copy-meta-d-image").
	model = key(model, "right")
	model = key(model, "right") // TabDetails
	msg := clickAndRunCmd("copy-meta-d-name")
	copied, ok := msg.(clipboardCopiedMsg)
	if !ok || copied.label != "api" {
		t.Fatalf("unexpected msg = %#v", msg)
	}
	out, _ := model.Update(copied)
	model = out.(Model)
	if !strings.Contains(model.toast, "✓ Copied: api") {
		t.Errorf("toast = %q, want '✓ Copied: api'", model.toast)
	}
	if !strings.Contains(model.View(), "✓ Copied: api") {
		t.Errorf("View() footer missing toast")
	}
	msg = clickAndRunCmd("copy-meta-d-image")
	if copied, ok := msg.(clipboardCopiedMsg); !ok || copied.label != "api:v1" {
		t.Errorf("unexpected details copy msg = %#v", msg)
	}

	// 2. Logs tab action buttons ("copy-logs-dedup" & "copy-logs-raw").
	model = key(model, "right") // TabLogs
	msg = clickAndRunCmd("copy-logs-dedup")
	if copied, ok := msg.(clipboardCopiedMsg); !ok || copied.btnID != "copy-logs-dedup" {
		t.Errorf("unexpected dedup msg = %#v", msg)
	}
	msg = clickAndRunCmd("copy-logs-raw")
	if copied, ok := msg.(clipboardCopiedMsg); !ok || copied.btnID != "copy-logs-raw" {
		t.Errorf("unexpected raw msg = %#v", msg)
	}

	// 3. Toast clear message removes toast and flashBtn.
	out, _ = model.Update(clipboardCopiedMsg{label: "test", btnID: "copy-logs-raw"})
	model = out.(Model)
	out, _ = model.Update(clipboardClearMsg{seq: model.toastSeq})
	model = out.(Model)
	if model.toast != "" || model.flashBtn != "" {
		t.Errorf("expected toast & flashBtn cleared, got toast=%q flashBtn=%q", model.toast, model.flashBtn)
	}

	// 4. Actions panel: non-destructive Restart on running container.
	msg = clickAndRunCmd("act-restart")
	actMsg, ok := msg.(dockerActionMsg)
	if !ok || actMsg.err != nil || actMsg.label != "Restarted api" {
		t.Fatalf("unexpected restart msg = %#v", msg)
	}

	// 5. Actions panel: destructive Rm -> Cancel -> Rm -> Confirm.
	if cmd := clickZone("act-rm"); cmd != nil {
		t.Fatalf("clicking destructive act-rm should wait for confirmation, got cmd")
	}
	if model.pendingConfirm != "act-rm" || !strings.Contains(model.View(), "Confirm Rm api?") {
		t.Fatalf("expected confirmation prompt, got pendingConfirm=%q", model.pendingConfirm)
	}
	_ = clickZone("act-cancel")
	if model.pendingConfirm != "" {
		t.Errorf("expected act-cancel to clear pendingConfirm")
	}
	_ = clickZone("act-rm")
	msg = clickAndRunCmd("act-confirm")
	if actMsg, ok := msg.(dockerActionMsg); !ok || actMsg.err != nil || actMsg.label != "Removed api" {
		t.Fatalf("unexpected confirm rm msg = %#v", msg)
	}
	if len(ops) != 2 || ops[0] != "restart:11111111111122222222" || ops[1] != "rm:11111111111122222222" {
		t.Errorf("recorded ops = %v", ops)
	}
}
