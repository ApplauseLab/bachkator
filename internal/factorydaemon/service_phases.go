package factorydaemon

import (
	"context"

	"github.com/applauselab/bachkator/internal/backend"
	"github.com/applauselab/bachkator/internal/config"
	"github.com/applauselab/bachkator/internal/planexecute"
	"github.com/applauselab/bachkator/internal/runner"
)

func (s Service) runPlanPhase(
	ctx context.Context,
	opts StartOptions,
	item backend.FactoryWorkItem,
	attemptID string,
	workflow *config.FactoryWorkflow,
) error {
	planPath := interpolate(workflow.Plan[0].Path, item, s.Factory.Name, workflow.Name)
	if err := validatePlanFile(s.ConfigProject.Root, planPath); err == nil {
		return s.writePhase(
			ctx,
			item,
			attemptID,
			config.FactoryPhasePlan,
			PhaseSkipped,
			"",
			"",
			planPath,
			"",
		)
	}
	if err := s.writePhase(
		ctx,
		item,
		attemptID,
		config.FactoryPhasePlan,
		PhaseRunning,
		"",
		"",
		planPath,
		"",
	); err != nil {
		return err
	}
	targetName, workspace, project, err := s.materializePlanningTarget(item, workflow, planPath)
	if err != nil {
		return err
	}
	if err := s.runner(opts).RunTargets(ctx, project, []string{targetName}); err != nil {
		return err
	}
	if err := copyPlannerPlan(s.ConfigProject.Root, workspace, planPath); err != nil {
		return err
	}
	return s.writePhase(
		ctx,
		item,
		attemptID,
		config.FactoryPhasePlan,
		PhaseSucceeded,
		targetName,
		latestRunID(ctx, s.Backend, targetName),
		planPath,
		"",
	)
}

func (s Service) runImplementPhase(
	ctx context.Context,
	opts StartOptions,
	item backend.FactoryWorkItem,
	attemptID string,
	workflow *config.FactoryWorkflow,
	planPath string,
) error {
	if err := s.writePhase(
		ctx,
		item,
		attemptID,
		config.FactoryPhaseImplement,
		PhaseRunning,
		"",
		"",
		planPath,
		"",
	); err != nil {
		return err
	}
	result, err := planexecute.Service{
		Project: s.RuntimeProject,
		Backend: s.Backend,
		Targets: s.Targets,
		Parsers: s.Parsers,
		Gates:   s.Gates,
		Stdout:  s.stdout(),
		Stderr:  s.stderr(),
		Now:     s.Now,
	}.Implement(ctx, planexecute.Options{
		Path:        planPath,
		Template:    workflow.Implement[0].AgentTemplate,
		Yes:         opts.Yes,
		Force:       opts.Force,
		LogOnly:     opts.LogOnly,
		Verbose:     opts.Verbose,
		Parallelism: opts.Parallelism,
	})
	ledgerID := ""
	if result.Ledger != nil {
		ledgerID = result.Ledger.LedgerID
	}
	status := PhaseSucceeded
	if err != nil {
		status = PhaseFailed
	}
	if phaseErr := s.writePhase(
		ctx,
		item,
		attemptID,
		config.FactoryPhaseImplement,
		status,
		result.Target,
		result.RunID,
		planPath,
		ledgerID,
	); phaseErr != nil {
		return phaseErr
	}
	return err
}

func (s Service) runTargetPhase(
	ctx context.Context,
	opts StartOptions,
	item backend.FactoryWorkItem,
	attemptID string,
	phaseKey string,
	targetName string,
) error {
	canonical, _ := s.RuntimeProject.ResolveTargetName(targetName)
	if err := s.writePhase(
		ctx,
		item,
		attemptID,
		phaseKey,
		PhaseRunning,
		canonical,
		"",
		"",
		"",
	); err != nil {
		return err
	}
	err := s.runner(opts).RunTargets(ctx, s.RuntimeProject, []string{canonical})
	status := PhaseSucceeded
	if err != nil {
		status = PhaseFailed
	}
	if phaseErr := s.writePhase(
		ctx,
		item,
		attemptID,
		phaseKey,
		status,
		canonical,
		latestRunID(ctx, s.Backend, canonical),
		"",
		"",
	); phaseErr != nil {
		return phaseErr
	}
	return err
}

func (s Service) writePhase(
	ctx context.Context,
	item backend.FactoryWorkItem,
	attemptID string,
	phaseKey string,
	status string,
	targetName string,
	runID string,
	planPath string,
	ledgerID string,
) error {
	now := s.now()
	phase := backend.FactoryWorkItemPhase{
		WorkItemID: item.ID,
		AttemptID:  attemptID,
		PhaseKey:   phaseKey,
		Status:     status,
		Target:     targetName,
		RunID:      runID,
		PlanPath:   planPath,
		LedgerID:   ledgerID,
		UpdatedAt:  now,
	}
	if status == PhaseRunning {
		phase.StartedAt = now
	}
	if status == PhaseSucceeded || status == PhaseFailed || status == PhaseSkipped {
		phase.FinishedAt = now
	}
	return s.Backend.Factory.UpdatePhase(ctx, phase)
}

func (s Service) runner(opts StartOptions) *runner.Runner {
	r := runner.Runner{
		Yes:         opts.Yes,
		Force:       opts.Force,
		LogOnly:     opts.LogOnly,
		Verbose:     opts.Verbose,
		Parallelism: opts.Parallelism,
		Stdout:      s.stdout(),
		Stderr:      s.stderr(),
		Targets:     s.Targets,
		Parsers:     s.Parsers,
		Gates:       s.Gates,
		Now:         s.Now,
	}
	return &r
}
