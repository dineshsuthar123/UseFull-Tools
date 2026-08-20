package render

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/local-first/what-changed/internal/compare"
	"github.com/local-first/what-changed/internal/snapshot"
)

func TestTextRendersRankAndLimit(t *testing.T) {
	now := time.Unix(1_700_000_600, 0).UTC()
	result := compare.Result{
		Baseline: compare.SnapshotRef{
			Label: "tests", CapturedAt: now.Add(-10 * time.Minute),
			Trigger: snapshot.Trigger{Command: []string{"go", "test", "./..."}},
		},
		Findings: []compare.Finding{
			{Score: 96, Category: "dependencies", Summary: "go.sum changed", Before: "old", After: "new", Why: "dependency changed"},
			{Score: 86, Category: "code", Summary: "main.go changed", Why: "source changed"},
		},
		Skipped: []string{"containers"},
	}
	var output bytes.Buffer
	if err := Text(&output, result, 1, now); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	for _, expected := range []string{"Since checkpoint \"tests\" (10 minutes ago after `go test ./...`)", "[96] Dependencies", "1 more change hidden", "Not compared: containers"} {
		if !strings.Contains(text, expected) {
			t.Errorf("output missing %q:\n%s", expected, text)
		}
	}
}
