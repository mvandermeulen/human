package forge

import "testing"

func TestIsForgeKind(t *testing.T) {
	tests := map[string]bool{
		"github":      true,
		"gitlab":      false, // not implemented yet (stubbed as a follow-up)
		"jira":        false,
		"linear":      false,
		"shortcut":    false,
		"clickup":     false,
		"azuredevops": false,
		"":            false,
	}
	for kind, want := range tests {
		if got := IsForgeKind(kind); got != want {
			t.Errorf("IsForgeKind(%q) = %v, want %v", kind, got, want)
		}
	}
}
