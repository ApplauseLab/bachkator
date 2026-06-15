package runner

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/applauselab/bachkator/internal/id"
	"github.com/applauselab/bachkator/internal/model"
)

func newRunRecord(project *Project, target string, dryRun, force bool, now time.Time) RunRecord {
	now = now.UTC()
	runID := id.MustNew()
	logDir := filepath.Join(filepath.Dir(project.StatePath), "runs", runID)
	return RunRecord{
		ID:        runID,
		Target:    target,
		DryRun:    dryRun,
		Force:     force,
		Status:    model.RunStatusRunning,
		StartedAt: now,
		LogDir:    logDir,
		Targets:   map[string]TargetRunRecord{},
	}
}

func targetRunDirectory(run *RunRecord, target string) string {
	return filepath.Join(run.LogDir, sanitizeLogName(target))
}

func targetLogPath(run *RunRecord, target string) string {
	return filepath.Join(run.LogDir, sanitizeLogName(target)+".log")
}

func sanitizeLogName(name string) string {
	replacer := strings.NewReplacer("/", "__", "\\", "__", ":", "_", " ", "_")
	return replacer.Replace(name)
}

func logf(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

func (r *Runner) streamsProgress(target *Target) bool {
	return r.Verbose || target == nil || !target.Spec().Runtime.Quiet
}

func (r *Runner) streamsCommandOutput(target *Target) bool {
	if r.LogOnly {
		return false
	}
	return r.streamsProgress(target)
}
