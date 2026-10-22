package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func sized(t *testing.T) Model {
	t.Helper()
	m, _ := New().Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	return m.(Model)
}

func key(m Model, k string) Model {
	var msg tea.KeyMsg
	switch k {
	case "enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	case "tab":
		msg = tea.KeyMsg{Type: tea.KeyTab}
	case "esc":
		msg = tea.KeyMsg{Type: tea.KeyEsc}
	default:
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
	}
	out, _ := m.Update(msg)
	return out.(Model)
}

func TestKeybindingsSwitchViewsAndTabs(t *testing.T) {
	tests := []struct {
		keys     []string
		wantView View
		wantTab  Tab
	}{
		{[]string{"2"}, ViewNetworks, TabSummary},
		{[]string{"3"}, ViewVolumes, TabSummary},
		{[]string{"3", "1"}, ViewContainers, TabSummary},
		{[]string{"d"}, ViewContainers, TabDiagnose},
		{[]string{"d", "?"}, ViewContainers, TabSummary},
		{[]string{"tab", "tab"}, ViewContainers, TabLogs},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.keys, ","), func(t *testing.T) {
			m := sized(t)
			for _, k := range tt.keys {
				m = key(m, k)
			}
			if m.view != tt.wantView || m.tab != tt.wantTab {
				t.Errorf("view=%v tab=%v, want view=%v tab=%v", m.view, m.tab, tt.wantView, tt.wantTab)
			}
		})
	}
}

func TestCommandBarDispatch(t *testing.T) {
	m := sized(t)
	m = key(m, "/") // focus command bar, prefilled with "/"
	if m.focus != FocusCommand {
		t.Fatalf("focus = %v, want FocusCommand", m.focus)
	}
	m = key(m, "volumes")
	m = key(m, "enter")
	if m.view != ViewVolumes {
		t.Errorf("view = %v, want Volumes", m.view)
	}
	if m.focus == FocusCommand {
		t.Errorf("command bar should lose focus after enter")
	}

	m = key(m, "/")
	m = key(m, "help")
	m = key(m, "enter")
	if !m.showHelp {
		t.Errorf("/help should open overlay")
	}
	m = key(m, "x")
	if m.showHelp {
		t.Errorf("any key should close overlay")
	}
}

func TestViewFitsTerminal(t *testing.T) {
	m := sized(t)
	out := m.View()
	if h := lipgloss.Height(out); h > 30 {
		t.Errorf("view height %d exceeds terminal height 30", h)
	}
	for i, line := range strings.Split(out, "\n") {
		if w := lipgloss.Width(line); w > 120 {
			t.Errorf("line %d width %d exceeds terminal width 120", i, w)
		}
	}
}

func TestHumanBytes(t *testing.T) {
	tests := []struct {
		in   uint64
		want string
	}{
		{0, "0B"},
		{512, "512B"},
		{2048, "2.0KB"},
		{5 * 1024 * 1024, "5.0MB"},
	}
	for _, tt := range tests {
		if got := humanBytes(tt.in); got != tt.want {
			t.Errorf("humanBytes(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
