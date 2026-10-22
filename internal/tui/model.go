package tui

import (
	"os"
	"runtime"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
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

// Model is the root Bubble Tea model of OkShield.
type Model struct {
	styles Styles
	width  int
	height int

	hostname string
	memAlloc uint64
	docker   ConnState
	otel     ConnState

	view   View
	tab    Tab
	focus  Focus
	cursor int

	input    textinput.Model
	status   string
	showHelp bool
}

// New builds the root model.
func New() Model {
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
		hostname: host,
		input:    ti,
		status:   "listo",
	}
}

// Init starts background ticks.
func (m Model) Init() tea.Cmd {
	return tea.Batch(sampleMem(), textinput.Blink)
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
