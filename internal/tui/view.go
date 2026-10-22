package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	headerHeight  = 1
	commandHeight = 3 // rounded border + 1 line
	hintHeight    = 1
	minSidebarW   = 26
)

// View renders the whole screen.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "iniciando okshield…"
	}
	if m.showHelp {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.renderHelp())
	}
	bodyH := max(3, m.height-headerHeight-commandHeight-hintHeight)
	return lipgloss.JoinVertical(lipgloss.Left,
		m.renderHeader(),
		m.renderBody(bodyH),
		m.renderCommandBar(),
		m.renderHint(),
	)
}

func (m Model) renderHeader() string {
	s := m.styles
	left := lipgloss.JoinHorizontal(lipgloss.Top,
		s.Title.Render("⛨ OkShield"),
		s.Muted.Render("  host "), s.Text.Render(m.hostname),
		s.Muted.Render("  docker "), m.connGlyph(m.docker),
		s.Muted.Render("  otlp "), m.connGlyph(m.otel),
	)
	badge := s.Badge.Render("mem " + humanBytes(m.memAlloc))
	gap := max(1, m.width-lipgloss.Width(left)-lipgloss.Width(badge))
	return left + strings.Repeat(" ", gap) + badge
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

func (m Model) renderBody(h int) string {
	sideW := max(minSidebarW, m.width/4)
	mainW := max(10, m.width-sideW)
	return lipgloss.JoinHorizontal(lipgloss.Top,
		m.renderSidebar(sideW, h),
		m.renderMain(mainW, h),
	)
}

// panel renders content inside a rounded border with an exact outer size.
func (m Model) panel(focused bool, w, h int, content string) string {
	st := m.styles.Panel
	if focused {
		st = m.styles.PanelFocused
	}
	// lipgloss v1: Width/Height include padding but exclude the border.
	return st.Width(max(1, w-2)).Height(max(1, h-2)).MaxHeight(h).Render(content)
}

func (m Model) renderSidebar(w, h int) string {
	s := m.styles
	var b strings.Builder
	for i := View(0); i <= ViewVolumes; i++ {
		label := fmt.Sprintf("%d %s", i+1, i)
		if i == m.view {
			b.WriteString(s.TabOn.Render(label))
		} else {
			b.WriteString(s.TabOff.Render(label))
		}
		b.WriteByte('\n')
	}
	b.WriteString(s.Muted.Render(strings.Repeat("─", max(1, w-4))))
	b.WriteString("\n")
	b.WriteString(m.renderList(w-4, h-2-4))
	return m.panel(m.focus == FocusSidebar, w, h, b.String())
}

// renderList renders the active resource list in a scrolling window of
// `rows` lines and `w` columns.
func (m Model) renderList(w, rows int) string {
	s := m.styles
	n := m.itemCount()
	if n == 0 {
		if m.docker == ConnDown {
			return s.Err.Render("docker no disponible")
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
		label = truncate(label, max(1, w-4))
		if i == m.cursor {
			b.WriteString(s.Selected.Render("› ") + glyph + " " + s.Selected.Render(label))
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
		return s.StatusGlyph(containerHealth(c)), c.Name
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

func (m Model) renderMain(w, h int) string {
	s := m.styles
	tabs := make([]string, 0, tabCount)
	for t := Tab(0); t < tabCount; t++ {
		if t == m.tab {
			tabs = append(tabs, s.TabOn.Render(t.String()))
		} else {
			tabs = append(tabs, s.TabOff.Render(t.String()))
		}
	}
	header := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
	body := s.Muted.Render(m.tabPlaceholder())
	if m.tab == TabDetails {
		body = m.renderDetails()
	}
	return m.panel(m.focus == FocusMain, w, h, header+"\n\n"+body)
}

func (m Model) tabPlaceholder() string {
	switch m.tab {
	case TabSummary:
		return "Pulsa ? o /whatsup para un resumen determinista (sin IA)."
	case TabDetails:
		return "Métricas en vivo, IPs, puertos, mounts y drivers del recurso seleccionado."
	case TabLogs:
		return "Logs deduplicados en plantillas (ring buffer de 500 líneas)."
	default:
		return "Pulsa d o /diagnose [container] para clasificar la causa raíz."
	}
}

func (m Model) renderCommandBar() string {
	st := m.styles.Panel
	if m.focus == FocusCommand {
		st = m.styles.PanelFocused
	}
	return st.Width(max(1, m.width-2)).Render(m.input.View())
}

func (m Model) renderHint() string {
	s := m.styles
	hint := "/ comando · ? pulse · d diagnose · 1/2/3 vistas · tab pestañas · q salir"
	left := s.Muted.Render(hint)
	right := s.Text.Render(m.status)
	gap := max(1, m.width-lipgloss.Width(left)-lipgloss.Width(right))
	return left + strings.Repeat(" ", gap) + right
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
