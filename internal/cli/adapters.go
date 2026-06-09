package cli

import (
	"context"
	"io"
)

type commandContext struct {
	context context.Context
	project *Project
	deps    Dependencies
	opts    *options
	stdout  io.Writer
	stderr  io.Writer
}

type commandAdapter struct {
	name         string
	use          string
	short        string
	needsProject bool
	run          func(ctx commandContext, args []string) error
}

func commandAdapters() []commandAdapter {
	return builtinCommandRegistry().Adapters()
}
