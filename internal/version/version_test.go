package version

import (
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
