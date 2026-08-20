package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/local-first/what-changed/internal/snapshot"
)

func TestSaveAndLoadLatestAndNamedCheckpoints(t *testing.T) {
	root := t.TempDir()
	first := minimalSnapshot(root, "tests", time.Unix(100, 0))
	second := minimalSnapshot(root, "build", time.Unix(200, 0))
	third := minimalSnapshot(root, "tests", time.Unix(300, 0))
	paths := make(map[string]struct{})
	for _, value := range []*snapshot.Snapshot{first, second, third} {
		path, err := Save(root, value)
		if err != nil {
			t.Fatal(err)
		}
		if _, duplicate := paths[path]; duplicate {
			t.Fatalf("checkpoint path reused: %s", path)
		}
		paths[path] = struct{}{}
	}

	latest, _, err := Load(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if latest.Label != "tests" || !latest.CapturedAt.Equal(third.CapturedAt) {
		t.Fatalf("latest=%#v, want third checkpoint", latest)
	}
	named, _, err := Load(root, "build")
	if err != nil {
		t.Fatal(err)
	}
	if !named.CapturedAt.Equal(second.CapturedAt) {
		t.Fatalf("named checkpoint time=%v, want %v", named.CapturedAt, second.CapturedAt)
	}
	entries, err := List(root)
	if err != nil || len(entries) != 3 {
		t.Fatalf("List() entries=%d err=%v", len(entries), err)
	}
}

func TestListIgnoresCorruptCheckpoint(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, directoryName)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "broken.json"), []byte("not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	entries, err := List(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("got %d entries, want corrupt file ignored", len(entries))
	}
	_, _, err = Load(root, "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Load error=%v, want ErrNotFound", err)
	}
}

func minimalSnapshot(root, label string, capturedAt time.Time) *snapshot.Snapshot {
	return &snapshot.Snapshot{
		SchemaVersion: snapshot.SchemaVersion, Label: label, CapturedAt: capturedAt.UTC(), Root: root,
		Files: map[string]snapshot.FileState{}, Environment: map[string]snapshot.EnvState{},
		Runtimes: map[string]snapshot.RuntimeState{}, Ports: map[string]snapshot.PortState{},
		Containers: map[string]snapshot.ContainerState{}, Complete: map[string]bool{},
	}
}
