package tui

import (
	"fmt"
	"time"

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
		if m.receiver != nil {
			if m.receiver.Stats().Listening {
				m.otel = ConnUp
			} else {
				m.otel = ConnDown
			}
		}
		return m, sampleMem()

	case refreshTickMsg:
		return m, tea.Batch(fetchResources(m.ctx, m.api), m.detailCmd())

	case resourcesMsg:
		return m.applyResources(msg)

	case detailMsg:
		if c, ok := m.selectedContainer(); ok && c.ID == msg.id {
			m.detail, m.stats, m.logs, m.clusters, m.detailErr = msg.detail, msg.stats, msg.logs, msg.clusters, msg.err
		}
		return m, nil

	case pulseMsg:
		if msg.err != nil {
			m.status = "/whatsup error: " + msg.err.Error()
			return m, nil
		}
		m.pulse = msg.report
		m.hasPulse = true
		m.status = fmt.Sprintf("/whatsup listo · %d ok · %d warn · %d err",
			msg.report.Running, msg.report.Degraded, msg.report.Failed)
		return m, nil

	case diagnoseMsg:
		if msg.err != nil {
			m.status = "/diagnose error: " + msg.err.Error()
			return m, nil
		}
		m.diag = msg.diag
		m.hasDiag = true
		m.status = fmt.Sprintf("/diagnose %s → %s [%s]",
			msg.diag.Container, msg.diag.Category, msg.diag.FaultLabel)
		return m, nil

	case pruneMsg:
		m.status = fmt.Sprintf("/prune-telemetry: %d registros antiguos eliminados", msg.removed)
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if m.showHelp || m.showOtel {
			m.showHelp = false
			m.showOtel = false
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
		m.focus = Focus((int(m.focus) + 1) % focusPaneCount)
	case "shift+tab":
		m.focus = Focus((int(m.focus) + focusPaneCount - 1) % focusPaneCount)
	case "left", "h":
		m.tab = (m.tab + tabCount - 1) % tabCount
		m.mainScroll = 0
	case "right", "l":
		m.tab = (m.tab + 1) % tabCount
		m.mainScroll = 0
	case "enter":
		m.focus = FocusMain
	case "up", "k":
		switch m.focus {
		case FocusMain:
			if m.mainScroll > 0 {
				m.mainScroll--
			}
		case FocusProcesses:
			if m.procScroll > 0 {
				m.procScroll--
			}
		case FocusMetadata:
			if m.metaScroll > 0 {
				m.metaScroll--
			}
		default:
			if m.cursor > 0 {
				m.cursor--
				m.mainScroll = 0
				m.metaScroll = 0
				return m, m.detailCmd()
			}
		}
	case "down", "j":
		switch m.focus {
		case FocusMain:
			m.mainScroll++
		case FocusProcesses:
			m.procScroll++
		case FocusMetadata:
			m.metaScroll++
		default:
			if m.cursor < m.itemCount()-1 {
				m.cursor++
				m.mainScroll = 0
				m.metaScroll = 0
				return m, m.detailCmd()
			}
		}
	}
	return m, nil
}

// applyResources stores a fresh snapshot and schedules the next refresh.
func (m Model) applyResources(msg resourcesMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.docker = ConnDown
		m.status = "docker: " + msg.err.Error()
		return m, scheduleRefresh()
	}
	prev, hadPrev := m.selectedContainer()
	first := m.docker != ConnUp
	m.docker = ConnUp
	m.containers, m.networks, m.volumes = msg.containers, msg.networks, msg.volumes

	if hadPrev {
		for i, c := range m.containers {
			if c.ID == prev.ID {
				m.cursor = i + 1
				break
			}
		}
	}
	m.clampCursor()

	var cmd tea.Cmd
	if first {
		m.status = fmt.Sprintf("docker conectado · %d contenedores", len(m.containers))
		cmd = tea.Batch(m.detailCmd(), runPulse(m.ctx, m.api, m.containers))
	}
	return m, tea.Batch(cmd, scheduleRefresh())
}

// dispatch executes a parsed command asynchronously.
func (m Model) dispatch(c Command) (tea.Model, tea.Cmd) {
	switch c.Kind {
	case CmdWhatsup:
		m.tab = TabSummary
		m.status = "ejecutando resumen determinista (/whatsup)…"
		return m, runPulse(m.ctx, m.api, m.containers)
	case CmdDiagnose:
		m.tab = TabDiagnose
		target := ""
		if len(c.Args) > 0 {
			target = c.Args[0]
		} else if sel, ok := m.selectedContainer(); ok {
			target = sel.ID
		} else if len(m.containers) > 0 {
			target = m.containers[0].ID
			for _, ct := range m.containers {
				if containerHealth(ct) != HealthOK {
					target = ct.ID
					break
				}
			}
		}
		if target == "" {
			m.status = "/diagnose: selecciona un contenedor o pasa su nombre"
			return m, nil
		}
		m.status = fmt.Sprintf("diagnosticando %s…", target)
		return m, runDiagnose(m.ctx, m.api, target)
	case CmdContainers, CmdNetworks, CmdVolumes:
		m.view = View(c.Kind - CmdContainers)
		m.cursor = 0
		m.status = "vista: " + m.view.String()
		return m, m.detailCmd()
	case CmdOtelStatus:
		m.showOtel = true
		m.status = "/otel status"
	case CmdPruneTelemetry:
		if m.store == nil {
			m.status = "/prune-telemetry: almacén no inicializado"
			return m, nil
		}
		st := m.store
		return m, func() tea.Msg {
			return pruneMsg{removed: st.Prune(time.Now())}
		}
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
