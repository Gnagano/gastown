package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/steveyegge/gastown/internal/events"
)

// beadFirstExec is replaceable in tests. Keeping the policy in the runtime,
// rather than in a site-specific shell script, makes it usable by every agent
// provider that implements PreToolUse hooks.
var beadFirstExec = func(dir, name string, args ...string) ([]byte, error) {
	c := exec.Command(name, args...)
	c.Dir = dir
	return c.Output()
}

var tapGuardBeadFirstCmd = &cobra.Command{
	Use:   "bead-first",
	Short: "Require bead-first workflow for repository mutations",
	Long: `Gate repository mutations, pull requests, and deploys.

The operation is allowed only from a primed Polecat session with a currently
hooked bead. Set GT_BEAD_FIRST_OVERRIDE to a non-empty justification for an
audited emergency override. Read-only commands and Gas Town/Beads workflow
commands are allowed directly. The guard reads standard PreToolUse JSON on
stdin and exits 2 when an operation is denied.`,
	RunE: runTapGuardBeadFirst,
}

type beadFirstHookInput struct {
	ToolName  string `json:"tool_name"`
	ToolInput struct {
		Command string `json:"command"`
	} `json:"tool_input"`
}

type beadFirstIssue struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func runTapGuardBeadFirst(cmd *cobra.Command, args []string) error {
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil // hook protocol errors fail open, matching the other guards
	}
	var hook beadFirstHookInput
	if json.Unmarshal(input, &hook) != nil || !isBeadFirstMutation(hook.ToolName, hook.ToolInput.Command) {
		return nil
	}

	actor := strings.TrimSpace(os.Getenv("GT_ROLE"))
	if reason := beadFirstOverrideReason(); reason != "" {
		_ = events.LogAudit("bead_first_override", actor, map[string]interface{}{
			"reason": reason, "tool": hook.ToolName, "command": hook.ToolInput.Command,
		})
		return nil
	}

	cwd, _ := os.Getwd()
	reason := beadFirstDenialReason(cwd, actor)
	if reason == "" {
		return nil
	}
	_ = events.LogAudit("bead_first_denied", actor, map[string]interface{}{
		"reason": reason, "tool": hook.ToolName, "command": hook.ToolInput.Command,
	})
	fmt.Fprintf(os.Stderr, "BEAD-FIRST GATE: operation blocked: %s\n", reason)
	fmt.Fprintln(os.Stderr, "Run gt prime, work from an assigned Polecat with a hooked bead, or set GT_BEAD_FIRST_OVERRIDE to an emergency justification.")
	return NewSilentExit(2)
}

func beadFirstOverrideReason() string {
	return strings.TrimSpace(os.Getenv("GT_BEAD_FIRST_OVERRIDE"))
}

func isBeadFirstMutation(tool, command string) bool {
	switch strings.ToLower(strings.TrimSpace(tool)) {
	case "write", "edit", "multiedit", "notebookedit":
		return true
	}
	lower := strings.ToLower(strings.TrimSpace(command))
	if lower == "" || isBeadFirstDirectCommand(lower) {
		return false
	}
	markers := []string{
		"git commit", "git push", "git merge", "git rebase", "git tag", "git checkout -b", "git switch -c",
		"gh pr create", "gh pr merge", "gh release create", "vercel deploy", "firebase deploy", "gcloud deploy",
		"kubectl apply", "helm upgrade", "terraform apply", "pulumi up", "npm publish", "docker push",
	}
	for _, marker := range markers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

// Workflow and inspection commands form the deliberately narrow direct
// allowlist. They do not mutate source code or external deployment state.
func isBeadFirstDirectCommand(command string) bool {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return true
	}
	if fields[0] == "bd" || fields[0] == "gt" {
		return true
	}
	return false
}

func beadFirstDenialReason(cwd, role string) string {
	parts := strings.Split(role, "/")
	if len(parts) != 3 || parts[1] != "polecats" || parts[2] == "" {
		return "mutations must be delegated to a Polecat"
	}
	if os.Getenv("GT_SESSION") == "" {
		return "Polecat session identity is missing"
	}
	if !hasFreshPrimeMarker(cwd) {
		return "session has not been primed"
	}
	out, err := beadFirstExec(cwd, "bd", "list", "--status=hooked", "--assignee="+role, "--json")
	if err != nil {
		return "unable to verify hooked bead"
	}
	var issues []beadFirstIssue
	if json.Unmarshal(out, &issues) != nil || len(issues) == 0 {
		return "Polecat has no currently hooked bead"
	}
	for _, issue := range issues {
		if issue.ID != "" && issue.Status == "hooked" {
			return ""
		}
	}
	return "Polecat hook is stale"
}

func hasFreshPrimeMarker(cwd string) bool {
	for dir := cwd; ; dir = filepath.Dir(dir) {
		path := filepath.Join(dir, ".runtime", "session_id")
		if info, err := os.Stat(path); err == nil && !info.IsDir() && time.Since(info.ModTime()) < 24*time.Hour {
			return true
		}
		next := filepath.Dir(dir)
		if next == dir {
			return false
		}
	}
}
