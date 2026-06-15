package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	backendpkg "github.com/applauselab/bachkator/internal/backend"
	"github.com/applauselab/bachkator/internal/clock"
	gitpkg "github.com/applauselab/bachkator/internal/git"
	"github.com/applauselab/bachkator/internal/model"
	"github.com/applauselab/bachkator/internal/quality"
	targetpkg "github.com/applauselab/bachkator/internal/target"
)

type Project = model.RunProject
type Target = model.RunTarget
type State = backendpkg.State
type StateRecord = backendpkg.StateRecord
type RunRecord = backendpkg.RunRecord
type TargetRunRecord = backendpkg.TargetRunRecord
type ToolRequirement = model.ToolRequirement
type PreflightCheck = model.PreflightCheck
type CompletionCheck = model.CompletionCheckSpec
type Input = model.Input
type RetryPolicy = model.RetryPolicy
type QualityGateSpec = model.QualityGateSpec
type QualityReportDeclaration = model.QualityReportDeclaration
type GitContext = gitpkg.Context

type Runner struct {
	DryRun      bool
	PlanJSON    bool
	Force       bool
	Yes         bool
	EnvFile     string
	LogOnly     bool
	Verbose     bool
	Parallelism int
	Stdout      io.Writer
	Stderr      io.Writer
	Targets     TargetHandlers
	Parsers     quality.ReportParsers
	Gates       quality.GateEvaluators
	Now         clock.NowFunc
}

type TargetHandlers interface {
	Handler(model.TargetType) (targetpkg.TargetHandler, error)
}

func (r *Runner) Run(ctx context.Context, project *Project, name string) error {
	return r.RunTargets(ctx, project, []string{name})
}

func (r *Runner) targetHandlers() TargetHandlers {
	if r.Targets != nil {
		return r.Targets
	}
	return targetpkg.BuiltinTargetRegistry()
}

func (r *Runner) reportParsers() quality.ReportParsers {
	if r.Parsers != nil {
		return r.Parsers
	}
	return quality.BuiltinReportParserRegistry()
}

func (r *Runner) gateEvaluators() quality.GateEvaluators {
	if r.Gates != nil {
		return r.Gates
	}
	return quality.BuiltinGateRegistry()
}

func (r *Runner) RunTargets(ctx context.Context, project *Project, names []string) error {
	if r.Parallelism < 1 {
		r.Parallelism = 1
	}
	store := backendpkg.NewProjectClient(project.Root, project.StatePath, project.Backend)
	load := func() (*State, error) { return store.Load(ctx) }
	if r.DryRun {
		load = func() (*State, error) { return store.LoadReadOnly(ctx) }
	}
	state, err := load()
	if err != nil {
		return err
	}
	gitContext := gitpkg.LoadContext(ctx, project.Root)
	plan, err := BuildPlanForTargetsWithHandlers(project, names, r.targetHandlers())
	if err != nil {
		return err
	}
	execGraph, err := buildExecutionGraph(plan)
	if err != nil {
		return err
	}
	if !r.DryRun && !r.Yes {
		if plan.EffectiveRisk.RequiresConfirmation {
			return fmt.Errorf(
				"target %q requires confirmation (risks: %s); run with -dry-run to inspect or -yes to execute",
				plan.TargetName,
				strings.Join(plan.EffectiveRisk.Labels(), ", "),
			)
		}
	}
	dotenv, err := loadDotenv(project.Root, r.EnvFile)
	if err != nil {
		return err
	}
	if r.DryRun && r.PlanJSON {
		return r.writeDryRunPlanJSON(r.Stdout, r.Force, project, state, plan, gitContext, dotenv)
	}
	run := newRunRecord(project, plan.TargetName, r.DryRun, r.Force, r.now())
	state.Runs = append(state.Runs, run)
	runIndex := len(state.Runs) - 1
	if !r.DryRun {
		if err := store.Runs.Create(ctx, run); err != nil {
			return err
		}
	}
	session := newSession(
		r,
		project,
		store,
		state,
		&state.Runs[runIndex],
		plan,
		execGraph,
		gitContext,
		dotenv,
	)
	if err := reportOrCheckPlannedRequiredTools(ctx, r.Stdout, r.DryRun, plan.Tools); err != nil {
		var checkErr ToolCheckError
		if errors.As(err, &checkErr) {
			for _, failure := range checkErr.Failures {
				session.recordToolCheckFailure(failure, checkErr.Error())
			}
		}
		session.completeCheckFailedRun(ctx, model.RunStatusFailed)
		return err
	}
	if err := reportOrCheckPlannedPreflights(ctx, r.Stdout, r.DryRun, plan.Preflights); err != nil {
		var checkErr PreflightCheckError
		if errors.As(err, &checkErr) {
			for _, failure := range checkErr.Failures {
				session.recordPreflightFailure(failure, checkErr.Error())
			}
		}
		session.completeCheckFailedRun(ctx, model.RunStatusPreflightFailed)
		return err
	}
	seenHeaders := map[string]bool{}
	for _, targetName := range plan.RequestedTargets {
		if err := session.printPipelineHeaders(ctx, targetName, seenHeaders); err != nil {
			return session.completeFailedRun(ctx, err)
		}
	}
	if err := r.runTargets(ctx, session); err != nil {
		return session.completeFailedRun(ctx, err)
	}
	return session.completeRun(ctx, model.RunStatusSuccess)
}

func (r *Runner) now() time.Time {
	return clock.UTC(r.Now)
}
