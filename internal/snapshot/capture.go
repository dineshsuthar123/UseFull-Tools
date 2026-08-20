package snapshot

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"
)

type Options struct {
	Root    string
	Label   string
	Trigger Trigger
	Now     func() time.Time
}

type detectorResult[T any] struct {
	value       T
	complete    bool
	diagnostics []Diagnostic
}

func Capture(ctx context.Context, opts Options) (*Snapshot, error) {
	root, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, fmt.Errorf("resolve project root: %w", err)
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}

	var files detectorResult[map[string]FileState]
	var runtimes detectorResult[map[string]RuntimeState]
	var git detectorResult[*GitState]
	var ports detectorResult[map[string]PortState]
	var containers detectorResult[map[string]ContainerState]
	var fileStats Stats

	var wg sync.WaitGroup
	wg.Add(5)
	go func() {
		defer wg.Done()
		files.value, fileStats, files.complete, files.diagnostics = captureFiles(ctx, root)
	}()
	go func() {
		defer wg.Done()
		runtimes.value, runtimes.complete, runtimes.diagnostics = captureRuntimes(ctx, root)
	}()
	go func() {
		defer wg.Done()
		git.value, git.complete, git.diagnostics = captureGit(ctx, root)
	}()
	go func() {
		defer wg.Done()
		ports.value, ports.complete, ports.diagnostics = capturePorts(ctx, root)
	}()
	go func() {
		defer wg.Done()
		containers.value, containers.complete, containers.diagnostics = captureContainers(ctx, root)
	}()

	environment := captureEnvironment()
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("capture interrupted: %w", err)
	}

	diagnostics := make([]Diagnostic, 0,
		len(files.diagnostics)+len(runtimes.diagnostics)+len(git.diagnostics)+
			len(ports.diagnostics)+len(containers.diagnostics))
	diagnostics = append(diagnostics, files.diagnostics...)
	diagnostics = append(diagnostics, runtimes.diagnostics...)
	diagnostics = append(diagnostics, git.diagnostics...)
	diagnostics = append(diagnostics, ports.diagnostics...)
	diagnostics = append(diagnostics, containers.diagnostics...)

	return &Snapshot{
		SchemaVersion: SchemaVersion,
		Label:         opts.Label,
		CapturedAt:    now().UTC(),
		Root:          filepath.Clean(root),
		Trigger:       opts.Trigger,
		Files:         nonNil(files.value),
		Environment:   environment,
		Runtimes:      nonNil(runtimes.value),
		Git:           git.value,
		Ports:         nonNil(ports.value),
		Containers:    nonNil(containers.value),
		Complete: map[string]bool{
			"files":       files.complete,
			"environment": true,
			"runtimes":    runtimes.complete,
			"git":         git.complete,
			"ports":       ports.complete,
			"containers":  containers.complete,
		},
		Stats:       fileStats,
		Diagnostics: diagnostics,
	}, nil
}

func nonNil[T any](value map[string]T) map[string]T {
	if value == nil {
		return map[string]T{}
	}
	return value
}
