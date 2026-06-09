package runner

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/applause/bachkator/internal/quality"
	statestore "github.com/applause/bachkator/internal/state"
)

type Session struct {
	runner     Runner
	project    *Project
	state      *State
	run        *RunRecord
	plan       *Plan
	gitContext GitContext
	dotenv     map[string]string

	fingerprints map[string]string
	dirtyTargets map[string]StateRecord
	stateMu      sync.Mutex
	outputMu     sync.Mutex
	locks        *lockManager
}

func newSession(
	r Runner,
	project *Project,
	state *State,
	run *RunRecord,
	plan *Plan,
	gitContext GitContext,
	dotenv map[string]string,
) *Session {
	return &Session{
		runner:       r,
		project:      project,
		state:        state,
		run:          run,
		plan:         plan,
		gitContext:   gitContext,
		dotenv:       dotenv,
		fingerprints: map[string]string{},
		dirtyTargets: map[string]StateRecord{},
		locks:        newLockManager(),
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
	if quality.IsGateError(err) {
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
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	record := s.run.Targets[target]
	record.Status = status
	record.FinishedAt = time.Now().UTC()
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
	if !s.runner.streamsTarget(target) {
		return
	}
	s.outputMu.Lock()
	defer s.outputMu.Unlock()
	_, _ = fmt.Fprintf(s.runner.Stdout, format, args...)
}

func (s *Session) commandOutput(target *Target, stream io.Writer, logFile io.Writer) io.Writer {
	if !s.runner.streamsTarget(target) {
		return logFile
	}
	return io.MultiWriter(stream, logFile)
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

func (s *Session) markTargetDirty(targetName string, record StateRecord) {
	s.state.Targets[targetName] = record
	s.dirtyTargets[targetName] = record
}

func (s *Session) printRunSummary() {
	printRunSummary(s.runner.Stdout, s.project, *s.run)
}
