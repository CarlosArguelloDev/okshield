package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// Update handles messages. It never blocks: any I/O is delegated to tea.Cmd.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.input.Width = max(10, msg.Width-8)
		return m, nil

	case memTickMsg:
		m.memAlloc = msg.alloc
		return m, sampleMem()

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if m.showHelp {
			// Any key closes the help overlay.
			m.showHelp = false
			return m, nil
		}
		if m.focus == FocusCommand {
			return m.updateCommandBar(msg)
		}
		return m.updateNavigation(msg)
	}
	return m, nil
}

func (m Model) updateCommandBar(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.input.Reset()
		m.input.Blur()
		m.focus = FocusSidebar
		return m, nil
	case "enter":
		cmd := ParseCommand(m.input.Value())
		m.input.Reset()
		m.input.Blur()
		m.focus = FocusSidebar
		return m.dispatch(cmd)
	case "tab":
		if s := Suggest(m.input.Value()); len(s) == 1 {
			m.input.SetValue(firstField(s[0]) + " ")
			m.input.CursorEnd()
		}
		return m, nil
	}
	var c tea.Cmd
	m.input, c = m.input.Update(msg)
	return m, c
}

func (m Model) updateNavigation(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "/", ":":
		m.focus = FocusCommand
		m.input.SetValue("/")
		m.input.CursorEnd()
		return m, m.input.Focus()
	case "?":
		return m.dispatch(Command{Kind: CmdWhatsup, Name: "whatsup"})
	case "d":
		return m.dispatch(Command{Kind: CmdDiagnose, Name: "diagnose"})
	case "1":
		return m.dispatch(Command{Kind: CmdContainers})
	case "2":
		return m.dispatch(Command{Kind: CmdNetworks})
	case "3":
		return m.dispatch(Command{Kind: CmdVolumes})
	case "tab":
		m.tab = (m.tab + 1) % tabCount
	case "shift+tab":
		m.tab = (m.tab + tabCount - 1) % tabCount
	case "left", "h":
		m.focus = FocusSidebar
	case "right", "l", "enter":
		m.focus = FocusMain
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		m.cursor++
	}
	return m, nil
}

// dispatch executes a parsed command. Engines are wired in later commits;
// for now commands switch views/tabs and report status.
func (m Model) dispatch(c Command) (tea.Model, tea.Cmd) {
	switch c.Kind {
	case CmdWhatsup:
		m.tab = TabSummary
		m.status = "/whatsup: motor determinista pendiente de conectar"
	case CmdDiagnose:
		m.tab = TabDiagnose
		target := "selección actual"
		if len(c.Args) > 0 {
			target = c.Args[0]
		}
		m.status = fmt.Sprintf("/diagnose %s: decision model pendiente de conectar", target)
	case CmdContainers, CmdNetworks, CmdVolumes:
		m.view = View(c.Kind - CmdContainers)
		m.cursor = 0
		m.status = "vista: " + m.view.String()
	case CmdOtelStatus:
		m.status = "/otel status: receptor OTLP aún no iniciado"
	case CmdPruneTelemetry:
		m.status = "/prune-telemetry: DuckDB aún no inicializado"
	case CmdHelp:
		m.showHelp = true
	case CmdQuit:
		return m, tea.Quit
	default:
		if c.Raw == "" || c.Raw == "/" {
			return m, nil
		}
		m.status = fmt.Sprintf("comando desconocido: %q (usa /help)", c.Raw)
	}
	return m, nil
}

func firstField(s string) string {
	for i, r := range s {
		if r == ' ' {
			return s[:i]
		}
	}
	return s
}
