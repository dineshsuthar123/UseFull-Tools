package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dineshsuthar123/UseFull-Tools/internal/commandid"
	"github.com/dineshsuthar123/UseFull-Tools/internal/store"
)

func TestCommandHelper(t *testing.T) {
	if os.Getenv("WHAT_CHANGED_HELPER") == "1" {
		code, _ := strconv.Atoi(os.Getenv("WHAT_CHANGED_HELPER_EXIT"))
		os.Exit(code)
	}
}

func TestRunDoesNotCheckpointFailedCommand(t *testing.T) {
	root := t.TempDir()
	t.Setenv("WHAT_CHANGED_HELPER", "1")
	t.Setenv("WHAT_CHANGED_HELPER_EXIT", "7")
	var stdout, stderr bytes.Buffer
	app := App{Stdout: &stdout, Stderr: &stderr}
	code := app.Run(context.Background(), []string{
		"run", "--root", root, "--", os.Args[0], "-test.run=TestCommandHelper", "--", "failure-probe",
	})
	if code != 7 {
		t.Fatalf("exit code=%d, want 7; stderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".what-changed")); !os.IsNotExist(err) {
		t.Fatalf("checkpoint directory exists after failure: %v", err)
	}
	if !strings.Contains(stderr.String(), "checkpoint not updated") {
		t.Fatalf("missing failure explanation: %s", stderr.String())
	}
}

func TestPerCommandBaselinesAndFailedRunPreservesPriorSuccess(t *testing.T) {
	root := t.TempDir()
	t.Setenv("WHAT_CHANGED_HELPER", "1")
	t.Setenv("WHAT_CHANGED_HELPER_EXIT", "0")
	commandA := []string{os.Args[0], "-test.run=TestCommandHelper", "--", "command-a"}
	commandB := []string{os.Args[0], "-test.run=TestCommandHelper", "--", "command-b"}
	for _, command := range [][]string{commandA, commandB} {
		var stdout, stderr bytes.Buffer
		args := append([]string{"run", "--root", root, "--"}, command...)
		if code := (App{Stdout: &stdout, Stderr: &stderr}).Run(context.Background(), args); code != 0 {
			t.Fatalf("successful command returned %d: %s", code, stderr.String())
		}
	}
	entries, err := store.List(root)
	if err != nil || len(entries) != 2 {
		t.Fatalf("entries=%d err=%v, want two command baselines", len(entries), err)
	}
	idA, _, _ := commandid.Identity(root, commandA)
	idB, _, _ := commandid.Identity(root, commandB)
	if idA == idB {
		t.Fatal("different commands share an identity")
	}
	baselineA, _, err := store.LoadByCommandID(root, idA)
	if err != nil {
		t.Fatal(err)
	}
	originalTime := baselineA.CapturedAt

	t.Setenv("WHAT_CHANGED_HELPER_EXIT", "7")
	var stdout, stderr bytes.Buffer
	args := append([]string{"run", "--root", root, "--"}, commandA...)
	if code := (App{Stdout: &stdout, Stderr: &stderr}).Run(context.Background(), args); code != 7 {
		t.Fatalf("failed rerun code=%d, want 7", code)
	}
	preserved, _, err := store.LoadByCommandID(root, idA)
	if err != nil || !preserved.CapturedAt.Equal(originalTime) {
		t.Fatalf("failed run advanced checkpoint: before=%v after=%v err=%v", originalTime, preserved.CapturedAt, err)
	}

	t.Setenv("WHAT_CHANGED_HELPER_EXIT", "0")
	stdout.Reset()
	stderr.Reset()
	diffArgs := append([]string{"diff", "--root", root, "--"}, commandB...)
	if code := (App{Stdout: &stdout, Stderr: &stderr, Now: func() time.Time { return time.Now() }}).Run(context.Background(), diffArgs); code != 0 {
		t.Fatalf("command-specific diff code=%d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "command-b") {
		t.Fatalf("diff did not select command B baseline:\n%s", stdout.String())
	}
}

func TestNoArgumentsExplainsMissingCheckpoint(t *testing.T) {
	root := t.TempDir()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(previous)
	var stderr bytes.Buffer
	app := App{Stdout: &bytes.Buffer{}, Stderr: &stderr}
	if code := app.Run(context.Background(), nil); code != 1 {
		t.Fatalf("code=%d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "mark` first") {
		t.Fatalf("unexpected error: %s", stderr.String())
	}
}
