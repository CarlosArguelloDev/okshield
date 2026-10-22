package tui

import (
	"context"
	"os"
	"runtime"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/CarlosArguelloDev/okshield/internal/docker"
)

// View is the infrastructure resource shown in the sidebar.
type View int

const (
	ViewContainers View = iota
	ViewNetworks
	ViewVolumes
)

var viewNames = [...]string{"Containers", "Networks", "Volumes"}

func (v View) String() string { return viewNames[v] }

// Tab is the active pane on the right side of the split view.
type Tab int

const (
	TabSummary Tab = iota
	TabDetails
	TabLogs
	TabDiagnose
	tabCount
)

var tabNames = [...]string{"Resumen", "Métricas/Detalles", "Logs", "Diagnóstico"}

func (t Tab) String() string { return tabNames[t] }

// Focus identifies which region receives keyboard input.
type Focus int

const (
	FocusSidebar Focus = iota
	FocusMain
	FocusCommand
)

// memTickInterval is how often the self-memory badge is refreshed.
const memTickInterval = time.Second

// memTickMsg carries a fresh sample of OkShield's own heap usage.
type memTickMsg struct{ alloc uint64 }

// ConnState is a coarse status for external backends shown in the header.
type ConnState int

const (
	ConnUnknown ConnState = iota
	ConnUp
	ConnDown
)

// Options configures the root model.
type Options struct {
	// Ctx bounds every background request; cancel it to stop all I/O.
	Ctx context.Context
	// Docker is the Engine API client (nil disables Docker features).
	Docker DockerAPI
}

// Model is the root Bubble Tea model of OkShield.
type Model struct {
	styles Styles
	width  int
	height int

	ctx context.Context
	api DockerAPI

	hostname string
	memAlloc uint64
	docker   ConnState
	otel     ConnState

	view   View
	tab    Tab
	focus  Focus
	cursor int

	containers []docker.Container
	networks   []docker.Network
	volumes    []docker.Volume
	detail     docker.ContainerDetail // inspect of the selected container
	detailErr  error

	input    textinput.Model
	status   string
	showHelp bool
}

// New builds the root model.
func New(opts Options) Model {
	if opts.Ctx == nil {
		opts.Ctx = context.Background()
	}
	ti := textinput.New()
	ti.Prompt = "> "
	ti.Placeholder = "escribe /help para ver comandos"
	ti.CharLimit = 128

	host, err := os.Hostname()
	if err != nil {
		host = "unknown"
	}

	s := DefaultStyles()
	ti.PromptStyle = s.Prompt
	ti.PlaceholderStyle = s.Muted

	return Model{
		styles:   s,
		ctx:      opts.Ctx,
		api:      opts.Docker,
		hostname: host,
		input:    ti,
		status:   "conectando con docker…",
	}
}

// Init starts background ticks and the first infrastructure snapshot.
func (m Model) Init() tea.Cmd {
	return tea.Batch(sampleMem(), textinput.Blink, fetchResources(m.ctx, m.api))
}

// sampleMem schedules the next self-memory sample. runtime.ReadMemStats is
// executed inside the Cmd goroutine, never in Update.
func sampleMem() tea.Cmd {
	return tea.Tick(memTickInterval, func(time.Time) tea.Msg {
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		return memTickMsg{alloc: ms.Alloc}
	})
}

// itemCount returns the number of rows in the active sidebar view.
func (m Model) itemCount() int {
	switch m.view {
	case ViewNetworks:
		return len(m.networks)
	case ViewVolumes:
		return len(m.volumes)
	default:
		return len(m.containers)
	}
}

// selectedContainer returns the highlighted container, if any.
func (m Model) selectedContainer() (docker.Container, bool) {
	if m.view != ViewContainers || m.cursor < 0 || m.cursor >= len(m.containers) {
		return docker.Container{}, false
	}
	return m.containers[m.cursor], true
}

// clampCursor keeps the cursor inside the active list.
func (m *Model) clampCursor() {
	if n := m.itemCount(); m.cursor >= n {
		m.cursor = n - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// detailCmd requests inspect data for the current selection.
func (m Model) detailCmd() tea.Cmd {
	c, ok := m.selectedContainer()
	if !ok {
		return nil
	}
	return fetchDetail(m.ctx, m.api, c.ID)
}
