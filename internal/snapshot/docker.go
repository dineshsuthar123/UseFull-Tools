package snapshot

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func captureContainers(ctx context.Context, root string) (map[string]ContainerState, bool, []Diagnostic) {
	output, err := commandOutput(ctx, 1800*time.Millisecond, root, "docker", "ps", "--format", "{{.Names}}\t{{.Image}}\t{{.State}}")
	if err != nil {
		message := "docker is unavailable"
		if !commandMissing(err) {
			message = fmt.Sprintf("docker state unavailable: %v", err)
		}
		return nil, false, []Diagnostic{{Detector: "containers", Severity: "info", Message: message}}
	}
	result := make(map[string]ContainerState)
	for _, line := range strings.Split(strings.ReplaceAll(output, "\r\n", "\n"), "\n") {
		parts := strings.Split(line, "\t")
		if len(parts) != 3 || strings.TrimSpace(parts[0]) == "" {
			continue
		}
		result[strings.TrimSpace(parts[0])] = ContainerState{
			Image: strings.TrimSpace(parts[1]), State: strings.TrimSpace(parts[2]),
		}
	}
	return result, true, nil
}
