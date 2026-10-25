package version

import (
	"runtime/debug"
	"strings"
	"testing"
)

func TestStringContainsBuildMetadata(t *testing.T) {
	s := String()
	for _, want := range []string{"okshield", Version, Commit} {
		if !strings.Contains(s, want) {
			t.Errorf("String() = %q, want it to contain %q", s, want)
		}
	}
}

func TestApplyBuildInfoFallback(t *testing.T) {
	// Reset to default dev values
	origVer, origCommit, origDate := Version, Commit, Date
	defer func() {
		Version, Commit, Date = origVer, origCommit, origDate
	}()

	Version = "dev"
	Commit = "none"
	Date = "unknown"

	fakeInfo := &debug.BuildInfo{
		Main: debug.Module{
			Version: "v1.2.3",
		},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "1234567890abcdef"},
			{Key: "vcs.time", Value: "2026-10-25T12:00:00Z"},
			{Key: "vcs.modified", Value: "true"},
		},
	}

	applyBuildInfo(fakeInfo)

	if Version != "v1.2.3" {
		t.Errorf("expected Version 'v1.2.3', got %q", Version)
	}
	if Commit != "1234567-dirty" {
		t.Errorf("expected Commit '1234567-dirty', got %q", Commit)
	}
	if Date != "2026-10-25T12:00:00Z" {
		t.Errorf("expected Date '2026-10-25T12:00:00Z', got %q", Date)
	}
}

func TestApplyBuildInfoPrecedence(t *testing.T) {
	// If ldflags already set Version/Commit/Date, BuildInfo should not overwrite them
	origVer, origCommit, origDate := Version, Commit, Date
	defer func() {
		Version, Commit, Date = origVer, origCommit, origDate
	}()

	Version = "v0.9.9"
	Commit = "custom"
	Date = "2026-01-01"

	fakeInfo := &debug.BuildInfo{
		Main: debug.Module{
			Version: "v1.2.3",
		},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "1234567890abcdef"},
			{Key: "vcs.time", Value: "2026-10-25T12:00:00Z"},
		},
	}

	applyBuildInfo(fakeInfo)

	if Version != "v0.9.9" {
		t.Errorf("expected ldflags Version to be preserved, got %q", Version)
	}
	if Commit != "custom" {
		t.Errorf("expected ldflags Commit to be preserved, got %q", Commit)
	}
	if Date != "2026-01-01" {
		t.Errorf("expected ldflags Date to be preserved, got %q", Date)
	}
}
