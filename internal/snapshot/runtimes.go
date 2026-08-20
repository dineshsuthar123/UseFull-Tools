package snapshot

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type runtimeProbe struct {
	name         string
	alternatives [][]string
}

var runtimeProbes = []runtimeProbe{
	{name: "Go", alternatives: [][]string{{"go", "version"}}},
	{name: "Node.js", alternatives: [][]string{{"node", "--version"}}},
	{name: "Python", alternatives: [][]string{{"python3", "--version"}, {"python", "--version"}}},
	{name: "Java", alternatives: [][]string{{"java", "-version"}}},
	{name: ".NET", alternatives: [][]string{{"dotnet", "--version"}}},
	{name: "Rust", alternatives: [][]string{{"rustc", "--version"}}},
	{name: "Docker", alternatives: [][]string{{"docker", "--version"}}},
}

func captureRuntimes(ctx context.Context, root string) (map[string]RuntimeState, bool, []Diagnostic) {
	result := make(map[string]RuntimeState)
	complete := true
	diagnostics := make([]Diagnostic, 0)
	for _, probe := range runtimeProbes {
		for _, command := range probe.alternatives {
			output, err := commandOutput(ctx, 1200*time.Millisecond, root, command[0], command[1:]...)
			if err == nil {
				if version := firstLine(output); version != "" {
					result[probe.name] = RuntimeState{Version: version}
				}
				break
			}
			if commandMissing(err) {
				continue
			}
			if errors.Is(err, context.DeadlineExceeded) {
				complete = false
				diagnostics = append(diagnostics, Diagnostic{
					Detector: "runtimes", Severity: "warning",
					Message: fmt.Sprintf("%s version check timed out", probe.name),
				})
				break
			}
			// An alternative executable may still work (for example python
			// when python3 is only a broken application alias on Windows).
			continue
		}
	}
	return result, complete, diagnostics
}
