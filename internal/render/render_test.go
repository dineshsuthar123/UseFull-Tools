package render

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/dineshsuthar123/UseFull-Tools/internal/compare"
	"github.com/dineshsuthar123/UseFull-Tools/internal/snapshot"
)

func TestDefaultTextIsConciseAndCommandSpecific(t *testing.T) {
	now := time.Unix(1_700_000_600, 0).UTC()
	result := renderFixture(now)
	var output bytes.Buffer
	if err := Text(&output, result, TextOptions{Limit: 1, Now: now}); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	for _, expected := range []string{
		"Since `go test ./...` last passed 10 minutes ago:",
		"Likely relevant changes:", "[96] go.sum changed", "1 more change hidden", "2 meaningful changes found",
	} {
		if !strings.Contains(text, expected) {
			t.Errorf("output missing %q:\n%s", expected, text)
		}
	}
	if strings.Contains(text, "sha256:") || strings.Contains(text, "Fact:") {
		t.Fatalf("default output contains verbose facts:\n%s", text)
	}
}

func TestVerboseTextSeparatesFactsFromRankingExplanation(t *testing.T) {
	now := time.Unix(1_700_000_600, 0).UTC()
	result := renderFixture(now)
	result.Diagnostics = []snapshot.Diagnostic{{Detector: "containers", Severity: "info", Message: "docker is unavailable"}}
	var output bytes.Buffer
	if err := Text(&output, result, TextOptions{Limit: 0, Verbose: true, Now: now}); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	for _, expected := range []string{"Fact: old -> new", "Why this is ranked highly:", "Containers detector: docker is unavailable"} {
		if !strings.Contains(text, expected) {
			t.Errorf("verbose output missing %q:\n%s", expected, text)
		}
	}
	if strings.Contains(strings.ToLower(text), "caused the failure") {
		t.Fatalf("output claimed causality:\n%s", text)
	}
}

func renderFixture(now time.Time) compare.Result {
	return compare.Result{
		Baseline: compare.SnapshotRef{
			Label: "go-test-12345678", CapturedAt: now.Add(-10 * time.Minute),
			Trigger: snapshot.Trigger{Kind: "successful-command", Command: []string{"go", "test", "./..."}},
		},
		Findings: []compare.Finding{
			{Score: 96, Category: "dependencies", Summary: "go.sum changed", Before: "old", After: "new", Why: "dependency metadata changed"},
			{Score: 86, Category: "code", Summary: "main.go changed", Before: "sha256:old", After: "sha256:new", Why: "source changed"},
		},
	}
}
