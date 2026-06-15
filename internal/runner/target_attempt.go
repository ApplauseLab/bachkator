package runner

import (
	"context"
	"io"
	"time"

	"github.com/applauselab/bachkator/internal/quality"
	targetpkg "github.com/applauselab/bachkator/internal/target"
)

func (r *Runner) runCommandContractAndQuality(
	ctx context.Context,
	s *Session,
	target *Target,
	workdir string,
	runtimeEnv map[string]string,
	logFile syncWriter,
) error {
	spec := target.Spec()
	attempts := spec.Runtime.Retry.Attempts
	if attempts == 0 {
		attempts = 1
	}
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if attempt > 1 {
			s.printf(target, "[%s] retry attempt %d/%d\n", targetLabel(target), attempt, attempts)
			logf(logFile, "[%s] retry attempt %d/%d\n", targetLabel(target), attempt, attempts)
		}
		stdout := s.commandOutput(target, s.runner.Stdout, logFile)
		stderr := s.commandOutput(target, s.runner.Stderr, logFile)
		err := executeTarget(
			ctx,
			s.targets,
			target,
			targetpkg.ExecuteRequest{
				Env:       runtimeEnv,
				WorkDir:   workdir,
				StatePath: s.project.StatePath,
				Stdout:    stdout,
				Stderr:    stderr,
				Now:       s.runner.Now,
				RunPolicyTarget: func(ctx context.Context, req targetpkg.PolicyTargetRequest) error {
					return r.runAgentPolicyTarget(ctx, s, req)
				},
				RunRequiredTargets: func(ctx context.Context, req targetpkg.RequiredTargetsRequest) error {
					return r.runAgentRequiredTargets(ctx, s, req)
				},
			},
		)
		flushCommandOutput(stdout)
		flushCommandOutput(stderr)
		commandErr := err
		if err == nil {
			err = logFile.Sync()
		}
		if err == nil {
			err = evaluateCompletionContract(
				ctx,
				s.project,
				target,
				workdir,
				runtimeEnv,
				absPath(s.project.Root, s.targetLogPath(target.Name)),
				logFile,
			)
		}
		if timeoutErr := targetRuntimeError(ctx, target); timeoutErr != nil {
			err = timeoutErr
			commandErr = timeoutErr
		}
		if err == nil {
			skipQualitySave := spec.Runtime.Retry.RetryOnQualityGateFailure && attempt < attempts
			err = quality.IngestReports(
				ctx,
				quality.IngestRequest{
					StatePath:      s.project.StatePath,
					RunID:          s.run.ID,
					TargetName:     target.Name,
					ProjectRoot:    s.project.Root,
					Workdir:        workdir,
					Env:            runtimeEnv,
					Plugins:        s.project.Plugins,
					Reports:        spec.Quality.Reports,
					Parsers:        r.reportParsers(),
					RegoPolicies:   spec.Quality.RegoPolicies,
					Gates:          spec.Quality.Gates,
					GateEvaluators: r.gateEvaluators(),
					SaveReports:    s.backend.Quality.RecordReports,
					Log:            s.progressLog(target, logFile),
					SkipSave:       skipQualitySave,
					Now:            s.runner.Now,
				},
			)
		}
		if commandErr != nil {
			_ = quality.IngestReports(
				ctx,
				quality.IngestRequest{
					StatePath:            s.project.StatePath,
					RunID:                s.run.ID,
					TargetName:           target.Name,
					ProjectRoot:          s.project.Root,
					Workdir:              workdir,
					Env:                  runtimeEnv,
					Plugins:              s.project.Plugins,
					Reports:              spec.Quality.Reports,
					Parsers:              r.reportParsers(),
					RegoPolicies:         spec.Quality.RegoPolicies,
					Gates:                spec.Quality.Gates,
					GateEvaluators:       r.gateEvaluators(),
					SaveReports:          s.backend.Quality.RecordReports,
					Log:                  s.progressLog(target, logFile),
					AllowMissingReports:  true,
					TreatFailuresAsNotes: true,
					Now:                  s.runner.Now,
				},
			)
		}
		if err == nil {
			return nil
		}
		lastErr = err
		if attempt == attempts || ctx.Err() != nil || !shouldRetryAttempt(err, spec.Runtime.Retry) {
			break
		}
		if spec.Runtime.Retry.Backoff > 0 {
			s.printf(
				target,
				"[%s] attempt %d/%d failed: %v; retrying in %s\n",
				targetLabel(target),
				attempt,
				attempts,
				err,
				spec.Runtime.Retry.Backoff,
			)
			logf(
				logFile,
				"[%s] attempt %d/%d failed: %v; retrying in %s\n",
				targetLabel(target),
				attempt,
				attempts,
				err,
				spec.Runtime.Retry.Backoff,
			)
			select {
			case <-time.After(spec.Runtime.Retry.Backoff):
			case <-ctx.Done():
				return targetRuntimeError(ctx, target)
			}
		} else {
			s.printf(
				target,
				"[%s] attempt %d/%d failed: %v; retrying\n",
				targetLabel(target),
				attempt,
				attempts,
				err,
			)
			logf(
				logFile,
				"[%s] attempt %d/%d failed: %v; retrying\n",
				targetLabel(target),
				attempt,
				attempts,
				err,
			)
		}
	}
	return lastErr
}

func flushCommandOutput(w io.Writer) {
	if flusher, ok := w.(interface{ Flush() }); ok {
		flusher.Flush()
	}
}

type syncWriter interface {
	io.Writer
	Sync() error
}
