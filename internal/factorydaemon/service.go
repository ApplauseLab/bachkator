package factorydaemon

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/applauselab/bachkator/internal/bacherr"
	"github.com/applauselab/bachkator/internal/backend"
	"github.com/applauselab/bachkator/internal/clock"
	"github.com/applauselab/bachkator/internal/config"
	"github.com/applauselab/bachkator/internal/model"
	"github.com/applauselab/bachkator/internal/quality"
	"github.com/applauselab/bachkator/internal/runner"
)

const (
	DefaultPollInterval  = 5 * time.Second
	DefaultLeaseTTL      = 30 * time.Second
	DefaultRenewInterval = 10 * time.Second

	PhasePending         = "pending"
	PhaseRunning         = "running"
	PhaseSkipped         = "skipped"
	PhaseSucceeded       = "succeeded"
	PhaseFailed          = "failed"
	PhaseWaitingApproval = "waiting_approval"
)

type Service struct {
	ConfigProject  *config.Project
	RuntimeProject *model.RunProject
	Factory        *config.Factory
	Backend        *backend.Client
	Targets        runner.TargetHandlers
	Parsers        quality.ReportParsers
	Gates          quality.GateEvaluators
	Stdout         io.Writer
	Stderr         io.Writer
	Now            clock.NowFunc
	NewID          func() (string, error)
}

type StartOptions struct {
	PollInterval  time.Duration
	LeaseTTL      time.Duration
	RenewInterval time.Duration
	DaemonID      string
	Hostname      string
	PID           int
	Yes           bool
	Force         bool
	LogOnly       bool
	Verbose       bool
	Parallelism   int
}

type StartResult struct {
	DaemonID string
	Lease    backend.FactoryDaemonLease
}

type StatusResult struct {
	Status backend.FactoryDaemonStatus
}

type StatusResultStatus = backend.FactoryDaemonStatus
type StatusResultLease = backend.FactoryDaemonLease

func (s Service) Start(ctx context.Context, opts StartOptions) (StartResult, error) {
	if err := s.validate(); err != nil {
		return StartResult{}, err
	}
	opts = s.defaults(opts)
	now := s.now()
	lease, err := s.Backend.Factory.AcquireDaemonLease(ctx, backend.FactoryDaemonLease{
		DaemonID:   opts.DaemonID,
		Factory:    s.Factory.Name,
		Hostname:   opts.Hostname,
		PID:        opts.PID,
		AcquiredAt: now,
		RenewedAt:  now,
		ExpiresAt:  now.Add(opts.LeaseTTL),
	})
	if err != nil {
		return StartResult{}, err
	}
	result := StartResult{DaemonID: opts.DaemonID, Lease: lease}
	defer func() {
		releaseCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _, _ = s.Backend.Factory.ReleaseDaemonLease(releaseCtx, opts.DaemonID, s.now())
	}()
	if _, err := fmt.Fprintf(
		s.stdout(),
		"factory daemon %s started factory=%s\n",
		opts.DaemonID,
		s.Factory.Name,
	); err != nil {
		return result, err
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	renewErr := make(chan error, 1)
	go s.renewLease(runCtx, opts, renewErr)
	var triggerErr <-chan error
	if len(s.Factory.ProviderTriggers()) > 0 {
		triggerErr = s.startProviderTriggers(runCtx)
	}
	poll := time.NewTicker(opts.PollInterval)
	defer poll.Stop()
	for {
		select {
		case <-ctx.Done():
			return result, nil
		case err := <-renewErr:
			return result, err
		case err := <-triggerErr:
			if err != nil {
				return result, err
			}
		default:
		}
		if err := s.processOne(runCtx, opts); err != nil {
			_, _ = fmt.Fprintf(s.stderr(), "factory daemon item failed: %v\n", err)
		}
		select {
		case <-ctx.Done():
			return result, nil
		case err := <-renewErr:
			return result, err
		case err := <-triggerErr:
			if err != nil {
				return result, err
			}
		case <-poll.C:
		}
	}
}

func (s Service) Status(ctx context.Context) (StatusResult, error) {
	if err := s.validate(); err != nil {
		return StatusResult{}, err
	}
	status, err := s.Backend.Factory.DaemonStatus(ctx, s.Factory.Name, s.now())
	return StatusResult{Status: status}, err
}

func (s Service) processOne(ctx context.Context, opts StartOptions) error {
	now := s.now()
	item, ok, err := s.Backend.Factory.ClaimWorkItem(
		ctx,
		s.Factory.Name,
		opts.DaemonID,
		now,
		now.Add(opts.LeaseTTL),
	)
	if err != nil || !ok {
		return err
	}
	workflow, err := s.workflow(item.Workflow)
	if err != nil {
		_, _, _ = s.Backend.Factory.FailWorkItem(
			ctx,
			item.Factory,
			item.ID,
			"plan",
			err.Error(),
			s.now(),
		)
		return err
	}
	attemptID := firstAttemptID(item)
	if attemptID == "" {
		err := fmt.Errorf("work item %s has no attempt", item.ID)
		_, _, _ = s.Backend.Factory.FailWorkItem(
			ctx,
			item.Factory,
			item.ID,
			"plan",
			err.Error(),
			s.now(),
		)
		return err
	}
	if err := s.runPlanPhase(ctx, opts, item, attemptID, workflow); err != nil {
		return s.failItem(ctx, item, config.FactoryPhasePlan, err)
	}
	planPath := interpolate(workflow.Plan[0].Path, item, s.Factory.Name, workflow.Name)
	if workflow.PlanRequiresApproval() {
		if err := s.ensurePlanApproval(ctx, item, attemptID, planPath); err != nil {
			if errors.Is(err, bacherr.ErrWaitingApproval) {
				return nil
			}
			return s.failItem(ctx, item, config.FactoryPhasePlan, err)
		}
	}
	if err := s.runImplementPhase(ctx, opts, item, attemptID, workflow, planPath); err != nil {
		return s.failItem(ctx, item, config.FactoryPhaseImplement, err)
	}
	if len(workflow.Merge) == 1 {
		if err := s.runTargetPhase(
			ctx,
			opts,
			item,
			attemptID,
			config.FactoryPhaseMerge,
			workflow.Merge[0].Target,
		); err != nil {
			return s.failItem(ctx, item, config.FactoryPhaseMerge, err)
		}
	}
	for _, phase := range workflow.Deploy {
		key := config.FactoryPhaseDeploy(phase.Name)
		if phase.RequiresApproval != nil && *phase.RequiresApproval {
			if err := s.ensureDeployApproval(ctx, item, attemptID, key); err != nil {
				if errors.Is(err, bacherr.ErrWaitingApproval) {
					return nil
				}
				return s.failItem(ctx, item, key, err)
			}
		}
		if err := s.runTargetPhase(ctx, opts, item, attemptID, key, phase.Target); err != nil {
			return s.failItem(ctx, item, key, err)
		}
	}
	for _, phase := range workflow.Verify {
		key := config.FactoryPhaseVerify(phase.Name)
		if err := s.runTargetPhase(ctx, opts, item, attemptID, key, phase.Target); err != nil {
			return s.failItem(ctx, item, key, err)
		}
	}
	_, _, err = s.Backend.Factory.CompleteWorkItem(ctx, item.Factory, item.ID, s.now())
	return err
}

func (s Service) failItem(
	ctx context.Context,
	item backend.FactoryWorkItem,
	phase string,
	cause error,
) error {
	_, _, _ = s.Backend.Factory.FailWorkItem(
		ctx,
		item.Factory,
		item.ID,
		phase,
		cause.Error(),
		s.now(),
	)
	return cause
}

func (s Service) validate() error {
	if s.ConfigProject == nil || s.RuntimeProject == nil || s.Factory == nil {
		return fmt.Errorf("config project, runtime project, and factory are required")
	}
	if s.Backend.Factory == (backend.FactoryQueueClient{}) {
		return fmt.Errorf("backend factory client is required")
	}
	return nil
}

func (s Service) defaults(opts StartOptions) StartOptions {
	if opts.PollInterval <= 0 {
		opts.PollInterval = DefaultPollInterval
	}
	if opts.LeaseTTL <= 0 {
		opts.LeaseTTL = DefaultLeaseTTL
	}
	if opts.RenewInterval <= 0 {
		opts.RenewInterval = DefaultRenewInterval
	}
	if opts.DaemonID == "" {
		opts.DaemonID, _ = firstID(s.NewID)
	}
	if opts.Hostname == "" {
		opts.Hostname, _ = os.Hostname()
	}
	if opts.PID == 0 {
		opts.PID = os.Getpid()
	}
	return opts
}

func (s Service) workflow(name string) (*config.FactoryWorkflow, error) {
	for _, workflow := range s.Factory.Workflows {
		if workflow != nil && workflow.Name == name {
			if !workflow.DaemonExecutable() {
				return nil, fmt.Errorf(
					"factory %q workflow %q is not daemon executable",
					s.Factory.Name,
					name,
				)
			}
			return workflow, nil
		}
	}
	return nil, fmt.Errorf("factory %q has no workflow %q", s.Factory.Name, name)
}

func (s Service) now() time.Time {
	return clock.UTC(s.Now)
}

func (s Service) stdout() io.Writer {
	if s.Stdout != nil {
		return s.Stdout
	}
	return io.Discard
}

func (s Service) stderr() io.Writer {
	if s.Stderr != nil {
		return s.Stderr
	}
	return io.Discard
}
