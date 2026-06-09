package main

import (
	"context"
	"os"

	"github.com/applause/bachkator/internal/app"
)

var version = "dev"

func main() {
	application := app.New(version)
	if err := application.Execute(
		context.Background(),
		os.Args[1:],
		os.Stdout,
		os.Stderr,
	); err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}
