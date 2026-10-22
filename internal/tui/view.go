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
	if m.showHelp {
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
	sideW := max(minSidebarW, m.width/5)
	remW := max(20, m.width-sideW)
	browserW := remW * 42 / 100
	inspectorW := remW - browserW

	return lipgloss.JoinHorizontal(lipgloss.Top,
		m.renderSidebar(sideW, h),
		m.renderBrowser(browserW, h),
		m.renderInspector(inspectorW, h),
	)
}

func (m Model) renderBottomRow(h int) string {
	w1 := m.width / 3
	w2 := m.width / 3
	w3 := m.width - w1 - w2

	return lipgloss.JoinHorizontal(lipgloss.Top,
		m.renderProcessesBox(w1, h),
		m.renderMetadataBox(w2, h),
		m.renderCommandBox(w3, h),
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

	b.WriteString("  " + s.Title.Render("⛨ OkShield") + "\n\n")

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
		{"tab", "Tabs"},
		{"←/→", "Focus"},
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
		FootBadges:  []string{"👁 Browser", fmt.Sprintf("%d/%d", pos, total)},
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
	var body string
	switch m.tab {
	case TabSummary:
		body = m.renderPulse(bodyRows)
	case TabDetails:
		body = m.renderDetails()
	case TabLogs:
		body = m.renderLogs(bodyRows)
	case TabDiagnose:
		body = m.renderDiagnose()
	}

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
		FootBadges:  []string{"👁 " + m.tab.String(), fmt.Sprintf("%d/%d", int(m.tab)+1, int(tabCount))},
	})
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
				fmt.Fprintf(&b, "%s %s: %s\n", s.Err.Render("✖"), s.Selected.Render(cp.Name), s.Text.Render(a))
			}
			if !cp.Healthy && len(cp.Anomalies) == 0 && cp.ErrorCount > 0 {
				anomalies++
				fmt.Fprintf(&b, "%s %s: %d error logs\n", s.Warn.Render("◐"), s.Selected.Render(cp.Name), cp.ErrorCount)
			}
		}
	}
	if anomalies == 0 && b.Len() == 0 {
		b.WriteString(s.Muted.Render("● Sin anomalías activas (? para /whatsup)"))
	}
	return renderBox(boxSpec{
		Width:       w,
		Height:      h,
		BorderColor: ColorSecondary,
		TopTitle:    "Processes",
		Body:        strings.TrimRight(b.String(), "\n"),
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
		}
	}
	if fields == 0 {
		b.WriteString(s.Muted.Render("Sin recurso seleccionado"))
	}
	return renderBox(boxSpec{
		Width:       w,
		Height:      h,
		BorderColor: ColorSecondary,
		TopTitle:    "Metadata",
		Body:        strings.TrimRight(b.String(), "\n"),
		FootBadges:  []string{fmt.Sprintf("%d/%d", min(1, fields), fields)},
	})
}

func (m Model) renderCommandBox(w, h int) string {
	s := m.styles
	var b strings.Builder
	if m.focus == FocusCommand {
		b.WriteString(m.input.View() + "\n")
		if sug := Suggest(m.input.Value()); len(sug) > 0 {
			b.WriteString(s.Muted.Render("↪ " + strings.Join(sug, " · ")))
		}
	} else {
		b.WriteString(s.Title.Render("> ") + s.Text.Render(m.status) + "\n")
		b.WriteString(s.Muted.Render("/ comando · ? whatsup · d diagnose\n"))
		b.WriteString(s.Muted.Render("1/2/3 vistas · tab pestañas · q salir"))
	}

	borderColor := ColorSecondary
	if m.focus == FocusCommand {
		borderColor = ColorFocus
	}
	return renderBox(boxSpec{
		Width:       w,
		Height:      h,
		BorderColor: borderColor,
		TopTitle:    "Command",
		Body:        strings.TrimRight(b.String(), "\n"),
		FootBadges:  []string{"mem " + humanBytes(m.memAlloc)},
	})
}

func (m Model) connGlyph(c ConnState) string {
	switch c {
	case ConnUp:
		return m.styles.StatusGlyph(HealthOK)
	case ConnDown:
		return m.styles.StatusGlyph(HealthError)
	default:
		return m.styles.StatusGlyph(HealthDegraded)
	}
}

// renderList renders the active resource list in a scrolling window of
// `rows` lines and `w` columns.
func (m Model) renderList(w, rows int) string {
	s := m.styles
	n := m.itemCount()
	if n == 0 {
		if m.docker == ConnDown {
			return s.Err.Render("✖ docker no disponible")
		}
		if m.docker == ConnUnknown {
			return s.Muted.Render("cargando…")
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
		label = truncate(label, max(1, w-5))
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

// listRow returns the status glyph and plain-text label for row i of the
// active view (labels must stay unstyled so truncate() is ANSI-safe).
func (m Model) listRow(i int) (string, string) {
	s := m.styles
	switch m.view {
	case ViewNetworks:
		n := m.networks[i]
		return s.Muted.Render("⇄"), n.Name + " (" + n.Driver + ")"
	case ViewVolumes:
		return s.Muted.Render("▣"), m.volumes[i].Name
	default:
		c := m.containers[i]
		return s.StatusGlyph(containerHealth(c)), c.Name + " · " + c.Image
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
	b.WriteString(s.Title.Render("Comandos de OkShield"))
	b.WriteString("\n\n")
	for _, c := range commandTable {
		key := c.Key
		if key == "" {
			key = "-"
		}
		fmt.Fprintf(&b, "%s  %s  %s\n",
			s.Selected.Render(fmt.Sprintf("%-24s", c.Usage)),
			s.Warn.Render(fmt.Sprintf("%-7s", key)),
			s.Text.Render(c.Summary))
	}
	b.WriteString("\n")
	b.WriteString(s.Muted.Render("pulsa cualquier tecla para cerrar"))
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
