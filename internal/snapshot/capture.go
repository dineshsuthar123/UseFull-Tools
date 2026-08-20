package snapshot

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"
)

type Options struct {
	Root      string
	Label     string
	CommandID string
	Trigger   Trigger
	Previous  *Snapshot
	Now       func() time.Time
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
		defer recoverDetector("files", &files.complete, &files.diagnostics)
		var previous map[string]FileState
		if opts.Previous != nil {
			previous = opts.Previous.Files
		}
		files.value, fileStats, files.complete, files.diagnostics = captureFiles(ctx, root, previous)
	}()
	go func() {
		defer wg.Done()
		defer recoverDetector("runtimes", &runtimes.complete, &runtimes.diagnostics)
		runtimes.value, runtimes.complete, runtimes.diagnostics = captureRuntimes(ctx, root)
	}()
	go func() {
		defer wg.Done()
		defer recoverDetector("git", &git.complete, &git.diagnostics)
		git.value, git.complete, git.diagnostics = captureGit(ctx, root)
	}()
	go func() {
		defer wg.Done()
		defer recoverDetector("ports", &ports.complete, &ports.diagnostics)
		ports.value, ports.complete, ports.diagnostics = capturePorts(ctx, root)
	}()
	go func() {
		defer wg.Done()
		defer recoverDetector("containers", &containers.complete, &containers.diagnostics)
		containers.value, containers.complete, containers.diagnostics = captureContainers(ctx, root)
	}()

	environment := captureEnvironment()
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("capture interrupted: %w", err)
	}
	project, projectComplete, projectDiagnostics := captureProjectContextSafe(ctx, root, nonNil(files.value))
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("project-context capture interrupted: %w", err)
	}

	diagnostics := make([]Diagnostic, 0,
		len(files.diagnostics)+len(runtimes.diagnostics)+len(git.diagnostics)+
			len(ports.diagnostics)+len(containers.diagnostics)+len(projectDiagnostics))
	diagnostics = append(diagnostics, files.diagnostics...)
	diagnostics = append(diagnostics, runtimes.diagnostics...)
	diagnostics = append(diagnostics, git.diagnostics...)
	diagnostics = append(diagnostics, ports.diagnostics...)
	diagnostics = append(diagnostics, containers.diagnostics...)
	diagnostics = append(diagnostics, projectDiagnostics...)

	return &Snapshot{
		SchemaVersion: SchemaVersion,
		Label:         opts.Label,
		CapturedAt:    now().UTC(),
		Root:          filepath.Clean(root),
		CommandID:     opts.CommandID,
		Trigger:       opts.Trigger,
		Files:         nonNil(files.value),
		Environment:   environment,
		Runtimes:      nonNil(runtimes.value),
		Git:           git.value,
		Ports:         nonNil(ports.value),
		Containers:    nonNil(containers.value),
		Project:       project,
		Complete: map[string]bool{
			"files":          files.complete,
			"environment":    true,
			"runtimes":       runtimes.complete,
			"git":            git.complete,
			"ports":          ports.complete,
			"containers":     containers.complete,
			"projectContext": projectComplete,
		},
		Stats:       fileStats,
		Diagnostics: diagnostics,
	}, nil
}

func recoverDetector(name string, complete *bool, diagnostics *[]Diagnostic) {
	if recovered := recover(); recovered != nil {
		*complete = false
		*diagnostics = append(*diagnostics, Diagnostic{
			Detector: name, Severity: "warning", Message: fmt.Sprintf("internal error: %v", recovered),
		})
	}
}

func captureProjectContextSafe(ctx context.Context, root string, files map[string]FileState) (
	project ProjectContext, complete bool, diagnostics []Diagnostic,
) {
	defer func() {
		if recovered := recover(); recovered != nil {
			project = ProjectContext{}
			complete = false
			diagnostics = []Diagnostic{{
				Detector: "project-context", Severity: "warning", Message: fmt.Sprintf("internal error: %v", recovered),
			}}
		}
	}()
	return captureProjectContext(ctx, root, files)
}

func nonNil[T any](value map[string]T) map[string]T {
	if value == nil {
		return map[string]T{}
	}
	return value
}
