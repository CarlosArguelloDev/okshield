package tui

import (
	"reflect"
	"testing"
)

func TestParseCommand(t *testing.T) {
	tests := []struct {
		in       string
		wantKind CommandKind
		wantName string
		wantArgs []string
	}{
		{"/whatsup", CmdWhatsup, "whatsup", nil},
		{"/pulse", CmdWhatsup, "whatsup", nil},
		{"  /PULSE  ", CmdWhatsup, "whatsup", nil},
		{"whatsup", CmdWhatsup, "whatsup", nil},
		{"/diagnose api", CmdDiagnose, "diagnose", []string{"api"}},
		{"/diagnose", CmdDiagnose, "diagnose", nil},
		{"/containers", CmdContainers, "containers", nil},
		{"/networks", CmdNetworks, "networks", nil},
		{"/volumes", CmdVolumes, "volumes", nil},
		{"/otel status", CmdOtelStatus, "otel", []string{"status"}},
		{"/otel", CmdOtelStatus, "otel", nil},
		{"/otel bogus", CmdUnknown, "otel", []string{"bogus"}},
		{"/prune-telemetry", CmdPruneTelemetry, "prune-telemetry", nil},
		{"/lang en", CmdLang, "lang", []string{"en"}},
		{"/idioma es", CmdLang, "lang", []string{"es"}},
		{"/q", CmdQuit, "quit", nil},
		{"/nope", CmdUnknown, "nope", nil},
		{"", CmdUnknown, "", nil},
		{"/", CmdUnknown, "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got := ParseCommand(tt.in)
			if got.Kind != tt.wantKind {
				t.Errorf("Kind = %v, want %v", got.Kind, tt.wantKind)
			}
			if got.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", got.Name, tt.wantName)
			}
			if len(got.Args) != 0 || len(tt.wantArgs) != 0 {
				if !reflect.DeepEqual(got.Args, tt.wantArgs) {
					t.Errorf("Args = %v, want %v", got.Args, tt.wantArgs)
				}
			}
		})
	}
}

func TestSuggest(t *testing.T) {
	tests := []struct {
		prefix string
		want   []string
	}{
		{"/d", []string{"/diagnose [container]"}},
		{"/ne", []string{"/networks"}},
		{"/zz", []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.prefix, func(t *testing.T) {
			if got := Suggest(tt.prefix); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Suggest(%q) = %v, want %v", tt.prefix, got, tt.want)
			}
		})
	}
}
