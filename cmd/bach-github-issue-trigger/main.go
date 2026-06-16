package main

import (
	"context"
	"fmt"
	"os"

	"github.com/applauselab/bachkator/internal/githubissuetrigger"
	"github.com/applauselab/bachkator/pkg/triggerprotocol"
)

func main() {
	provider := githubissuetrigger.New(nil)
	if err := triggerprotocol.Serve(
		context.Background(),
		os.Stdin,
		os.Stdout,
		provider,
	); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
