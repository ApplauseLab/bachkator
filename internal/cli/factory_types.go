package cli

import (
	"context"
	"io"
	"time"

	factorypkg "github.com/applauselab/bachkator/internal/factory"
	"github.com/applauselab/bachkator/internal/factorydaemon"
	"github.com/applauselab/bachkator/internal/model"
)

type FactorySubmitOptions struct {
	Workflow  string
	Title     string
	Body      string
	BodyFile  string
	Priority  model.Priority
	Labels    []string
	DedupeKey string
	Plan      string
}

type FactoryListOptions struct {
	Workflow string
	Status   model.Lifecycle
}

type FactoryCancelOptions struct {
	Reason string
}

type FactoryApproveOptions struct {
	Phase  string
	Reason string
}

type FactoryStartOptions struct {
	Yes           bool
	Force         bool
	LogOnly       bool
	Verbose       bool
	Parallelism   int
	PollInterval  time.Duration
	RenewInterval time.Duration
	LeaseTTL      time.Duration
	Stdout        io.Writer
	Stderr        io.Writer
}

type FactorySubmitFunc func(
	context.Context,
	*Project,
	string,
	FactorySubmitOptions,
) (factorypkg.SubmitResult, error)

type FactoryListFunc func(
	context.Context,
	*Project,
	string,
	FactoryListOptions,
) ([]factorypkg.WorkItem, error)

type FactoryInspectFunc func(context.Context, *Project, string, string) (factorypkg.WorkItem, error)

type FactoryCancelFunc func(
	context.Context,
	*Project,
	string,
	string,
	FactoryCancelOptions,
) (factorypkg.WorkItem, error)

type FactoryApproveFunc func(
	context.Context,
	*Project,
	string,
	string,
	FactoryApproveOptions,
) (factorypkg.ApproveResult, error)

type FactoryStartFunc func(
	context.Context,
	*Project,
	string,
	FactoryStartOptions,
) (factorydaemon.StartResult, error)

type FactoryStatusFunc func(
	context.Context,
	*Project,
	string,
) (factorydaemon.StatusResult, error)
