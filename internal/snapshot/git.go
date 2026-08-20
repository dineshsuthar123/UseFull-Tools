package snapshot

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func captureGit(ctx context.Context, root string) (*GitState, bool, []Diagnostic) {
	inside, err := commandOutput(ctx, 1200*time.Millisecond, root, "git", "rev-parse", "--is-inside-work-tree")
	if err != nil || strings.TrimSpace(inside) != "true" {
		message := "not a Git worktree"
		if commandMissing(err) {
			message = "git is unavailable"
		}
		return nil, false, []Diagnostic{{Detector: "git", Severity: "info", Message: message}}
	}
	branch, err := commandOutput(ctx, 1200*time.Millisecond, root, "git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return nil, false, []Diagnostic{{Detector: "git", Severity: "warning", Message: fmt.Sprintf("read branch: %v", err)}}
	}
	commit, err := commandOutput(ctx, 1200*time.Millisecond, root, "git", "rev-parse", "HEAD")
	if err != nil {
		return nil, false, []Diagnostic{{Detector: "git", Severity: "warning", Message: fmt.Sprintf("read commit: %v", err)}}
	}
	status, err := commandOutput(ctx, 2*time.Second, root, "git", "status", "--porcelain=v1", "-uno")
	if err != nil {
		return nil, false, []Diagnostic{{Detector: "git", Severity: "warning", Message: fmt.Sprintf("read status: %v", err)}}
	}
	dirty := 0
	if status != "" {
		dirty = len(strings.Split(strings.ReplaceAll(status, "\r\n", "\n"), "\n"))
	}
	return &GitState{Branch: firstLine(branch), Commit: firstLine(commit), Dirty: dirty}, true, nil
}
