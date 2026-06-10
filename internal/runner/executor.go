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
		s.dotenv,
		fingerprintInputs,
	)
	if err != nil {
		s.finishTarget(target.Name, "failed")
		return err
	}

	if !targetRunnable(target) {
		r.finishNoopTarget(s, target, fingerprint, description.Operation, logFile)
		return nil
	}
	if r.DryRun {
		status := ""
		if targetCacheable(target) && !r.Force &&
			targetFresh(target, s.project.Root, record, fingerprint) {
			status = "cached"
		} else if targetCacheable(target) {
			reasons := targetStaleReasons(
				target,
				s.project.Root,
				record,
				fingerprint,
				fingerprintParts,
				r.Force,
			)
			if len(reasons) > 0 {
				status = "stale: " + strings.Join(reasons, ", ")
			}
		}
		r.printTargetOperation(s, target, status, description.Operation, logFile)
		s.rememberFingerprint(target.Name, fingerprint)
		s.finishTarget(target.Name, "dry-run")
		return nil
	}
	if !r.Force && targetCacheable(target) &&
		targetFresh(target, s.project.Root, record, fingerprint) {
		r.printTargetOperation(s, target, "cached", description.Operation, logFile)
		s.rememberFingerprint(target.Name, fingerprint)
		s.finishTarget(target.Name, "cached")
		return nil
	}
	status := ""
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
			status = "stale: " + strings.Join(reasons, ", ")
		}
	}

	workdir := s.project.Root
	if shell, ok := target.Spec().Body.(model.ShellSpec); ok && shell.WorkDir != "" {
		workdir = absPath(s.project.Root, shell.WorkDir)
	}

	r.printTargetOperation(s, target, status, description.Operation, logFile)

	if err := r.runCommandContractAndQuality(
		ctx,
		s,
		target,
		workdir,
		runtimeEnv,
		logFile,
	); err != nil {
		if quality.IsGateError(err) || quality.IsParseError(err) {
			s.finishTarget(target.Name, "quality-failed")
			return err
		}
		s.finishTarget(target.Name, "failed")
		return err
	}
	if targetCacheable(target) {
		fingerprint, fingerprintParts, err = targetFingerprintParts(
			s.project,
			target,
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

func (r Runner) printTargetOperation(
	s *Session,
	target *Target,
	status string,
	operation string,
	logFile io.Writer,
) {
	line := TargetOperationLine{
		Timestamp: time.Now().UTC(),
		Label:     targetLabel(target),
		Status:    status,
		Operation: operation,
	}
	console := r.consoleWriter()
	if s.runner.streamsProgress(target) {
		s.outputMu.Lock()
		console.TargetOperation(s.runner.Stdout, line)
		s.outputMu.Unlock()
	}
	console.TargetOperation(logFile, line)
}

func (r Runner) runCommandContractAndQuality(
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
			target,
			targetpkg.ExecuteRequest{
				Env:     runtimeEnv,
				WorkDir: workdir,
				Stdout:  stdout,
				Stderr:  stderr,
			},
		)
		flushCommandOutput(stdout)
		flushCommandOutput(stderr)
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
			skipQualitySave := spec.Runtime.Retry.RetryOnQualityGateFailure && attempt < attempts
			err = quality.IngestReports(
				ctx,
				quality.IngestRequest{
					StatePath:   s.project.StatePath,
					RunID:       s.run.ID,
					TargetName:  target.Name,
					ProjectRoot: s.project.Root,
					Workdir:     workdir,
					Env:         runtimeEnv,
					Plugins:     s.project.Plugins,
					Reports:     spec.Quality.Reports,
					Gates:       spec.Quality.Gates,
					Log:         s.progressLog(target, logFile),
					SkipSave:    skipQualitySave,
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

func shouldRetryAttempt(err error, retry model.RetryPolicy) bool {
	if quality.IsParseError(err) {
		return false
	}
	if quality.IsGateError(err) {
		return retry.RetryOnQualityGateFailure
	}
	return true
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
		if target.Spec().Runtime.Timeout <= 0 {
			return ctx.Err()
		}
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
	operation string,
	logFile io.Writer,
) {
	switch {
	case operation != "":
		if _, ok := target.Spec().Body.(model.PipelineSpec); !ok {
			s.printf(target, "[%s] %s\n", target.Name, operation)
		}
		logf(logFile, "[%s] %s\n", target.Name, operation)
	case len(target.DependsOn) > 0:
		s.printf(target, "[%s] aggregate\n", target.Name)
		logf(logFile, "[%s] aggregate\n", target.Name)
	default:
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
