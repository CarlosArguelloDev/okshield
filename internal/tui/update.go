package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"
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
		m.status = fmt.Sprintf("/whatsup ready · %d ok · %d warn · %d err",
			msg.report.Running, msg.report.Degraded, msg.report.Failed)
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

	case tea.MouseMsg:
		return m.updateMouse(msg)

	case clipboardCopiedMsg:
		m.toastSeq++
		seq := m.toastSeq
		m.toast = "✓ Copied: " + truncate(msg.label, 32)
		m.flashBtn = msg.btnID
		m.status = m.toast
		return m, tea.Tick(toastDuration, func(time.Time) tea.Msg {
			return clipboardClearMsg{seq: seq}
		})

	case clipboardClearMsg:
		if msg.seq == m.toastSeq {
			m.toast = ""
			m.flashBtn = ""
		}
		return m, nil

	case dockerActionMsg:
		m.toastSeq++
		seq := m.toastSeq
		m.flashBtn = msg.btnID
		if msg.err != nil {
			m.toast = IconClose + " " + truncate(msg.err.Error(), 32)
		} else {
			m.toast = IconCheck + " " + truncate(msg.label, 32)
		}
		m.status = m.toast
		return m, tea.Batch(
			fetchResources(m.ctx, m.api),
			m.detailCmd(),
			tea.Tick(toastDuration, func(time.Time) tea.Msg {
				return clipboardClearMsg{seq: seq}
			}),
		)
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
	case "4":
		return m.dispatch(Command{Kind: CmdImages})
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
				m.pendingConfirm = ""
				m.pendingLabel = ""
				return m, tea.Batch(m.detailCmd(), m.metricsCmd())
			}
		}
	case "down", "j":
		switch m.focus {
		case FocusMain:
			if m.tab == TabMetrics {
				m.metricKind = (m.metricKind + 1) % metricKindCount
			} else if maxOff := m.maxMainScroll(); m.mainScroll < maxOff {
				m.mainScroll++
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
				m.pendingConfirm = ""
				m.pendingLabel = ""
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
	m.containers, m.networks, m.volumes, m.images = msg.containers, msg.networks, msg.volumes, msg.images

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
		m.status = fmt.Sprintf("docker connected · %d containers", len(m.containers))
		cmd = tea.Batch(m.detailCmd(), m.metricsCmd(), runPulse(m.ctx, m.api, m.containers))
	}
	return m, tea.Batch(cmd, scheduleRefresh())
}

// dispatch executes a parsed command asynchronously.
func (m Model) dispatch(c Command) (tea.Model, tea.Cmd) {
	switch c.Kind {
	case CmdWhatsup:
		m.tab = TabSummary
		m.status = "running deterministic summary (/whatsup)…"
		return m, runPulse(m.ctx, m.api, m.containers)
	case CmdContainers, CmdNetworks, CmdVolumes, CmdImages:
		m.view = View(c.Kind - CmdContainers)
		m.cursor = 0
		m.pendingConfirm = ""
		m.pendingLabel = ""
		m.status = "view: " + m.view.String()
		return m, m.detailCmd()
	case CmdHelp:
		m.showHelp = true
	case CmdQuit:
		return m, tea.Quit
	default:
		if c.Raw == "" || c.Raw == "/" {
			return m, nil
		}
		m.status = fmt.Sprintf("unknown command: %q (use /help)", c.Raw)
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

func (m Model) maxMainScroll() int {
	botH := bottomH
	if m.height < 20 {
		botH = max(5, m.height/3)
	}
	bodyRows := max(1, max(6, m.height-botH)-4)
	_, _, inspectorW := m.topColumnWidths()
	body := m.inspectorBody(bodyRows)
	if m.tab != TabMetrics {
		body = wrapBody(body, max(1, inspectorW-4))
	}
	return max(0, countLines(body)-bodyRows)
}

func (m Model) updateMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown {
		return m.handleMouseWheel(msg)
	}
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}

	if m.showHelp || m.focus == FocusCommand {
		for _, c := range commandTable {
			if zone.Get("cmd-" + c.Names[0]).InBounds(msg) {
				m.showHelp = false
				if m.focus == FocusCommand {
					m.input.Reset()
					m.input.Blur()
					m.focus = FocusSidebar
				}
				return m.dispatch(Command{Kind: c.Kind, Name: c.Names[0]})
			}
		}
		if m.showHelp {
			m.showHelp = false
			return m, nil
		}
		m.input.Reset()
		m.input.Blur()
		m.focus = FocusSidebar
		return m, nil
	}

	for i := View(0); i <= ViewImages; i++ {
		if zone.Get(fmt.Sprintf("view-%d", i)).InBounds(msg) {
			return m.dispatch(Command{Kind: CmdContainers + CommandKind(i)})
		}
	}

	switch {
	case zone.Get("action-whatsup").InBounds(msg):
		return m.dispatch(Command{Kind: CmdWhatsup, Name: "whatsup"})
	case zone.Get("action-command").InBounds(msg):
		m.focus = FocusCommand
		m.input.SetValue("/")
		m.input.CursorEnd()
		return m, m.input.Focus()
	case zone.Get("action-quit").InBounds(msg):
		return m, tea.Quit
	}

	for t := Tab(0); t < tabCount; t++ {
		if zone.Get(fmt.Sprintf("tab-%d", t)).InBounds(msg) {
			m.tab = t
			m.mainScroll = 0
			m.focus = FocusMain
			return m, nil
		}
	}

	if m.tab == TabMetrics {
		for k := MetricKind(0); k < metricKindCount; k++ {
			if zone.Get(fmt.Sprintf("metric-%d", k)).InBounds(msg) {
				m.metricKind = k
				m.focus = FocusMain
				return m, nil
			}
		}
	}

	if m.view == ViewContainers {
		for i, c := range m.containers {
			if zone.Get(fmt.Sprintf("container-%s", c.ID)).InBounds(msg) {
				m.cursor = i + 1
				m.focus = FocusSidebar
				m.mainScroll = 0
				m.metaScroll = 0
				m.pendingConfirm = ""
				m.pendingLabel = ""
				return m, tea.Batch(m.detailCmd(), m.metricsCmd())
			}
		}
	}

	for i := 0; i < m.itemCount(); i++ {
		if zone.Get(fmt.Sprintf("row-%d", i)).InBounds(msg) {
			m.cursor = i
			m.focus = FocusSidebar
			m.mainScroll = 0
			m.metaScroll = 0
			m.pendingConfirm = ""
			m.pendingLabel = ""
			return m, tea.Batch(m.detailCmd(), m.metricsCmd())
		}
	}

	if m.pendingConfirm != "" {
		if zone.Get("act-confirm").InBounds(msg) {
			actID := m.pendingConfirm
			m.pendingConfirm = ""
			m.pendingLabel = ""
			return m.triggerAction(actID)
		}
		if zone.Get("act-cancel").InBounds(msg) {
			m.pendingConfirm = ""
			m.pendingLabel = ""
			return m, nil
		}
	} else {
		_, specs := m.contextActions()
		for _, sp := range specs {
			if zone.Get(sp.id).InBounds(msg) {
				m.focus = FocusMetadata
				if sp.destructive {
					m.pendingConfirm = sp.id
					m.pendingLabel = sp.confirmLabel
					return m, nil
				}
				return m.triggerAction(sp.id)
			}
		}
	}

	if m.tab == TabLogs {
		if zone.Get("copy-logs-dedup").InBounds(msg) && len(m.clusters) > 0 {
			var b strings.Builder
			for i, cl := range m.clusters {
				if i > 0 {
					b.WriteByte('\n')
				}
				fmt.Fprintf(&b, "[%s] x%d: %s", cl.Severity, cl.Count, cl.Template)
			}
			m.focus = FocusMain
			return m, copyToClipboard(b.String(), fmt.Sprintf("%d patterns", len(m.clusters)), "copy-logs-dedup")
		}
		if zone.Get("copy-logs-raw").InBounds(msg) && len(m.logs) > 0 {
			m.focus = FocusMain
			return m, copyToClipboard(strings.Join(m.logs, "\n"), fmt.Sprintf("%d log lines", len(m.logs)), "copy-logs-raw")
		}
	}

	for _, t := range m.metaCopyTargets() {
		zoneID := "copy-meta-" + t.key
		if zone.Get(zoneID).InBounds(msg) {
			return m, copyToClipboard(t.value, t.label, zoneID)
		}
	}

	if m.tab == TabSummary || m.tab == TabLogs {
		for i, cl := range m.visibleClusters() {
			zoneID := fmt.Sprintf("cluster-%d", i)
			if zone.Get(zoneID).InBounds(msg) {
				text := cl.Sample
				if text == "" {
					text = cl.Template
				}
				m.focus = FocusMain
				return m, copyToClipboard(text, truncate(cl.Template, 32), zoneID)
			}
		}
	}

	switch {
	case zone.Get(zonePaneSidebar).InBounds(msg), zone.Get(zonePaneBrowser).InBounds(msg):
		m.focus = FocusSidebar
	case zone.Get(zonePaneInspector).InBounds(msg):
		m.focus = FocusMain
	case zone.Get(zonePaneProcesses).InBounds(msg):
		m.focus = FocusProcesses
	case zone.Get(zonePaneMetadata).InBounds(msg):
		m.focus = FocusMetadata
	}
	return m, nil
}

func (m Model) handleMouseWheel(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	up := msg.Button == tea.MouseButtonWheelUp

	if zone.Get(zonePaneBrowser).InBounds(msg) || zone.Get(zonePaneSidebar).InBounds(msg) {
		if up && m.cursor > 0 {
			m.cursor--
			m.mainScroll = 0
			m.metaScroll = 0
			m.pendingConfirm = ""
			m.pendingLabel = ""
			return m, tea.Batch(m.detailCmd(), m.metricsCmd())
		}
		if !up && m.cursor < m.itemCount()-1 {
			m.cursor++
			m.mainScroll = 0
			m.metaScroll = 0
			m.pendingConfirm = ""
			m.pendingLabel = ""
			return m, tea.Batch(m.detailCmd(), m.metricsCmd())
		}
		return m, nil
	}

	if zone.Get(zonePaneProcesses).InBounds(msg) {
		if up && m.procScroll > 0 {
			m.procScroll--
		} else if !up && m.procScroll < max(0, len(m.containers)+2) {
			m.procScroll++
		}
		return m, nil
	}

	if zone.Get(zonePaneMetadata).InBounds(msg) {
		if up && m.metaScroll > 0 {
			m.metaScroll--
		} else if !up && m.metaScroll < 5 {
			m.metaScroll++
		}
		return m, nil
	}

	if m.tab == TabMetrics {
		if up {
			m.metricKind = (m.metricKind + metricKindCount - 1) % metricKindCount
		} else {
			m.metricKind = (m.metricKind + 1) % metricKindCount
		}
		return m, nil
	}

	if up {
		if m.mainScroll > 0 {
			m.mainScroll--
		}
	} else if maxOff := m.maxMainScroll(); m.mainScroll < maxOff {
		m.mainScroll++
	}
	return m, nil
}
