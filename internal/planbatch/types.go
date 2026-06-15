package planbatch

import (
	"context"
	"time"

	"github.com/applauselab/bachkator/internal/clock"
	"github.com/applauselab/bachkator/internal/plan"
	"github.com/applauselab/bachkator/internal/planexecute"
)

const (
	StateImplemented        = "implemented"
	StateAlreadyImplemented = "already_implemented"
	StateFailed             = "failed"
	StateBlocked            = "blocked"
	StateSkipped            = "skipped"
)

type StopMode string

const (
	StopOnFailure StopMode = "failure"
	StopOnNever   StopMode = "never"
)

func (m StopMode) Valid() bool {
	switch m {
	case StopOnFailure, StopOnNever:
		return true
	}
	return false
}

type Options struct {
	Paths       []string
	Parallelism int
	StopOn      StopMode
	Force       bool
	DryRun      bool
	Yes         bool
	EnvFile     string
	LogOnly     bool
	Verbose     bool
	Template    string
}

type PlanResult struct {
	Plan        plan.Document
	State       string
	RunID       string
	Target      string
	Ledger      *plan.LedgerSummary
	Reason      string
	Diagnostics []plan.Diagnostic
}

type Result struct {
	Plans     []PlanResult
	Waves     [][]string
	StartedAt time.Time
	EndedAt   time.Time
}

type ImplementFunc func(context.Context, planexecute.Options) (planexecute.Result, error)

type Service struct {
	Implement ImplementFunc
	Now       clock.NowFunc
}

func (s Service) now() time.Time {
	return clock.UTC(s.Now)
}
