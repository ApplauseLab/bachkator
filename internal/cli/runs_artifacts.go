package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/applauselab/bachkator/internal/query"
)

func runListArtifacts(
	project *Project,
	deps Dependencies,
	opts *options,
	args []string,
	stdout io.Writer,
) error {
	if deps.ListArtifacts == nil {
		return fmt.Errorf("artifact list query dependency is not configured")
	}
	since, err := parseSince(opts.runsSince)
	if err != nil {
		return err
	}
	runID := ""
	if len(args) > 0 {
		runID = args[0]
	}
	artifacts, err := deps.ListArtifacts(project, query.ArtifactListOptions{
		RunID:  runID,
		Target: opts.runsTarget,
		Status: opts.runsStatus,
		Since:  since,
		Path:   opts.artifactPath,
		Limit:  opts.runsLimit,
	})
	if err != nil {
		return err
	}
	for _, artifact := range artifacts {
		if _, err := fmt.Fprintf(
			stdout,
			"%s %-12s %-24s %-12s %s\n",
			artifact.RunID,
			artifact.Kind,
			artifact.Target,
			artifact.CreatedAt.Format(timeFormat),
			artifact.Location,
		); err != nil {
			return err
		}
	}
	return nil
}

func agentReportsForTarget(
	root string,
	runID string,
	logDir string,
	targetName string,
) []agentReportInspection {
	var reports []agentReportInspection
	for _, path := range reportArtifactPaths(root, runID, logDir, "report.json") {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var raw map[string]any
		if err := json.Unmarshal(data, &raw); err != nil ||
			stringValue(raw["target"]) != targetName {
			continue
		}
		report := agentReportInspection{
			Path:               path,
			Mode:               stringValue(raw["mode"]),
			Status:             stringValue(raw["status"]),
			ProviderName:       stringValue(raw["provider_name"]),
			ProviderType:       stringValue(raw["provider_type"]),
			ProviderCommand:    stringSliceValue(raw["provider_command"]),
			Subject:            mapValue(raw["subject"]),
			PRURL:              stringValue(raw["pr_url"]),
			TargetBranchCommit: stringValue(raw["target_branch_commit"]),
			MergeCommit:        stringValue(raw["merge_commit"]),
			Summary:            stringValue(raw["summary"]),
		}
		reports = append(reports, report)
	}
	return reports
}

func policyEvaluationsForTarget(
	root string,
	runID string,
	logDir string,
	targetName string,
) []policyEvaluationInspection {
	var evaluations []policyEvaluationInspection
	for _, path := range reportArtifactPaths(root, runID, logDir, "policy-report.json") {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var raw map[string]any
		if err := json.Unmarshal(data, &raw); err != nil {
			continue
		}
		target := stringValue(raw["target"])
		if target == "" {
			if subject := mapValue(raw["subject"]); subject != nil {
				target = stringValue(subject["target"])
			}
		}
		if target != targetName {
			continue
		}
		evaluations = append(evaluations, policyEvaluationInspection{
			Path: path, Schema: stringValue(raw["schema"]), Target: target,
			Verdict: firstNonEmpty(stringValue(raw["verdict"]), stringValue(raw["status"])),
			RunID:   stringValue(raw["run_id"]),
		})
	}
	evaluations = append(evaluations, appliedPoliciesForRun(root, runID, targetName)...)
	return evaluations
}

func appliedPoliciesForRun(
	root string,
	runID string,
	targetName string,
) []policyEvaluationInspection {
	base := filepath.Join(root, ".bach", "artifacts", "policies", runID)
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil
	}
	var evaluations []policyEvaluationInspection
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(base, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var raw map[string]any
		if err := json.Unmarshal(data, &raw); err != nil ||
			stringValue(raw["target"]) != targetName {
			continue
		}
		evaluations = append(evaluations, policyEvaluationInspection{
			Path:    path,
			Schema:  stringValue(raw["schema"]),
			Target:  stringValue(raw["target"]),
			Verdict: stringValue(raw["verdict"]),
			RunID:   stringValue(raw["run_id"]),
		})
	}
	return evaluations
}

func reportArtifactPaths(root string, runID string, logDir string, suffix string) []string {
	if logDir == "" {
		return nil
	}
	base := logDir
	if !filepath.IsAbs(base) {
		base = filepath.Join(root, base)
	}
	base = filepath.Clean(base)
	expected := filepath.Join(root, ".bach", "runs", runID)
	rel, err := filepath.Rel(expected, base)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil
	}
	var paths []string
	_ = filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err == nil &&
			info != nil &&
			!info.IsDir() &&
			strings.HasSuffix(filepath.Base(path), suffix) {
			if info.Mode()&os.ModeSymlink != 0 || !pathWithin(path, expected) {
				return nil
			}
			paths = append(paths, path)
		}
		return nil
	})
	sort.Strings(paths)
	return paths
}

func pathWithin(path string, root string) bool {
	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(resolvedRoot, resolvedPath)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}

func stringSliceValue(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if text, ok := item.(string); ok {
			out = append(out, text)
		}
	}
	return out
}

func mapValue(value any) map[string]any {
	item, _ := value.(map[string]any)
	return item
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
