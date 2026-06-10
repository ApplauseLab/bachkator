package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	gitpkg "github.com/applause/bachkator/internal/git"
	"github.com/applause/bachkator/internal/model"
	statestore "github.com/applause/bachkator/internal/state"
)

type Project = model.RunProject
type Target = model.RunTarget
type State = statestore.State
type StateRecord = statestore.Record
type RunRecord = statestore.RunRecord
type TargetRunRecord = statestore.TargetRunRecord
type ToolRequirement = model.ToolRequirement
type PreflightCheck = model.PreflightCheck
type CompletionCheck = model.CompletionCheckSpec
type Input = model.Input
type RetryPolicy = model.RetryPolicy
type QualityGateSpec = model.QualityGateSpec
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
}

func (r Runner) Run(ctx context.Context, project *Project, name string) error {
	if r.Parallelism < 1 {
		r.Parallelism = 1
	}
	store := statestore.NewStore(project.StatePath)
	load := store.Load
	if r.DryRun {
		load = store.LoadReadOnly
	}
	state, err := load()
	if err != nil {
		return err
	}
	gitContext := gitpkg.LoadContext(ctx, project.Root)
	plan, err := BuildPlan(project, name)
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
	run := newRunRecord(project, plan.TargetName, r.DryRun, r.Force)
	state.Runs = append(state.Runs, run)
	runIndex := len(state.Runs) - 1
	session := newSession(
		r,
		project,
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
		session.completeCheckFailedRun("failed")
		return err
	}
	if err := reportOrCheckPlannedPreflights(ctx, r.Stdout, r.DryRun, plan.Preflights); err != nil {
		var checkErr PreflightCheckError
		if errors.As(err, &checkErr) {
			for _, failure := range checkErr.Failures {
				session.recordPreflightFailure(failure, checkErr.Error())
			}
		}
		session.completeCheckFailedRun("preflight-failed")
		return err
	}
	if err := session.printPipelineHeaders(ctx, plan.TargetName, map[string]bool{}); err != nil {
		return session.completeFailedRun(err)
	}
	if err := r.runTargets(ctx, session); err != nil {
		return session.completeFailedRun(err)
	}
	return session.completeRun("success")
}
