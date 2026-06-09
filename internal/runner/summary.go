package runner

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const failureExcerptLines = 20
const failurePatternLines = 8

func printRunSummary(stdout io.Writer, project *Project, run RunRecord) {
	if stdout == nil {
		return
	}
	_, _ = io.WriteString(stdout, formatRunSummary(project, run))
}

func formatRunSummary(project *Project, run RunRecord) string {
	var out strings.Builder
	duration := run.FinishedAt.Sub(run.StartedAt)
	if run.FinishedAt.IsZero() || duration < 0 {
		duration = 0
	}
	fmt.Fprintf(
		&out,
		"run %s %s target=%s duration=%s logs=%s\n",
		run.ID,
		run.Status,
		run.Target,
		formatSummaryDuration(duration),
		summaryLogDir(project, run.LogDir),
	)
	fmt.Fprintf(&out, "targets: %s\n", formatTargetStatusCounts(run.Targets))

	if run.Status == "failed" || run.Status == "preflight-failed" ||
		run.Status == "quality-failed" {
		if target, logPath := firstFailedTarget(run); logPath != "" {
			if lines, err := failureExcerptLinesForLog(logPath); err == nil && len(lines) > 0 {
				fmt.Fprintf(&out, "failure excerpt (%s):\n", target)
				for _, line := range lines {
					out.WriteString(line)
					out.WriteByte('\n')
				}
			}
		}
	}
	return out.String()
}

func formatSummaryDuration(duration time.Duration) string {
	return duration.Round(time.Millisecond).String()
}

func summaryLogDir(project *Project, logDir string) string {
	if project == nil || project.Root == "" || logDir == "" {
		return logDir
	}
	rel, err := filepath.Rel(project.Root, logDir)
	if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) ||
		rel == ".." {
		return logDir
	}
	return rel
}

func formatTargetStatusCounts(targets map[string]TargetRunRecord) string {
	counts := map[string]int{}
	for _, target := range targets {
		counts[target.Status]++
	}
	ordered := []string{
		"success",
		"cached",
		"failed",
		"preflight-failed",
		"quality-failed",
		"dry-run",
		"running",
	}
	seen := map[string]bool{}
	parts := make([]string, 0, len(ordered)+len(counts))
	for _, status := range ordered {
		seen[status] = true
		parts = append(parts, fmt.Sprintf("%s=%d", status, counts[status]))
	}
	var extra []string
	for status := range counts {
		if !seen[status] {
			extra = append(extra, status)
		}
	}
	sort.Strings(extra)
	for _, status := range extra {
		parts = append(parts, fmt.Sprintf("%s=%d", status, counts[status]))
	}
	return strings.Join(parts, " ")
}

func firstFailedTarget(run RunRecord) (string, string) {
	type failedTarget struct {
		name      string
		startedAt time.Time
		logPath   string
	}
	failed := make([]failedTarget, 0)
	for name, target := range run.Targets {
		if target.Status == "failed" || target.Status == "preflight-failed" ||
			target.Status == "quality-failed" {
			failed = append(
				failed,
				failedTarget{name: name, startedAt: target.StartedAt, logPath: target.LogPath},
			)
		}
	}
	if len(failed) == 0 {
		return "", ""
	}
	sort.Slice(failed, func(i, j int) bool {
		if failed[i].startedAt.Equal(failed[j].startedAt) {
			return failed[i].name < failed[j].name
		}
		return failed[i].startedAt.Before(failed[j].startedAt)
	})
	return failed[0].name, failed[0].logPath
}

func failureExcerptLinesForLog(path string) ([]string, error) {
	lines, err := nonEmptyLines(path)
	if err != nil {
		return nil, err
	}
	if matches := likelyFailureLines(lines, failurePatternLines); len(matches) > 0 {
		return matches, nil
	}
	return lastLines(lines, failureExcerptLines), nil
}

func nonEmptyLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	var lines []string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

func likelyFailureLines(lines []string, limit int) []string {
	if limit <= 0 {
		return nil
	}
	matches := make([]string, 0, limit)
	seen := map[string]bool{}
	for _, line := range lines {
		if !isLikelyFailureLine(line) || seen[line] {
			continue
		}
		seen[line] = true
		matches = append(matches, line)
		if len(matches) == limit {
			break
		}
	}
	return matches
}

func isLikelyFailureLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	lower := strings.ToLower(trimmed)
	needles := []string{
		"--- fail:",
		"fail\t",
		"fail:",
		"panic:",
		"fatal error:",
		"traceback ",
		"exception:",
		"error:",
		"error from server",
		"docker: error response from daemon",
		"failed to ",
		"failed with ",
		"failed at ",
		"build failed",
		"cannot find module",
		"command not found",
		"no such file",
		"permission denied",
		"npm err!",
	}
	for _, needle := range needles {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return strings.HasPrefix(trimmed, "FAIL\t") || strings.HasPrefix(trimmed, "FAIL ") ||
		strings.HasPrefix(trimmed, "FAIL:")
}

func lastLines(lines []string, limit int) []string {
	if limit <= 0 || len(lines) == 0 {
		return nil
	}
	if len(lines) <= limit {
		return lines
	}
	return lines[len(lines)-limit:]
}
