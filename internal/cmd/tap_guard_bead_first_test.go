package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsBeadFirstMutation(t *testing.T) {
	tests := []struct {
		tool, command string
		want          bool
	}{
		{"Write", "", true},
		{"Bash", "git commit -m test", true},
		{"Bash", "gh pr create --fill", true},
		{"Bash", "firebase deploy", true},
		{"Bash", "git status", false},
		{"Bash", "bd create --title=x", false},
		{"Bash", "gt sling gt-abc rig", false},
	}
	for _, tt := range tests {
		if got := isBeadFirstMutation(tt.tool, tt.command); got != tt.want {
			t.Errorf("isBeadFirstMutation(%q, %q)=%v, want %v", tt.tool, tt.command, got, tt.want)
		}
	}
}

func TestBeadFirstDenialReason(t *testing.T) {
	original := beadFirstExec
	t.Cleanup(func() { beadFirstExec = original })
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".runtime"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".runtime", "session_id"), []byte("session"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GT_SESSION", "gt-demo-toast")

	if got := beadFirstDenialReason(dir, "demo/mayor"); got == "" {
		t.Fatal("mayor unexpectedly allowed")
	}
	beadFirstExec = func(string, string, ...string) ([]byte, error) {
		return []byte(`[{"id":"gt-abc","status":"hooked"}]`), nil
	}
	if got := beadFirstDenialReason(dir, "demo/polecats/toast"); got != "" {
		t.Fatalf("valid hook denied: %s", got)
	}
	beadFirstExec = func(string, string, ...string) ([]byte, error) { return []byte(`[]`), nil }
	if got := beadFirstDenialReason(dir, "demo/polecats/toast"); got == "" {
		t.Fatal("missing hook unexpectedly allowed")
	}
	beadFirstExec = func(string, string, ...string) ([]byte, error) {
		return []byte(`[{"id":"gt-abc","status":"open"}]`), nil
	}
	if got := beadFirstDenialReason(dir, "demo/polecats/toast"); got != "Polecat hook is stale" {
		t.Fatalf("got %q", got)
	}
}

func TestBeadFirstOverrideIsExplicit(t *testing.T) {
	t.Setenv("GT_BEAD_FIRST_OVERRIDE", "incident-42")
	if got := beadFirstOverrideReason(); got != "incident-42" {
		t.Fatalf("override reason = %q", got)
	}
	t.Setenv("GT_BEAD_FIRST_OVERRIDE", "   ")
	if got := beadFirstOverrideReason(); got != "" {
		t.Fatalf("blank override unexpectedly accepted: %q", got)
	}
}
