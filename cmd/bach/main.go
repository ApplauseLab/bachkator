package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/applauselab/bachkator/internal/app"
	"github.com/applauselab/bachkator/internal/cli"
)

var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	application := app.New(version)
	err := application.Execute(
		ctx,
		os.Args[1:],
		os.Stdout,
		os.Stderr,
	)
	stop()
	if err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(exitCodeFor(err))
	}
}

func exitCodeFor(err error) int {
	if cli.IsUsageError(err) {
		return 2
	}
	return 1
}
