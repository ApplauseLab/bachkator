package runner

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/applause/bachkator/internal/model"
	"github.com/applause/bachkator/internal/quality"
	targetpkg "github.com/applause/bachkator/internal/target"
)

func (r Runner) runOne(ctx context.Context, s *Session, plan *Plan, target *Target) error {
	if _, ok := target.Spec().Body.(model.PipelineSpec); ok {
		return r.runPipeline(ctx, s, target)
	}
	ctx, cancel := targetRuntimeContext(ctx, target)
	defer cancel()
	fingerprintInputs := s.dependencyFingerprints(plan, target.Name)
	s.stateMu.Lock()
	record := s.state.Targets[target.Name]
	s.stateMu.Unlock()
	runDirectory, err := s.ensureTargetRunDirectory(target.Name)
	if err != nil {
		return err
	}
	runtimeEnv := commandEnv(
		s.gitContext,
		s.dotenv,
		projectRuntimeEnv(s.project),
		[]string{"BACH_RUN_DIRECTORY=" + runDirectory, "RUN_DIRECTORY=" + runDirectory},
		target.Env,
	)
	description, err := targetOperation(ctx, target, runtimeEnv)
	if err != nil {
		return err
	}
	targetRun := s.startTarget(target, description.Operation)
	logFile, err := s.openTargetLog(target.Name)
	if err != nil {
		return err
	}
	defer func() { _ = logFile.Close() }()
	logf(
		logFile,
		"[%s] run=%s target=%s\n",
		targetRun.StartedAt.Format(time.RFC3339Nano),
		s.run.ID,
		target.Name,
	)

	fingerprint, fingerprintParts, err := targetFingerprintParts(
		s.project,
		target,
		s.gitContext,
		s.dotenv,
		fingerprintInputs,
	)
	if err != nil {
		s.finishTarget(target.Name, "failed")
		return err
	}

	if !targetRunnable(target) {
		r.finishNoopTarget(s, target, fingerprint, logFile)
		return nil
	}
	if !r.Force && targetCacheable(target) &&
		targetFresh(target, s.project.Root, record, fingerprint) {
		s.printf(target, "[%s] up to date\n", target.Name)
		logf(logFile, "[%s] up to date\n", target.Name)
		s.rememberFingerprint(target.Name, fingerprint)
		s.finishTarget(target.Name, "cached")
		return nil
	}
	if targetCacheable(target) {
		reasons := targetStaleReasons(
			target,
			s.project.Root,
			record,
			fingerprint,
			fingerprintParts,
			r.Force,
		)
		if len(reasons) > 0 {
			s.printf(target, "[%s] stale: %s\n", target.Name, strings.Join(reasons, ", "))
			logf(logFile, "[%s] stale: %s\n", target.Name, strings.Join(reasons, ", "))
		}
	}

	workdir := s.project.Root
	if shell, ok := target.Spec().Body.(model.ShellSpec); ok && shell.WorkDir != "" {
		workdir = absPath(s.project.Root, shell.WorkDir)
	}

	s.printf(target, "[%s] %s\n", targetLabel(target), description.Operation)
	logf(logFile, "[%s] %s\n", targetLabel(target), description.Operation)
	if r.DryRun {
		s.rememberFingerprint(target.Name, fingerprint)
		s.finishTarget(target.Name, "dry-run")
		return nil
	}

	if err := r.runCommandAndContract(ctx, s, target, workdir, runtimeEnv, logFile); err != nil {
		s.finishTarget(target.Name, "failed")
		return err
	}
	spec := target.Spec()
	if err := quality.IngestReports(
		ctx,
		quality.IngestRequest{
			StatePath:  s.project.StatePath,
			RunID:      s.run.ID,
			TargetName: target.Name,
			Workdir:    workdir,
			Env:        runtimeEnv,
			Reports:    spec.Quality.Reports,
			Gates:      spec.Quality.Gates,
			Log:        logFile,
		},
	); err != nil {
		s.finishTarget(target.Name, "quality-failed")
		return err
	}
	if targetCacheable(target) {
		fingerprint, fingerprintParts, err = targetFingerprintParts(
			s.project,
			target,
			s.gitContext,
			s.dotenv,
			fingerprintInputs,
		)
		if err != nil {
			s.finishTarget(target.Name, "failed")
			return err
		}
		s.rememberFingerprint(target.Name, fingerprint)
		record := StateRecord{
			Fingerprint:      fingerprint,
			FingerprintParts: fingerprintParts,
			CompletedAt:      time.Now().UTC(),
		}
		s.stateMu.Lock()
		s.markTargetDirty(target.Name, record)
		s.stateMu.Unlock()
		s.finishTarget(target.Name, "success")
		return nil
	}
	s.rememberFingerprint(target.Name, fingerprint)
	s.finishTarget(target.Name, "success")
	return nil
}

func (r Runner) runCommandAndContract(
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
		err := executeTarget(
			ctx,
			target,
			targetpkg.ExecuteRequest{
				Env:     runtimeEnv,
				WorkDir: workdir,
				Stdout:  s.commandOutput(target, s.runner.Stdout, logFile),
				Stderr:  s.commandOutput(target, s.runner.Stderr, logFile),
			},
		)
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
		}
		if err == nil {
			return nil
		}
		lastErr = err
		if attempt == attempts || ctx.Err() != nil {
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

type syncWriter interface {
	io.Writer
	Sync() error
}

func targetRuntimeContext(
	ctx context.Context,
	target *Target,
) (context.Context, context.CancelFunc) {
	timeout := target.Spec().Runtime.Timeout
	if timeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, timeout)
}

func targetRuntimeError(ctx context.Context, target *Target) error {
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf(
			"target %q timed out after %s",
			target.Name,
			target.Spec().Runtime.Timeout,
		)
	}
	return ctx.Err()
}

func (r Runner) finishNoopTarget(
	s *Session,
	target *Target,
	fingerprint string,
	logFile io.Writer,
) {
	if len(target.DependsOn) > 0 {
		s.printf(target, "[%s] aggregate\n", target.Name)
		logf(logFile, "[%s] aggregate\n", target.Name)
	} else {
		s.printf(target, "[%s] noop\n", target.Name)
		logf(logFile, "[%s] noop\n", target.Name)
	}
	s.rememberFingerprint(target.Name, fingerprint)
	if !r.DryRun {
		record := StateRecord{Fingerprint: fingerprint, CompletedAt: time.Now().UTC()}
		s.stateMu.Lock()
		s.markTargetDirty(target.Name, record)
		s.stateMu.Unlock()
	}
	s.finishTarget(target.Name, "success")
}
