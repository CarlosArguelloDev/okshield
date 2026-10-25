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
		return m, sampleMem()

	case refreshTickMsg:
		return m, tea.Batch(fetchResources(m.ctx, m.api), m.detailCmd())

	case metricsTickMsg:
		return m, tea.Batch(scheduleMetrics(), m.metricsCmd())

	case metricsSampleMsg:
		if msg.err == nil {
			m.recordMetrics(msg.id, msg.stats, msg.at)
			if c, ok := m.selectedContainer(); (ok && c.ID == msg.id) || !ok {
				m.stats = msg.stats
			}
		}
		return m, nil

	case resourcesMsg:
		return m.applyResources(msg)

	case detailMsg:
		if msg.err == nil && msg.detail.Running {
			m.recordMetrics(msg.id, msg.stats, time.Now())
		}
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

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if m.showHelp {
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
	case "i":
		return m.dispatch(Command{Kind: CmdLang, Name: "lang"})
	case "m", " ":
		if m.tab == TabMetrics {
			m.metricKind = (m.metricKind + 1) % metricKindCount
			return m, nil
		}
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
			if m.tab == TabMetrics {
				m.metricKind = (m.metricKind + metricKindCount - 1) % metricKindCount
			} else if m.mainScroll > 0 {
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
				return m, tea.Batch(m.detailCmd(), m.metricsCmd())
			}
		}
	case "down", "j":
		switch m.focus {
		case FocusMain:
			if m.tab == TabMetrics {
				m.metricKind = (m.metricKind + 1) % metricKindCount
			} else {
				botH := bottomH
				if m.height < 20 {
					botH = max(5, m.height/3)
				}
				bodyRows := max(1, max(6, m.height-botH)-4)
				if maxOff := max(0, countLines(m.inspectorBody(bodyRows))-bodyRows); m.mainScroll < maxOff {
					m.mainScroll++
				}
			}
		case FocusProcesses:
			if m.procScroll < max(0, len(m.containers)+2) {
				m.procScroll++
			}
		case FocusMetadata:
			if m.metaScroll < 5 {
				m.metaScroll++
			}
		default:
			if m.cursor < m.itemCount()-1 {
				m.cursor++
				m.mainScroll = 0
				m.metaScroll = 0
				return m, tea.Batch(m.detailCmd(), m.metricsCmd())
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
		cmd = tea.Batch(m.detailCmd(), m.metricsCmd(), runPulse(m.ctx, m.api, m.containers))
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
	case CmdLang:
		if len(c.Args) > 0 {
			if l, ok := ParseLang(c.Args[0]); ok {
				m.lang = l
			}
		} else {
			if m.lang == LangES {
				m.lang = LangEN
			} else {
				m.lang = LangES
			}
		}
		m.input.Placeholder = m.tr().HelpPlaceholder
		m.status = "lang: " + m.lang.Code()
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

func countLines(s string) int {
	if s == "" {
		return 0
	}
	n := 1
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			n++
		}
	}
	return n
}
