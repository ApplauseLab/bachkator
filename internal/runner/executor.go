package runner

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/applauselab/bachkator/internal/model"
	"github.com/applauselab/bachkator/internal/quality"
)

func (r *Runner) runOne(ctx context.Context, s *Session, plan *Plan, target *Target) error {
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
		target.Env,
		[]string{"BACH_RUN_DIRECTORY=" + runDirectory, "RUN_DIRECTORY=" + runDirectory},
	)
	description, err := targetOperation(ctx, s.targets, target, runtimeEnv)
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
		s.targets,
		s.project,
		target,
		s.dotenv,
		fingerprintInputs,
	)
	if err != nil {
		s.finishTargetWithExitCode(target.Name, model.RunStatusFailed, exitCodeFromError(err))
		return err
	}

	if !targetRunnable(s.targets, target) {
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
	status := targetStaleStatus(
		target,
		s.project.Root,
		record,
		fingerprint,
		fingerprintParts,
		r.Force,
	)

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
			s.finishTarget(target.Name, model.RunStatusQualityFailed)
			return err
		}
		s.finishTargetWithExitCode(target.Name, model.RunStatusFailed, exitCodeFromError(err))
		return err
	}
	if targetCacheable(target) {
		fingerprint, fingerprintParts, err = targetFingerprintParts(
			s.targets,
			s.project,
			target,
			s.dotenv,
			fingerprintInputs,
		)
		if err != nil {
			s.finishTarget(target.Name, model.RunStatusFailed)
			return err
		}
		s.rememberFingerprint(target.Name, fingerprint)
		record := StateRecord{
			Fingerprint:      fingerprint,
			FingerprintParts: fingerprintParts,
			CompletedAt:      s.now(),
		}
		s.stateMu.Lock()
		s.markTargetDirty(target.Name, record)
		s.stateMu.Unlock()
		s.finishTarget(target.Name, model.RunStatusSuccess)
		return nil
	}
	s.rememberFingerprint(target.Name, fingerprint)
	s.finishTarget(target.Name, model.RunStatusSuccess)
	return nil
}

func (r *Runner) printTargetOperation(
	s *Session,
	target *Target,
	status string,
	operation string,
	logFile io.Writer,
) {
	line := TargetOperationLine{
		Timestamp: s.now(),
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
