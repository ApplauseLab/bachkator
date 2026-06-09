package runner

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"
)

func newRunRecord(project *Project, target string, dryRun, force bool) RunRecord {
	now := time.Now().UTC()
	id := now.Format("20060102T150405.000000000Z")
	logDir := filepath.Join(filepath.Dir(project.StatePath), "runs", id)
	return RunRecord{
		ID:        id,
		Target:    target,
		DryRun:    dryRun,
		Force:     force,
		Status:    "running",
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

func (r Runner) streamsTarget(target *Target) bool {
	if r.LogOnly {
		return false
	}
	return r.Verbose || target == nil || !target.Spec().Runtime.Quiet
}
