package cli

import (
	"context"
	"io"

	"github.com/applauselab/bachkator/internal/query"
	"github.com/spf13/cobra"
)

type QualitySnapshot = query.QualitySnapshot
type QLimits = query.QualityLimits

type QResult struct {
	Snapshot QualitySnapshot
	Err      error
}

type QualityQuerier interface {
	QueryQuality(*Project, QLimits) QResult
}

type QualityQueryFunc func(*Project, QLimits) QResult

func (f QualityQueryFunc) QueryQuality(project *Project, limits QLimits) QResult {
	return f(project, limits)
}

type InspectRunFunc func(project *Project, opts query.RunInspectOptions) (query.RunInspection, error)
type ListRunsFunc func(project *Project, opts query.RunListOptions) ([]query.RunListRecord, error)
type ListArtifactsFunc func(
	project *Project,
	opts query.ArtifactListOptions,
) ([]query.ArtifactListRecord, error)
type LogsFunc func(project *Project, opts query.LogOptions) ([]query.LogSection, error)

type commandContext struct {
	context context.Context
	project *Project
	deps    Dependencies
	opts    *options
	stdout  io.Writer
	stderr  io.Writer
	stdin   io.Reader
}

type commandAdapter struct {
	name               string
	use                string
	short              string
	needsProject       bool
	disableFlagParsing bool
	run                func(ctx commandContext, args []string) error
	bindFlags          func(cmd *cobra.Command, opts *options)
	subcommand         func(deps Dependencies, opts *options, stdout, stderr io.Writer, stdin io.Reader) *cobra.Command
}

func commandAdapters() []commandAdapter {
	return builtinCommandRegistry().Adapters()
}
