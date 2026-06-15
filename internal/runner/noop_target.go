package runner

import (
	"io"

	"github.com/applauselab/bachkator/internal/model"
)

func (r *Runner) finishNoopTarget(
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
		record := StateRecord{Fingerprint: fingerprint, CompletedAt: s.now()}
		s.stateMu.Lock()
		s.markTargetDirty(target.Name, record)
		s.stateMu.Unlock()
	}
	s.finishTarget(target.Name, model.RunStatusSuccess)
}
