package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/dineshsuthar123/UseFull-Tools/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	app := cli.App{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr}
	os.Exit(app.Run(ctx, os.Args[1:]))
}
