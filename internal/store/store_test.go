package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dineshsuthar123/UseFull-Tools/internal/snapshot"
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

func TestLoadByCommandIDMaintainsIndependentBaselines(t *testing.T) {
	root := t.TempDir()
	goOld := minimalSnapshot(root, "go-test-old", time.Unix(100, 0))
	goOld.CommandID = "go-test-aaaaaaaa"
	npm := minimalSnapshot(root, "npm-test", time.Unix(200, 0))
	npm.CommandID = "npm-test-bbbbbbbb"
	goNew := minimalSnapshot(root, "go-test-new", time.Unix(300, 0))
	goNew.CommandID = goOld.CommandID
	for _, value := range []*snapshot.Snapshot{goOld, npm, goNew} {
		if _, err := Save(root, value); err != nil {
			t.Fatal(err)
		}
	}
	loadedGo, _, err := LoadByCommandID(root, goOld.CommandID)
	if err != nil || !loadedGo.CapturedAt.Equal(goNew.CapturedAt) {
		t.Fatalf("Go baseline=%#v err=%v, want newest Go snapshot", loadedGo, err)
	}
	loadedNPM, _, err := LoadByCommandID(root, npm.CommandID)
	if err != nil || !loadedNPM.CapturedAt.Equal(npm.CapturedAt) {
		t.Fatalf("npm baseline=%#v err=%v", loadedNPM, err)
	}
}

func TestReadUpgradesV1AndRemovesLegacyPlaintext(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, directoryName)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	legacy := map[string]any{
		"schemaVersion": 1, "label": "last-success", "capturedAt": "2024-01-01T00:00:00Z", "root": root,
		"trigger": map[string]any{"kind": "successful-command", "command": []string{"go", "test", "./..."}},
		"files":   map[string]any{"main.go": map[string]any{"sha256": "abc", "size": 10, "kind": "source"}},
		"environment": map[string]any{
			"MY_INTERNAL_VALUE": map[string]any{"sha256": "one", "value": "must-not-survive"},
			"NODE_ENV":          map[string]any{"sha256": "two", "value": "test"},
		},
		"runtimes": map[string]any{}, "ports": map[string]any{}, "containers": map[string]any{},
		"complete": map[string]any{"files": true, "environment": true}, "stats": map[string]any{},
	}
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "legacy.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	value, err := read(path)
	if err != nil {
		t.Fatal(err)
	}
	if value.SchemaVersion != snapshot.SchemaVersion || value.CommandID == "" || !value.Files["main.go"].Tracked {
		t.Fatalf("legacy metadata not upgraded: %#v", value)
	}
	if state := value.Environment["MY_INTERNAL_VALUE"]; state.Value != "" || state.Sensitivity != "unknown" {
		t.Fatalf("legacy plaintext survived upgrade: %#v", state)
	}
	if state := value.Environment["NODE_ENV"]; state.Value != "test" || state.Sensitivity != "safe" {
		t.Fatalf("safe legacy value not retained: %#v", state)
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
