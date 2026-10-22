// Package tui contains the Bubble Tea models, lipgloss styles, overlays and
// command bar of OkShield.
package tui

import "github.com/charmbracelet/lipgloss"

// Palette — dark modern theme with neon/pastel accents (Antigravity-inspired).
var (
	ColorFocus     = lipgloss.Color("#00F5D4") // cyan: active focus
	ColorSecondary = lipgloss.Color("#7B2CBF") // violet: secondary borders
	ColorOK        = lipgloss.Color("#06D6A0") // green
	ColorWarn      = lipgloss.Color("#FFD166") // amber
	ColorErr       = lipgloss.Color("#EF476F") // coral
	ColorText      = lipgloss.Color("#E6E6F0")
	ColorMuted     = lipgloss.Color("#6C6F93")
	ColorSurface   = lipgloss.Color("#1A1B26")
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

// DefaultStyles builds the OkShield theme.
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

		Title:    lipgloss.NewStyle().Foreground(ColorFocus).Bold(true),
		Muted:    lipgloss.NewStyle().Foreground(ColorMuted),
		Text:     lipgloss.NewStyle().Foreground(ColorText),
		Badge:    lipgloss.NewStyle().Foreground(ColorSurface).Background(ColorFocus).Padding(0, 1).Bold(true),
		TabOn:    lipgloss.NewStyle().Foreground(ColorFocus).Bold(true).Underline(true).Padding(0, 1),
		TabOff:   lipgloss.NewStyle().Foreground(ColorMuted).Padding(0, 1),
		Prompt:   lipgloss.NewStyle().Foreground(ColorFocus).Bold(true),
		Selected: lipgloss.NewStyle().Foreground(ColorFocus).Bold(true),

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
