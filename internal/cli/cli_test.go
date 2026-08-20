package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestRunDoesNotCheckpointFailedCommand(t *testing.T) {
	if os.Getenv("WHAT_CHANGED_HELPER") == "1" {
		separator := 0
		for index, argument := range os.Args {
			if argument == "--" {
				separator = index
			}
		}
		code, _ := strconv.Atoi(os.Args[separator+1])
		os.Exit(code)
	}
	root := t.TempDir()
	t.Setenv("WHAT_CHANGED_HELPER", "1")
	var stdout, stderr bytes.Buffer
	app := App{Stdout: &stdout, Stderr: &stderr}
	code := app.Run(context.Background(), []string{
		"run", "--root", root, "--", os.Args[0], "-test.run=TestRunDoesNotCheckpointFailedCommand", "--", "7",
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
