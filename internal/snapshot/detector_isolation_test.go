package snapshot

import (
	"context"
	"testing"
)

func TestMissingDockerIsReportedWithoutError(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	containers, complete, diagnostics := captureContainers(context.Background(), t.TempDir())
	if complete || len(containers) != 0 || len(diagnostics) != 1 || diagnostics[0].Detector != "containers" {
		t.Fatalf("unexpected missing Docker result: containers=%#v complete=%v diagnostics=%#v", containers, complete, diagnostics)
	}
}

func TestMissingGitIsReportedWithoutError(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	git, complete, diagnostics := captureGit(context.Background(), t.TempDir())
	if complete || git != nil || len(diagnostics) != 1 || diagnostics[0].Detector != "git" {
		t.Fatalf("unexpected missing Git result: git=%#v complete=%v diagnostics=%#v", git, complete, diagnostics)
	}
}

func TestOptionalDetectorFailuresDoNotBreakSnapshot(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "main.go", "package main\n")
	t.Setenv("PATH", t.TempDir())
	value, err := Capture(context.Background(), Options{Root: root, Label: "isolated"})
	if err != nil {
		t.Fatalf("optional detector failures aborted capture: %v", err)
	}
	if !value.Complete["files"] || value.Files["main.go"].SHA256 == "" {
		t.Fatalf("core file result was lost: %#v", value)
	}
	if len(value.Diagnostics) < 2 {
		t.Fatalf("optional failures were not disclosed: %#v", value.Diagnostics)
	}
}
