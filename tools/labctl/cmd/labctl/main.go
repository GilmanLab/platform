package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/gilmanlab/platform/tools/labctl/internal/cli"
)

// version is set by release builds with -ldflags "-X main.version=<version>".
var version = "dev"

func main() {
	os.Exit(run())
}

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return cli.Run(ctx, os.Args[1:], cli.Options{
		Version:   version,
		LookupEnv: os.LookupEnv,
		Stdin:     os.Stdin,
		Stdout:    os.Stdout,
		Stderr:    os.Stderr,
	})
}
