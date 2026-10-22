package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestStatusGlyph(t *testing.T) {
	s := DefaultStyles()
	tests := []struct {
		name string
		h    Health
		want string
	}{
		{"ok", HealthOK, GlyphRunning},
		{"degraded", HealthDegraded, GlyphDegraded},
		{"error", HealthError, GlyphError},
		{"info", HealthInfo, GlyphInfo},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := s.StatusGlyph(tt.h); !strings.Contains(got, tt.want) {
				t.Errorf("StatusGlyph(%v) = %q, want badge %q", tt.h, got, tt.want)
			}
		})
	}
	if got := s.StatusBadge(HealthOK, GlyphOK); !strings.Contains(got, " OK ") {
		t.Errorf("StatusBadge(OK) = %q, want ' OK '", got)
	}
	if got := s.StatusBadge(HealthError, GlyphDown); !strings.Contains(got, " DOWN ") {
		t.Errorf("StatusBadge(DOWN) = %q, want ' DOWN '", got)
	}
}

func TestRenderBoxDimensionsAndBadges(t *testing.T) {
	box := renderBox(boxSpec{
		Width:       40,
		Height:      8,
		BorderColor: ColorFocus,
		TopTitle:    "Metadata",
		Header:      "Header Line",
		Body:        "row 1\nrow 2",
		FootBadges:  []string{"Browser", "1/6"},
	})
	if gotH := lipgloss.Height(box); gotH != 8 {
		t.Errorf("Height = %d, want 8", gotH)
	}
	for i, line := range strings.Split(box, "\n") {
		if gotW := lipgloss.Width(line); gotW != 40 {
			t.Errorf("line %d width = %d, want 40", i, gotW)
		}
	}
	for _, want := range []string{"Metadata", "Header Line", "Browser", "1/6"} {
		if !strings.Contains(box, want) {
			t.Errorf("box missing %q", want)
		}
	}
}
