package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/applause/bachkator/internal/model"
	"github.com/applause/bachkator/internal/quality"
	statestore "github.com/applause/bachkator/internal/state"
)

type Session struct {
	runner     Runner
	project    *Project
	state      *State
	run        *RunRecord
	plan       *Plan
	execGraph  *executionGraph
	gitContext GitContext
	dotenv     map[string]string

	fingerprints map[string]string
	dirtyTargets map[string]StateRecord
	stateMu      sync.Mutex
	outputMu     sync.Mutex
	locks        *lockManager
	scopeCtx     map[string]context.Context
	scopeCancel  map[string]context.CancelFunc
	reservedLock map[string]string
}

func newSession(
	r Runner,
	project *Project,
	state *State,
	run *RunRecord,
	plan *Plan,
	execGraph *executionGraph,
	gitContext GitContext,
	dotenv map[string]string,
) *Session {
	return &Session{
		runner:       r,
		project:      project,
		state:        state,
		run:          run,
		plan:         plan,
		execGraph:    execGraph,
		gitContext:   gitContext,
		dotenv:       dotenv,
		fingerprints: map[string]string{},
		dirtyTargets: map[string]StateRecord{},
		locks:        newLockManager(),
		scopeCtx:     map[string]context.Context{},
		scopeCancel:  map[string]context.CancelFunc{},
		reservedLock: map[string]string{},
	}
}

func (s *Session) completeRun(status string) error {
	s.run.Status = status
	s.run.FinishedAt = time.Now().UTC()
	s.run.Artifacts = indexRunArtifacts(s.project, *s.run, s.plan.Targets)
	if !s.runner.DryRun {
		if err := statestore.NewStore(s.project.StatePath).
			RecordRunCompletion(s.dirtyTargets, *s.run); err != nil {
			return err
		}
	}
	s.printRunSummary()
	return nil
}

func (s *Session) completeFailedRun(err error) error {
	status := "failed"
	if quality.IsGateError(err) || quality.IsParseError(err) {
		status = "quality-failed"
	}
	if completeErr := s.completeRun(status); completeErr != nil {
		return completeErr
	}
	return err
}

func (s *Session) completeCheckFailedRun(status string) {
	s.run.Status = status
	s.run.FinishedAt = time.Now().UTC()
	s.run.Artifacts = indexRunArtifacts(s.project, *s.run, s.plan.Targets)
	if !s.runner.DryRun {
		_ = statestore.NewStore(s.project.StatePath).RecordRunCompletion(nil, *s.run)
	}
	s.printRunSummary()
}

func (s *Session) startTarget(target *Target, operation string) TargetRunRecord {
	startedAt := time.Now().UTC()
	record := TargetRunRecord{
		Status:    "running",
		StartedAt: startedAt,
		LogPath:   s.targetLogPath(target.Name),
		Operation: operation,
	}
	s.stateMu.Lock()
	s.run.Targets[target.Name] = record
	s.stateMu.Unlock()
	return record
}

func (s *Session) finishTarget(target string, status string) {
	s.finishTargetWithExitCode(target, status, nil)
}

func (s *Session) finishTargetWithExitCode(target string, status string, exitCode *int) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	record := s.run.Targets[target]
	record.Status = status
	record.FinishedAt = time.Now().UTC()
	if exitCode != nil {
		record.ExitCode = exitCode
	}
	s.run.Targets[target] = record
}

func (s *Session) recordSyntheticTargetFailure(
	targetName string,
	status string,
	operation string,
	message string,
) {
	startedAt := time.Now().UTC()
	s.stateMu.Lock()
	s.run.Targets[targetName] = TargetRunRecord{
		Status:     status,
		StartedAt:  startedAt,
		FinishedAt: startedAt,
		LogPath:    s.targetLogPath(targetName),
		Operation:  operation,
	}
	s.stateMu.Unlock()
	logFile, err := s.openTargetLog(targetName)
	if err != nil {
		return
	}
	defer func() { _ = logFile.Close() }()
	logf(
		logFile,
		"[%s] run=%s target=%s\n",
		startedAt.Format(time.RFC3339Nano),
		s.run.ID,
		targetName,
	)
	logf(logFile, "%s\n", message)
}

func (s *Session) recordToolCheckFailure(failure ToolCheckFailure, message string) {
	for _, targetName := range failure.Targets {
		s.recordSyntheticTargetFailure(
			targetName,
			"failed",
			"required tool check",
			fmt.Sprintf("required tool check failed\n%s", message),
		)
	}
}

func (s *Session) recordPreflightFailure(failure PreflightFailure, message string) {
	for _, targetName := range failure.Targets {
		s.recordSyntheticTargetFailure(
			targetName,
			"preflight-failed",
			"credential/session preflight",
			fmt.Sprintf("credential/session preflight failed\n%s", message),
		)
	}
}

func (s *Session) openTargetLog(target string) (*os.File, error) {
	if err := os.MkdirAll(s.run.LogDir, 0o755); err != nil {
		return nil, err
	}
	return os.OpenFile(
		absPath(s.project.Root, s.targetLogPath(target)),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0o600,
	)
}

func (s *Session) ensureTargetRunDirectory(target string) (string, error) {
	dir := targetRunDirectory(s.run, target)
	abs := absPath(s.project.Root, dir)
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func (s *Session) targetLogPath(target string) string {
	return targetLogPath(s.run, target)
}

func (s *Session) printf(target *Target, format string, args ...any) {
	if !s.runner.streamsProgress(target) {
		return
	}
	s.outputMu.Lock()
	defer s.outputMu.Unlock()
	_, _ = fmt.Fprintf(s.runner.Stdout, format, args...)
}

func (s *Session) commandOutput(target *Target, stream io.Writer, logFile io.Writer) io.Writer {
	if !s.runner.streamsCommandOutput(target) {
		return logFile
	}
	return newCommandOutputWriter(stream, logFile, s.runner.consoleWriter(), targetLabel(target))
}

func (s *Session) progressLog(target *Target, logFile io.Writer) io.Writer {
	if !s.runner.streamsProgress(target) {
		return logFile
	}
	return newCommandOutputWriter(
		s.runner.Stdout,
		logFile,
		s.runner.consoleWriter(),
		targetLabel(target),
	)
}

func (s *Session) dependencyFingerprints(plan *Plan, targetName string) map[string]string {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	return plan.DependencyFingerprints(targetName, s.fingerprints)
}

func (s *Session) rememberFingerprint(targetName string, fingerprint string) {
	s.stateMu.Lock()
	s.fingerprints[targetName] = fingerprint
	s.stateMu.Unlock()
}

func (s *Session) startScope(ctx context.Context, target *Target, operation string) {
	if target == nil {
		return
	}
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	if _, exists := s.scopeCtx[target.Name]; exists {
		return
	}
	startedAt := time.Now().UTC()
	s.run.Targets[target.Name] = TargetRunRecord{
		Status:    "running",
		StartedAt: startedAt,
		LogPath:   s.targetLogPath(target.Name),
		Operation: operation,
	}
	scopeCtx, cancel := targetRuntimeContext(s.scopedContextLocked(ctx, target.Name), target)
	s.scopeCtx[target.Name] = scopeCtx
	s.scopeCancel[target.Name] = cancel
}

func (s *Session) finishScope(target *Target) {
	if target == nil {
		return
	}
	s.stateMu.Lock()
	cancel := s.scopeCancel[target.Name]
	delete(s.scopeCtx, target.Name)
	delete(s.scopeCancel, target.Name)
	s.stateMu.Unlock()
	if cancel != nil {
		cancel()
	}
	s.locks.release(target.Spec().Runtime.Lock)
}

func (s *Session) closeActiveScopes(status string) {
	s.stateMu.Lock()
	cancels := make([]context.CancelFunc, 0, len(s.scopeCancel))
	locks := make([]string, 0, len(s.scopeCtx))
	for name, cancel := range s.scopeCancel {
		if cancel != nil {
			cancels = append(cancels, cancel)
		}
		if target := s.plan.Target(name); target != nil {
			locks = append(locks, target.Spec().Runtime.Lock)
		}
		record := s.run.Targets[name]
		if record.Status == "running" {
			record.Status = status
			record.FinishedAt = time.Now().UTC()
			s.run.Targets[name] = record
		}
	}
	s.scopeCtx = map[string]context.Context{}
	s.scopeCancel = map[string]context.CancelFunc{}
	s.stateMu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
	for _, lock := range locks {
		s.locks.release(lock)
	}
}

func (s *Session) tryReserveExecutionNode(name string) bool {
	if target := s.execGraph.scopeStartTarget(name); target != nil {
		return s.tryReserveLock(name, target.Spec().Runtime.Lock, s.execGraph.memberScopes(name))
	}
	if s.execGraph.scopeEndTarget(name) != nil || s.execGraph.isBarrier(name) {
		return true
	}
	if target := s.execGraph.target(name); target != nil {
		return s.tryReserveLock(name, target.Spec().Runtime.Lock, s.execGraph.memberScopes(name))
	}
	return true
}

func (s *Session) tryReserveLock(name string, lock string, scopes []executionScope) bool {
	if lock == "" || s.lockHeldByActiveScope(lock, scopes) {
		return true
	}
	if !s.locks.tryAcquire(lock) {
		return false
	}
	s.stateMu.Lock()
	s.reservedLock[name] = lock
	s.stateMu.Unlock()
	return true
}

func (s *Session) lockHeldByActiveScope(lock string, scopes []executionScope) bool {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	for _, scope := range scopes {
		if s.scopeCtx[scope.Name] == nil {
			continue
		}
		if target := s.plan.Target(scope.Name); target != nil &&
			target.Spec().Runtime.Lock == lock {
			return true
		}
	}
	return false
}

func (s *Session) releaseExecutionNode(name string) {
	s.stateMu.Lock()
	lock := s.reservedLock[name]
	delete(s.reservedLock, name)
	s.stateMu.Unlock()
	s.locks.release(lock)
}

func (s *Session) forgetExecutionNodeReservation(name string) {
	s.stateMu.Lock()
	delete(s.reservedLock, name)
	s.stateMu.Unlock()
}

func (s *Session) scopedContext(ctx context.Context, targetName string) context.Context {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	return s.scopedContextLocked(ctx, targetName)
}

func (s *Session) scopedContextLocked(ctx context.Context, targetName string) context.Context {
	var selected context.Context
	selectedDepth := -1
	for _, scope := range s.execGraph.memberScopes(targetName) {
		if scoped := s.scopeCtx[scope.Name]; scoped != nil && scope.Depth > selectedDepth {
			selected = scoped
			selectedDepth = scope.Depth
		}
	}
	if selected != nil {
		return selected
	}
	return ctx
}

func (s *Session) scopeRuntimeError(targetName string, err error) error {
	if err == nil {
		return nil
	}
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	var selectedErr error
	selectedDepth := -1
	for _, scope := range s.execGraph.memberScopes(targetName) {
		ctx := s.scopeCtx[scope.Name]
		target := s.plan.Target(scope.Name)
		if ctx != nil && ctx.Err() == context.DeadlineExceeded && target != nil &&
			target.Spec().Runtime.Timeout > 0 && scope.Depth > selectedDepth {
			selectedErr = targetRuntimeError(ctx, target)
			selectedDepth = scope.Depth
		}
	}
	if selectedErr != nil {
		return selectedErr
	}
	return err
}

func (s *Session) markTargetDirty(targetName string, record StateRecord) {
	s.state.Targets[targetName] = record
	s.dirtyTargets[targetName] = record
}

func (s *Session) printRunSummary() {
	printRunSummary(s.runner.Stdout, s.project, *s.run)
}

func (s *Session) printPipelineHeaders(
	ctx context.Context,
	targetName string,
	seen map[string]bool,
) error {
	if seen[targetName] {
		return nil
	}
	seen[targetName] = true
	target := s.plan.Target(targetName)
	if target == nil {
		return nil
	}
	pipeline, ok := target.Spec().Body.(model.PipelineSpec)
	if !ok {
		return nil
	}
	runtimeEnv := commandEnv(s.gitContext, s.dotenv, projectRuntimeEnv(s.project), target.Env)
	description, err := targetOperation(ctx, target, runtimeEnv)
	if err != nil {
		return err
	}
	s.printf(target, "[%s] %s\n", targetLabel(target), description.Operation)
	for _, step := range pipeline.Steps {
		if err := s.printPipelineHeaders(ctx, step, seen); err != nil {
			return err
		}
	}
	return nil
}
