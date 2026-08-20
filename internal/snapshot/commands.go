package snapshot

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"
)

func commandOutput(ctx context.Context, timeout time.Duration, dir string, name string, args ...string) (string, error) {
	commandCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	command := exec.CommandContext(commandCtx, name, args...)
	command.Dir = dir
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	err := command.Run()
	if commandCtx.Err() != nil {
		return strings.TrimSpace(output.String()), commandCtx.Err()
	}
	return strings.TrimSpace(output.String()), err
}

func commandMissing(err error) bool {
	var execErr *exec.Error
	return errors.As(err, &execErr) && errors.Is(execErr.Err, exec.ErrNotFound)
}

func firstLine(output string) string {
	for _, line := range strings.Split(strings.ReplaceAll(output, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			if len(line) > 300 {
				return line[:300]
			}
			return line
		}
	}
	return ""
}
