package tui

import (
	"strings"
	"testing"
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := s.StatusGlyph(tt.h); !strings.Contains(got, tt.want) {
				t.Errorf("StatusGlyph(%v) = %q, want glyph %q", tt.h, got, tt.want)
			}
		})
	}
}
