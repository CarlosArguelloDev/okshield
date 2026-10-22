// Package tui contains the Bubble Tea models, lipgloss styles, overlays and
// command bar of OkShield.
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Palette — warm, high-legibility dark theme inspired by Superfile / Gruvbox
// Dark Soft (replaces harsh neon colors with comfortable earth/pastel tones).
var (
	ColorFocus     = lipgloss.Color("#EADBB2") // warm cream: active panel border
	ColorSidebar   = lipgloss.Color("#E08A3C") // warm amber/orange: sidebar border
	ColorSecondary = lipgloss.Color("#5C6370") // muted slate gray: inactive panel border
	ColorAccent    = lipgloss.Color("#8EC07C") // sage aqua/green: headers & active cursor
	ColorSection   = lipgloss.Color("#E06C75") // warm coral: sidebar section headers
	ColorKey       = lipgloss.Color("#61AFEF") // soft sky blue: keybinding labels
	ColorOK        = lipgloss.Color("#98C379") // soft sage green
	ColorWarn      = lipgloss.Color("#E5C07B") // warm wheat/gold
	ColorErr       = lipgloss.Color("#E06C75") // soft terracotta red
	ColorText      = lipgloss.Color("#EBDBB2") // warm cream text
	ColorMuted     = lipgloss.Color("#7C6F64") // warm gray-brown muted text
	ColorSurface   = lipgloss.Color("#1D2021") // deep warm charcoal
)

// Status glyphs (AGENTS.md §6).
const (
	GlyphRunning  = "●"
	GlyphDegraded = "◐"
	GlyphError    = "✖"
)

// Styles groups every reusable lipgloss style so views never build styles in
// the render hot path.
type Styles struct {
	Panel        lipgloss.Style
	PanelFocused lipgloss.Style
	Overlay      lipgloss.Style

	Title    lipgloss.Style
	Section  lipgloss.Style
	Key      lipgloss.Style
	Muted    lipgloss.Style
	Text     lipgloss.Style
	Badge    lipgloss.Style
	TabOn    lipgloss.Style
	TabOff   lipgloss.Style
	Prompt   lipgloss.Style
	Selected lipgloss.Style

	OK   lipgloss.Style
	Warn lipgloss.Style
	Err  lipgloss.Style
}

// DefaultStyles builds the warm Superfile-inspired OkShield theme.
func DefaultStyles() Styles {
	panel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorSecondary).
		Padding(0, 1)

	return Styles{
		Panel:        panel,
		PanelFocused: panel.BorderForeground(ColorFocus),
		Overlay: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorFocus).
			Padding(1, 2),

		Title:    lipgloss.NewStyle().Foreground(ColorAccent),
		Section:  lipgloss.NewStyle().Foreground(ColorSection),
		Key:      lipgloss.NewStyle().Foreground(ColorKey),
		Muted:    lipgloss.NewStyle().Foreground(ColorMuted),
		Text:     lipgloss.NewStyle().Foreground(ColorText),
		Badge:    lipgloss.NewStyle().Foreground(ColorSurface).Background(ColorAccent).Padding(0, 1),
		TabOn:    lipgloss.NewStyle().Foreground(ColorText),
		TabOff:   lipgloss.NewStyle().Foreground(ColorMuted),
		Prompt:   lipgloss.NewStyle().Foreground(ColorAccent),
		Selected: lipgloss.NewStyle().Foreground(ColorText),

		OK:   lipgloss.NewStyle().Foreground(ColorOK),
		Warn: lipgloss.NewStyle().Foreground(ColorWarn),
		Err:  lipgloss.NewStyle().Foreground(ColorErr),
	}
}

// Health is the minimal status classification used by status glyphs.
type Health int

const (
	HealthOK Health = iota
	HealthDegraded
	HealthError
)

// StatusGlyph renders the colored glyph for a health state.
func (s Styles) StatusGlyph(h Health) string {
	switch h {
	case HealthDegraded:
		return s.Warn.Render(GlyphDegraded)
	case HealthError:
		return s.Err.Render(GlyphError)
	default:
		return s.OK.Render(GlyphRunning)
	}
}

// boxSpec configures a Superfile-style bordered box with optional header
// divider, top-left embedded title (`─┤Title├──`) and bottom-right embedded
// badges (`──┤Label├┤1/6├─`).
type boxSpec struct {
	Width       int
	Height      int
	BorderColor lipgloss.Color
	TopTitle    string   // embedded in top border: ─┤TopTitle├──
	Header      string   // rendered on row 1 followed by a full-width divider row
	Body        string   // main content lines
	FootBadges  []string // embedded in bottom-right border: ─┤b1├┤b2├─
}

// renderBox draws a box matching the Superfile TUI aesthetic.
func renderBox(spec boxSpec) string {
	w := max(6, spec.Width)
	h := max(3, spec.Height)
	innerW := w - 2
	innerH := h - 2

	bc := lipgloss.NewStyle().Foreground(spec.BorderColor)
	tc := lipgloss.NewStyle().Foreground(ColorText)

	// 1. Top border: ┌─┤TopTitle├────────┐ or ┌───────────────────┐
	topLine := buildTopBorder(innerW, spec.TopTitle, bc, tc)

	// 2. Collect interior lines (each padded/truncated to innerW)
	var rows []string
	if spec.Header != "" && innerH >= 2 {
		rows = append(rows, bc.Render("│")+padOrClip(spec.Header, innerW)+bc.Render("│"))
		rows = append(rows, bc.Render("├"+strings.Repeat("─", innerW)+"┤"))
	}

	bodyLines := strings.Split(spec.Body, "\n")
	if spec.Body == "" {
		bodyLines = nil
	}
	for _, line := range bodyLines {
		if len(rows) >= innerH {
			break
		}
		rows = append(rows, bc.Render("│")+padOrClip(line, innerW)+bc.Render("│"))
	}
	emptyRow := bc.Render("│") + strings.Repeat(" ", innerW) + bc.Render("│")
	for len(rows) < innerH {
		rows = append(rows, emptyRow)
	}

	// 3. Bottom border: └──────────┤Badge1├┤Badge2├─┘
	botLine := buildBottomBorder(innerW, spec.FootBadges, bc, tc)

	var out strings.Builder
	out.WriteString(topLine)
	out.WriteByte('\n')
	for _, r := range rows {
		out.WriteString(r)
		out.WriteByte('\n')
	}
	out.WriteString(botLine)
	return out.String()
}

func buildTopBorder(innerW int, title string, bc, tc lipgloss.Style) string {
	if title == "" {
		return bc.Render("┌" + strings.Repeat("─", innerW) + "┐")
	}
	tagPlain := "┤" + title + "├"
	tagW := lipgloss.Width(tagPlain)
	if tagW+2 > innerW {
		return bc.Render("┌" + strings.Repeat("─", innerW) + "┐")
	}
	leftDashes := 1
	rightDashes := innerW - leftDashes - tagW
	return bc.Render("┌"+strings.Repeat("─", leftDashes)+"┤") +
		tc.Render(title) +
		bc.Render("├"+strings.Repeat("─", rightDashes)+"┐")
}

func buildBottomBorder(innerW int, badges []string, bc, tc lipgloss.Style) string {
	if len(badges) == 0 {
		return bc.Render("└" + strings.Repeat("─", innerW) + "┘")
	}
	var segPlain strings.Builder
	var segStyled strings.Builder
	for i, b := range badges {
		if i > 0 {
			segPlain.WriteString("─")
			segStyled.WriteString(bc.Render("─"))
		}
		segPlain.WriteString(fmt.Sprintf("┤%s├", b))
		segStyled.WriteString(bc.Render("┤") + tc.Render(b) + bc.Render("├"))
	}
	segW := lipgloss.Width(segPlain.String())
	rightDashes := 1
	leftDashes := innerW - segW - rightDashes
	if leftDashes < 1 {
		return bc.Render("└" + strings.Repeat("─", innerW) + "┘")
	}
	return bc.Render("└"+strings.Repeat("─", leftDashes)) +
		segStyled.String() +
		bc.Render(strings.Repeat("─", rightDashes)+"┘")
}

func padOrClip(s string, width int) string {
	if width <= 0 {
		return ""
	}
	// Preserve 1 space left/right padding inside the box.
	contentW := max(1, width-2)
	w := lipgloss.Width(s)
	if w > contentW {
		// ANSI-aware width clip via lipgloss MaxWidth.
		s = lipgloss.NewStyle().MaxWidth(contentW).Render(s)
		w = lipgloss.Width(s)
	}
	pad := max(0, contentW-w)
	return " " + s + strings.Repeat(" ", pad) + " "
}
