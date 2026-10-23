package tui

import (
	"context"
	"os"
	"runtime"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/CarlosArguelloDev/okshield/internal/diagnose"
	"github.com/CarlosArguelloDev/okshield/internal/docker"
	"github.com/CarlosArguelloDev/okshield/internal/summarizer"
	"github.com/CarlosArguelloDev/okshield/internal/telemetry"
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
	TabMetrics
	TabDetails
	TabLogs
	TabDiagnose
	tabCount
)

var tabNames = [...]string{"Resumen", "Métricas", "Detalles", "Logs", "Diagnóstico"}

func (t Tab) String() string { return tabNames[t] }

// Focus identifies which region receives keyboard input.
type Focus int

const (
	FocusSidebar Focus = iota
	FocusMain
	FocusProcesses
	FocusMetadata
	FocusCommand
)

const focusPaneCount = 4

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
	// Store is the telemetry ring-buffer + warm store.
	Store *telemetry.Store
	// Receiver is the embedded OTLP receiver.
	Receiver *telemetry.Receiver
	// Lang sets the initial display language (LangES by default).
	Lang Lang
}

// Model is the root Bubble Tea model of OkShield.
type Model struct {
	styles Styles
	width  int
	height int

	ctx      context.Context
	api      DockerAPI
	store    *telemetry.Store
	receiver *telemetry.Receiver

	hostname string
	memAlloc uint64
	docker   ConnState
	otel     ConnState

	view       View
	tab        Tab
	focus      Focus
	cursor     int
	lang       Lang
	metricKind MetricKind
	mainScroll int
	procScroll int
	metaScroll int

	containers []docker.Container
	networks   []docker.Network
	volumes    []docker.Volume
	detail     docker.ContainerDetail // inspect of the selected container
	stats      docker.Stats
	logs       []string
	clusters   []summarizer.LogCluster
	detailErr  error
	history    map[string]*MetricSeries
	cpuBar     progress.Model
	memBar     progress.Model

	pulse    summarizer.Report
	hasPulse bool
	diag     diagnose.Diagnosis
	hasDiag  bool

	input    textinput.Model
	status   string
	showHelp bool
	showOtel bool
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

	otelState := ConnUnknown
	if opts.Receiver != nil {
		if opts.Receiver.Stats().Listening {
			otelState = ConnUp
		} else {
			otelState = ConnDown
		}
	}

	cpuBar := progress.New(
		progress.WithSolidFill(string(ColorFocus)),
		progress.WithoutPercentage(),
		progress.WithWidth(14),
	)
	memBar := progress.New(
		progress.WithSolidFill(string(ColorAccent)),
		progress.WithoutPercentage(),
		progress.WithWidth(14),
	)

	return Model{
		styles:   s,
		ctx:      opts.Ctx,
		api:      opts.Docker,
		store:    opts.Store,
		receiver: opts.Receiver,
		hostname: host,
		otel:     otelState,
		lang:     opts.Lang,
		history:  make(map[string]*MetricSeries),
		cpuBar:   cpuBar,
		memBar:   memBar,
		input:    ti,
		status:   "conectando con docker…",
	}
}

// Init starts background ticks and the first infrastructure snapshot.
func (m Model) Init() tea.Cmd {
	return tea.Batch(sampleMem(), scheduleMetrics(), textinput.Blink, fetchResources(m.ctx, m.api))
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
// In ViewContainers, row 0 is the "General" (all containers) option when containers exist.
func (m Model) itemCount() int {
	switch m.view {
	case ViewNetworks:
		return len(m.networks)
	case ViewVolumes:
		return len(m.volumes)
	default:
		if len(m.containers) == 0 {
			return 0
		}
		return len(m.containers) + 1
	}
}

// selectedContainer returns the highlighted container, if any.
// Row 0 in ViewContainers represents "General" (all containers).
func (m Model) selectedContainer() (docker.Container, bool) {
	if m.view != ViewContainers || m.cursor <= 0 || m.cursor > len(m.containers) {
		return docker.Container{}, false
	}
	return m.containers[m.cursor-1], true
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

// detailCmd requests inspect, stats and logs data for the current selection.
func (m Model) detailCmd() tea.Cmd {
	c, ok := m.selectedContainer()
	if !ok {
		return nil
	}
	return fetchDetail(m.ctx, m.api, m.store, c.ID)
}

// metricsCmd polls live stats for the selected container (or the first running
// container when General is selected).
func (m Model) metricsCmd() tea.Cmd {
	if c, ok := m.selectedContainer(); ok {
		if c.State == "running" {
			return fetchMetrics(m.ctx, m.api, c.ID)
		}
		return nil
	}
	for _, c := range m.containers {
		if c.State == "running" {
			return fetchMetrics(m.ctx, m.api, c.ID)
		}
	}
	return nil
}

// recordMetrics stores a live CPU/RAM/Net sample in the bounded ring buffer.
func (m *Model) recordMetrics(id string, st docker.Stats, at time.Time) {
	if id == "" {
		return
	}
	if m.history == nil {
		m.history = make(map[string]*MetricSeries)
	}
	if len(m.history) > 64 {
		clear(m.history)
	}
	series, ok := m.history[id]
	if !ok {
		series = &MetricSeries{}
		m.history[id] = series
	}
	series.PushFull(st.CPUPercent, st.MemPercent, st.NetRxBytes, st.NetTxBytes, st.BlkReadBytes, st.BlkWriteBytes, at)
}
