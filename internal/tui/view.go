package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	minSidebarW = 22
	bottomH     = 8
)

// View renders the whole screen in a Superfile-inspired multi-pane layout:
// Top row:    [Sidebar] [Resource Browser] [Active Inspector/Analysis Pane]
// Bottom row: [Processes / Pulse] [Metadata] [Command / Telemetry]
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "iniciando okshield…"
	}
	if m.focus == FocusCommand || m.showHelp {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.renderHelp())
	}
	if m.showOtel {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.renderOtelStatus())
	}

	botH := bottomH
	if m.height < 20 {
		botH = max(5, m.height/3)
	}
	topH := max(6, m.height-botH)

	return lipgloss.JoinVertical(lipgloss.Left,
		m.renderTopRow(topH),
		m.renderBottomRow(botH),
	)
}

func (m Model) renderTopRow(h int) string {
	sideW := max(minSidebarW, m.width/6)
	remW := max(20, m.width-sideW)
	browserW := max(24, remW*26/100)
	inspectorW := max(20, remW-browserW)

	return lipgloss.JoinHorizontal(lipgloss.Top,
		m.renderSidebar(sideW, h),
		m.renderBrowser(browserW, h),
		m.renderInspector(inspectorW, h),
	)
}

func (m Model) renderBottomRow(h int) string {
	w1 := m.width / 2
	w2 := m.width - w1

	return lipgloss.JoinHorizontal(lipgloss.Top,
		m.renderProcessesBox(w1, h),
		m.renderMetadataBox(w2, h),
	)
}

func (m Model) sectionDivider(title string, innerW int) string {
	s := m.styles
	prefix := "▪ " + title + " "
	rem := max(1, innerW-lipgloss.Width(prefix)-2)
	return s.Section.Render(prefix) + s.Muted.Render(strings.Repeat("─", rem))
}

func (m Model) renderSidebar(w, h int) string {
	s := m.styles
	innerW := max(8, w-2)
	var b strings.Builder

	b.WriteString("  " + s.Title.Render("✦ OkShield") + "\n\n")

	icons := [...]string{"🐳", "⇄ ", "▣ "}
	for i := View(0); i <= ViewVolumes; i++ {
		cursor := "  "
		style := s.Text
		if i == m.view {
			cursor = s.Title.Render("> ")
			style = s.Selected
		}
		fmt.Fprintf(&b, "%s%s %s\n", cursor, icons[i], style.Render(i.String()))
	}

	b.WriteByte('\n')
	b.WriteString(m.sectionDivider("Pinned", innerW) + "\n")
	for idx, t := range tabNames {
		cursor := "  "
		st := s.Text
		if Tab(idx) == m.tab {
			cursor = s.Title.Render("> ")
			st = s.Selected
		}
		fmt.Fprintf(&b, "%s%s\n", cursor, st.Render(t))
	}

	b.WriteByte('\n')
	b.WriteString(m.sectionDivider("Host", innerW) + "\n")
	fmt.Fprintf(&b, "  %s %s\n", s.Muted.Render("host"), s.Text.Render(truncate(m.hostname, max(4, innerW-9))))
	fmt.Fprintf(&b, "  %s %s  %s %s\n",
		s.Muted.Render("dkr"), m.connGlyph(m.docker),
		s.Muted.Render("otlp"), m.connGlyph(m.otel),
	)
	fmt.Fprintf(&b, "  %s %s\n\n", s.Muted.Render("mem"), s.Warn.Render(humanBytes(m.memAlloc)))

	controls := [][2]string{
		{"↑/↓", "Scroll"},
		{"1/2/3", "Views"},
		{"←/→", "Tabs"},
		{"tab", "Focus"},
		{"?", "Whatsup"},
		{"d", "Diagnose"},
		{"/", "Command"},
		{"q", "Quit"},
	}
	for i, kv := range controls {
		fmt.Fprintf(&b, "  %s %s",
			s.Key.Render(fmt.Sprintf("%-7s", kv[0])),
			s.Muted.Render(kv[1]),
		)
		if i < len(controls)-1 {
			b.WriteByte('\n')
		}
	}

	borderColor := ColorSidebar
	if m.focus == FocusSidebar {
		borderColor = ColorFocus
	}
	return renderBox(boxSpec{
		Width:       w,
		Height:      h,
		BorderColor: borderColor,
		Body:        b.String(),
	})
}

func (m Model) renderBrowser(w, h int) string {
	s := m.styles
	pathLabel := "/docker/containers"
	switch m.view {
	case ViewNetworks:
		pathLabel = "/docker/networks"
	case ViewVolumes:
		pathLabel = "/docker/volumes"
	}
	header := s.Title.Render("🐳 " + pathLabel)

	// Subtract 2 border rows + 2 header/divider rows.
	bodyRows := max(1, h-4)
	body := m.renderList(max(6, w-4), bodyRows)

	total := m.itemCount()
	pos := 0
	if total > 0 {
		pos = m.cursor + 1
	}

	borderColor := ColorSecondary
	if m.focus == FocusSidebar {
		borderColor = ColorFocus
	}
	return renderBox(boxSpec{
		Width:       w,
		Height:      h,
		BorderColor: borderColor,
		Header:      header,
		Body:        body,
		FootBadges:  []string{"Browser", fmt.Sprintf("%d/%d", pos, total)},
	})
}

func (m Model) renderInspector(w, h int) string {
	s := m.styles
	tabs := make([]string, 0, tabCount)
	for t := Tab(0); t < tabCount; t++ {
		if t == m.tab {
			tabs = append(tabs, s.Title.Render("▸ "+t.String()))
		} else {
			tabs = append(tabs, s.Muted.Render(t.String()))
		}
	}
	header := strings.Join(tabs, s.Muted.Render(" │ "))

	bodyRows := max(1, h-4)
	body := scrollLines(m.inspectorBody(bodyRows), m.mainScroll, bodyRows)

	borderColor := ColorSecondary
	if m.focus == FocusMain {
		borderColor = ColorFocus
	}
	return renderBox(boxSpec{
		Width:       w,
		Height:      h,
		BorderColor: borderColor,
		Header:      header,
		Body:        body,
		FootBadges:  []string{m.tab.String(), fmt.Sprintf("%d/%d", int(m.tab)+1, int(tabCount))},
	})
}

func (m Model) inspectorBody(bodyRows int) string {
	switch m.tab {
	case TabSummary:
		return m.renderPulse(bodyRows)
	case TabMetrics:
		return m.renderMetrics(bodyRows)
	case TabDetails:
		return m.renderDetails()
	case TabLogs:
		return m.renderLogs(bodyRows)
	case TabDiagnose:
		return m.renderDiagnose()
	}
	return ""
}

func (m Model) renderProcessesBox(w, h int) string {
	s := m.styles
	var b strings.Builder
	anomalies := 0
	if m.hasPulse && len(m.pulse.Containers) > 0 {
		fmt.Fprintf(&b, "%s %d ok  %s %d warn  %s %d err\n",
			s.StatusGlyph(HealthOK), m.pulse.Running,
			s.StatusGlyph(HealthDegraded), m.pulse.Degraded,
			s.StatusGlyph(HealthError), m.pulse.Failed,
		)
		for _, cp := range m.pulse.Containers {
			for _, a := range cp.Anomalies {
				anomalies++
				fmt.Fprintf(&b, "%s %s: %s\n", s.StatusGlyph(HealthError), s.Selected.Render(cp.Name), s.Text.Render(a))
			}
			if !cp.Healthy && len(cp.Anomalies) == 0 && cp.ErrorCount > 0 {
				anomalies++
				fmt.Fprintf(&b, "%s %s: %d error logs\n", s.StatusGlyph(HealthDegraded), s.Selected.Render(cp.Name), cp.ErrorCount)
			}
		}
	}
	if anomalies == 0 && b.Len() == 0 {
		b.WriteString(s.StatusBadge(HealthOK, GlyphOK) + " " + s.Muted.Render("Sin anomalías activas (? para /whatsup)"))
	}
	bodyRows := max(1, h-2)
	borderColor := ColorSecondary
	if m.focus == FocusProcesses {
		borderColor = ColorFocus
	}
	return renderBox(boxSpec{
		Width:       w,
		Height:      h,
		BorderColor: borderColor,
		TopTitle:    "Processes",
		Body:        scrollLines(strings.TrimRight(b.String(), "\n"), m.procScroll, bodyRows),
		FootBadges:  []string{fmt.Sprintf("%d/%d", anomalies, len(m.containers))},
	})
}

func (m Model) renderMetadataBox(w, h int) string {
	s := m.styles
	var b strings.Builder
	row := func(k, v string) {
		if v == "" {
			v = "-"
		}
		fmt.Fprintf(&b, "%s %s\n", s.Muted.Render(fmt.Sprintf("%-14s", k)), s.Text.Render(v))
	}

	fields := 0
	switch m.view {
	case ViewNetworks:
		if m.cursor >= 0 && m.cursor < len(m.networks) {
			n := m.networks[m.cursor]
			row("NetworkName", n.Name)
			row("Driver", n.Driver)
			row("Scope", n.Scope)
			if len(n.Subnets) > 0 {
				row("Subnet", n.Subnets[0].CIDR)
			}
			fields = 4
		}
	case ViewVolumes:
		if m.cursor >= 0 && m.cursor < len(m.volumes) {
			v := m.volumes[m.cursor]
			row("VolumeName", v.Name)
			row("Driver", v.Driver)
			row("Mountpoint", v.Mountpoint)
			fields = 3
		}
	default:
		if c, ok := m.selectedContainer(); ok {
			row("ContainerName", c.Name)
			row("Image", c.Image)
			row("State", c.Status)
			if len(c.Networks) > 0 {
				row("NetworkIP", c.Networks[0].Network+" "+c.Networks[0].IP)
			}
			if len(c.Mounts) > 0 {
				row("Mount", c.Mounts[0].Destination)
			}
			fields = 5
		} else if len(m.containers) > 0 {
			row("Scope", "General (All)")
			row("Host", m.hostname)
			row("Containers", fmt.Sprintf("%d total", len(m.containers)))
			row("Networks", fmt.Sprintf("%d redes", len(m.networks)))
			row("Volumes", fmt.Sprintf("%d volúmenes", len(m.volumes)))
			fields = 5
		}
	}
	if fields == 0 {
		b.WriteString(s.Muted.Render("Sin recurso seleccionado"))
	}
	bodyRows := max(1, h-2)
	borderColor := ColorSecondary
	if m.focus == FocusMetadata {
		borderColor = ColorFocus
	}
	return renderBox(boxSpec{
		Width:       w,
		Height:      h,
		BorderColor: borderColor,
		TopTitle:    "Metadata",
		Body:        scrollLines(strings.TrimRight(b.String(), "\n"), m.metaScroll, bodyRows),
		FootBadges:  []string{fmt.Sprintf("%d/%d", min(1, fields), fields)},
	})
}

func scrollLines(body string, offset, rows int) string {
	if offset <= 0 || body == "" {
		return body
	}
	lines := strings.Split(body, "\n")
	maxOff := max(0, len(lines)-max(1, rows))
	if offset > maxOff {
		offset = maxOff
	}
	return strings.Join(lines[offset:], "\n")
}

func (m Model) connGlyph(c ConnState) string {
	switch c {
	case ConnUp:
		return m.styles.StatusBadge(HealthOK, GlyphOK)
	case ConnDown:
		return m.styles.StatusBadge(HealthError, GlyphDown)
	default:
		return m.styles.StatusBadge(HealthDegraded, GlyphDegraded)
	}
}

// renderList renders the active resource list in a scrolling window of
// `rows` lines and `w` columns.
func (m Model) renderList(w, rows int) string {
	s := m.styles
	n := m.itemCount()
	if n == 0 {
		if m.docker == ConnDown {
			return s.StatusBadge(HealthError, GlyphDown) + " " + s.Err.Render("docker no disponible")
		}
		if m.docker == ConnUnknown {
			return s.StatusGlyph(HealthInfo) + " " + s.Muted.Render("cargando…")
		}
		return s.Muted.Render("(vacío)")
	}
	rows = max(1, rows)
	start := 0
	if m.cursor >= rows {
		start = m.cursor - rows + 1
	}
	end := min(n, start+rows)

	var b strings.Builder
	for i := start; i < end; i++ {
		glyph, label := m.listRow(i)
		glyphW := lipgloss.Width(glyph)
		label = truncate(label, max(1, w-glyphW-4))
		if i == m.cursor {
			b.WriteString(s.Title.Render("> ") + glyph + " " + s.Selected.Render(label))
		} else {
			b.WriteString("  " + glyph + " " + s.Text.Render(label))
		}
		if i < end-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// listRow returns the status badge and plain-text label for row i of the
// active view (labels must stay unstyled so truncate() is ANSI-safe).
func (m Model) listRow(i int) (string, string) {
	s := m.styles
	switch m.view {
	case ViewNetworks:
		n := m.networks[i]
		return s.StatusBadge(HealthInfo, " NET "), n.Name + " (" + n.Driver + ")"
	case ViewVolumes:
		return s.StatusBadge(HealthInfo, " VOL "), m.volumes[i].Name
	default:
		if i == 0 {
			return s.StatusBadge(HealthInfo, " ALL "), "General"
		}
		c := m.containers[i-1]
		h := containerHealth(c)
		if h == HealthError {
			return s.StatusBadge(HealthError, GlyphDown), c.Name
		}
		return s.StatusGlyph(h), c.Name
	}
}

// truncate shortens plain text to at most w runes, adding an ellipsis.
func truncate(str string, w int) string {
	if lipgloss.Width(str) <= w {
		return str
	}
	r := []rune(str)
	if w <= 1 || len(r) <= 1 {
		return "…"
	}
	return string(r[:min(len(r), w-1)]) + "…"
}

func (m Model) renderHelp() string {
	s := m.styles
	var b strings.Builder

	b.WriteString(s.Title.Render("✦ Comandos de OkShield"))
	b.WriteString("\n\n")

	if m.focus == FocusCommand {
		b.WriteString(s.Key.Render("Comando: ") + m.input.View())
		b.WriteString("\n\n")
	}

	b.WriteString(m.sectionDivider("Comandos Disponibles", 62) + "\n")
	matches := Suggest(m.input.Value())
	matchSet := make(map[string]bool, len(matches))
	for _, u := range matches {
		matchSet[u] = true
	}

	var avail []string
	for _, c := range commandTable {
		cmdName := firstField(c.Usage)
		if m.focus == FocusCommand && m.input.Value() != "" && m.input.Value() != "/" && matchSet[c.Usage] {
			avail = append(avail, s.StatusBadge(HealthInfo, " "+cmdName+" "))
		} else {
			avail = append(avail, s.Key.Render(cmdName))
		}
	}
	b.WriteString("  " + strings.Join(avail, s.Muted.Render("  ·  ")) + "\n\n")

	b.WriteString(m.sectionDivider("Descripción y Detalles", 62) + "\n")
	for _, c := range commandTable {
		key := c.Key
		if key == "" {
			key = "-"
		}
		usageStyle := s.Selected
		if m.focus == FocusCommand && m.input.Value() != "" && m.input.Value() != "/" && !matchSet[c.Usage] {
			usageStyle = s.Muted
		}
		fmt.Fprintf(&b, "  %s  %s  %s\n",
			usageStyle.Render(fmt.Sprintf("%-22s", c.Usage)),
			s.Warn.Render(fmt.Sprintf("%-7s", key)),
			s.Text.Render(c.Summary))
	}
	b.WriteString("\n")
	if m.focus == FocusCommand {
		b.WriteString(s.Muted.Render("enter ejecutar  ·  tab autocompletar  ·  esc cerrar"))
	} else {
		b.WriteString(s.Muted.Render("pulsa cualquier tecla para cerrar"))
	}
	return s.Overlay.Render(b.String())
}

// humanBytes formats a byte count compactly (e.g. "4.2MB").
func humanBytes(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%dB", n)
	}
	div, exp := uint64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(n)/float64(div), "KMGTPE"[exp])
}
