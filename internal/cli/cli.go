package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/local-first/what-changed/internal/compare"
	"github.com/local-first/what-changed/internal/render"
	"github.com/local-first/what-changed/internal/snapshot"
	"github.com/local-first/what-changed/internal/store"
)

const Version = "0.1.0"

type App struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	Now    func() time.Time
}

func (app App) Run(ctx context.Context, args []string) int {
	app = app.defaults()
	if len(args) == 0 {
		return app.diff(ctx, nil)
	}
	switch args[0] {
	case "run":
		return app.runCommand(ctx, args[1:])
	case "mark":
		return app.mark(ctx, args[1:])
	case "diff":
		return app.diff(ctx, args[1:])
	case "list":
		return app.list(args[1:])
	case "version", "--version", "-version":
		fmt.Fprintf(app.Stdout, "what-changed %s\n", Version)
		return 0
	case "help", "--help", "-h":
		app.usage(app.Stdout)
		return 0
	default:
		fmt.Fprintf(app.Stderr, "unknown command %q\n\n", args[0])
		app.usage(app.Stderr)
		return 2
	}
}

func (app App) defaults() App {
	if app.Stdin == nil {
		app.Stdin = os.Stdin
	}
	if app.Stdout == nil {
		app.Stdout = os.Stdout
	}
	if app.Stderr == nil {
		app.Stderr = os.Stderr
	}
	if app.Now == nil {
		app.Now = time.Now
	}
	return app
}

func (app App) runCommand(ctx context.Context, args []string) int {
	flags := app.flagSet("run")
	name := flags.String("name", "last-success", "checkpoint name")
	rootFlag := flags.String("root", ".", "project root and command working directory")
	note := flags.String("note", "", "short context note")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	commandArgs := flags.Args()
	if len(commandArgs) == 0 {
		fmt.Fprintln(app.Stderr, "run requires a command after --")
		fmt.Fprintln(app.Stderr, "example: what-changed run -- go test ./...")
		return 2
	}
	root, err := projectRoot(*rootFlag)
	if err != nil {
		fmt.Fprintf(app.Stderr, "what-changed: %v\n", err)
		return 1
	}
	started := app.Now()
	command := exec.CommandContext(ctx, commandArgs[0], commandArgs[1:]...)
	command.Dir = root
	command.Stdin = app.Stdin
	command.Stdout = app.Stdout
	command.Stderr = app.Stderr
	err = command.Run()
	finished := app.Now()
	if err != nil {
		exitCode := commandExitCode(err)
		fmt.Fprintf(app.Stderr, "what-changed: command failed (exit %d); checkpoint not updated\n", exitCode)
		return exitCode
	}

	trigger := snapshot.Trigger{
		Kind: "successful-command", Command: append([]string(nil), commandArgs...), Note: *note,
		Duration: finished.Sub(started), ExitCode: 0, FinishedAt: finished.UTC(),
	}
	value, err := snapshot.Capture(ctx, snapshot.Options{Root: root, Label: *name, Trigger: trigger, Now: app.Now})
	if err != nil {
		fmt.Fprintf(app.Stderr, "what-changed: command succeeded, but checkpoint capture failed: %v\n", err)
		return 1
	}
	path, err := store.Save(root, value)
	if err != nil {
		fmt.Fprintf(app.Stderr, "what-changed: command succeeded, but checkpoint save failed: %v\n", err)
		return 1
	}
	app.printCaptured(value, path)
	return 0
}

func (app App) mark(ctx context.Context, args []string) int {
	flags := app.flagSet("mark")
	name := flags.String("name", "manual", "checkpoint name")
	rootFlag := flags.String("root", ".", "project root")
	note := flags.String("note", "", "short context note")
	jsonOutput := flags.Bool("json", false, "emit JSON")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(app.Stderr, "mark does not accept positional arguments: %s\n", strings.Join(flags.Args(), " "))
		return 2
	}
	root, err := projectRoot(*rootFlag)
	if err != nil {
		fmt.Fprintf(app.Stderr, "what-changed: %v\n", err)
		return 1
	}
	value, err := snapshot.Capture(ctx, snapshot.Options{
		Root: root, Label: *name, Trigger: snapshot.Trigger{Kind: "manual", Note: *note}, Now: app.Now,
	})
	if err != nil {
		fmt.Fprintf(app.Stderr, "what-changed: capture failed: %v\n", err)
		return 1
	}
	path, err := store.Save(root, value)
	if err != nil {
		fmt.Fprintf(app.Stderr, "what-changed: save failed: %v\n", err)
		return 1
	}
	if *jsonOutput {
		if err := render.JSON(app.Stdout, map[string]any{"path": path, "snapshot": value}); err != nil {
			fmt.Fprintf(app.Stderr, "what-changed: render failed: %v\n", err)
			return 1
		}
	} else {
		app.printCaptured(value, path)
	}
	return 0
}

func (app App) diff(ctx context.Context, args []string) int {
	flags := app.flagSet("diff")
	name := flags.String("name", "", "checkpoint name (default: latest)")
	rootFlag := flags.String("root", ".", "project root")
	jsonOutput := flags.Bool("json", false, "emit JSON")
	limit := flags.Int("limit", 20, "maximum text findings; 0 shows all")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(app.Stderr, "diff does not accept positional arguments: %s\n", strings.Join(flags.Args(), " "))
		return 2
	}
	if *limit < 0 {
		fmt.Fprintln(app.Stderr, "what-changed: --limit cannot be negative")
		return 2
	}
	root, err := projectRoot(*rootFlag)
	if err != nil {
		fmt.Fprintf(app.Stderr, "what-changed: %v\n", err)
		return 1
	}
	baseline, _, err := store.Load(root, *name)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			fmt.Fprintln(app.Stderr, "what-changed: no matching checkpoint; run `what-changed run -- <command>` or `what-changed mark` first")
		} else {
			fmt.Fprintf(app.Stderr, "what-changed: load checkpoint: %v\n", err)
		}
		return 1
	}
	current, err := snapshot.Capture(ctx, snapshot.Options{
		Root: root, Label: "current", Trigger: snapshot.Trigger{Kind: "comparison"}, Now: app.Now,
	})
	if err != nil {
		fmt.Fprintf(app.Stderr, "what-changed: current scan failed: %v\n", err)
		return 1
	}
	result := compare.Snapshots(baseline, current)
	if *jsonOutput {
		err = render.JSON(app.Stdout, result)
	} else {
		err = render.Text(app.Stdout, result, *limit, app.Now())
	}
	if err != nil {
		fmt.Fprintf(app.Stderr, "what-changed: render failed: %v\n", err)
		return 1
	}
	return 0
}

func (app App) list(args []string) int {
	flags := app.flagSet("list")
	rootFlag := flags.String("root", ".", "project root")
	jsonOutput := flags.Bool("json", false, "emit JSON")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(app.Stderr, "list does not accept positional arguments: %s\n", strings.Join(flags.Args(), " "))
		return 2
	}
	root, err := projectRoot(*rootFlag)
	if err != nil {
		fmt.Fprintf(app.Stderr, "what-changed: %v\n", err)
		return 1
	}
	entries, err := store.List(root)
	if err != nil {
		fmt.Fprintf(app.Stderr, "what-changed: list checkpoints: %v\n", err)
		return 1
	}
	if *jsonOutput {
		if err := render.JSON(app.Stdout, entries); err != nil {
			fmt.Fprintf(app.Stderr, "what-changed: render failed: %v\n", err)
			return 1
		}
		return 0
	}
	if len(entries) == 0 {
		fmt.Fprintln(app.Stdout, "No checkpoints recorded.")
		return 0
	}
	for _, entry := range entries {
		trigger := entry.Snapshot.Trigger.Kind
		if len(entry.Snapshot.Trigger.Command) > 0 {
			trigger = strings.Join(entry.Snapshot.Trigger.Command, " ")
		}
		fmt.Fprintf(app.Stdout, "%-20s %s  %s\n", entry.Snapshot.Label, entry.Snapshot.CapturedAt.Local().Format(time.RFC3339), trigger)
	}
	return 0
}

func (app App) printCaptured(value *snapshot.Snapshot, path string) {
	relative := path
	if candidate, err := filepath.Rel(value.Root, path); err == nil {
		relative = filepath.ToSlash(candidate)
	}
	warnings := 0
	for _, diagnostic := range value.Diagnostics {
		if diagnostic.Severity == "warning" {
			warnings++
		}
	}
	fmt.Fprintf(app.Stdout, "Checkpoint %q recorded: %d files, %d runtimes, %d listening ports (%s).\n",
		value.Label, value.Stats.FilesHashed, len(value.Runtimes), len(value.Ports), relative)
	if warnings > 0 {
		fmt.Fprintf(app.Stdout, "%d detector warning%s stored with the checkpoint.\n", warnings, plural(warnings))
	}
}

func (app App) flagSet(name string) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(app.Stderr)
	return flags
}

func (app App) usage(writer io.Writer) {
	fmt.Fprintln(writer, "WhatChanged — show what changed since a command last worked")
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "Usage:")
	fmt.Fprintln(writer, "  what-changed run [--name NAME] -- COMMAND [ARG...]  Run and checkpoint on success")
	fmt.Fprintln(writer, "  what-changed mark [--name NAME] [--note TEXT]       Record a manual checkpoint")
	fmt.Fprintln(writer, "  what-changed diff [--name NAME] [--json]            Rank changes since a checkpoint")
	fmt.Fprintln(writer, "  what-changed list [--json]                           List local checkpoints")
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "All checkpoint data stays under .what-changed/ in the project root.")
}

func projectRoot(value string) (string, error) {
	root, err := filepath.Abs(value)
	if err != nil {
		return "", fmt.Errorf("resolve project root: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return "", fmt.Errorf("inspect project root: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("project root is not a directory: %s", root)
	}
	return root, nil
}

func commandExitCode(err error) int {
	if errors.Is(err, context.Canceled) {
		return 130
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		if code := exitError.ExitCode(); code >= 0 {
			return code
		}
	}
	if errors.Is(err, exec.ErrNotFound) {
		return 127
	}
	return 1
}

func plural(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}
