package factorydaemon

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/applauselab/bachkator/internal/backend"
	"github.com/applauselab/bachkator/internal/id"
	"github.com/applauselab/bachkator/internal/model"
	"github.com/applauselab/bachkator/internal/plan"
)

func firstAttemptID(item backend.FactoryWorkItem) string {
	if len(item.Attempts) == 0 {
		return ""
	}
	return item.Attempts[0].ID
}

func firstID(fn func() (string, error)) (string, error) {
	if fn != nil {
		return fn()
	}
	return id.New()
}

func cloneProject(project *model.RunProject) *model.RunProject {
	clone := *project
	clone.Targets = make(map[string]*model.RunTarget, len(project.Targets)+1)
	for key, value := range project.Targets {
		clone.Targets[key] = value
	}
	return &clone
}

func canonicalTemplateRef(ref string) string {
	if strings.HasPrefix(ref, "agent_template/") {
		return ref
	}
	return "agent_template/" + strings.TrimPrefix(ref, "agent_template.")
}

func interpolate(
	template string,
	item backend.FactoryWorkItem,
	factory string,
	workflow string,
) string {
	replacer := strings.NewReplacer(
		"${work_item.id}", item.ID,
		"${work_item.slug}", item.ID,
		"${factory.name}", factory,
		"${workflow.name}", workflow,
	)
	return replacer.Replace(template)
}

func copyPlannerPlan(root string, workspace string, planPath string) error {
	if err := validateProjectRelativePlanPath(planPath); err != nil {
		return err
	}
	source := filepath.Join(workspace, filepath.FromSlash(planPath))
	if err := ensureContained(workspace, source); err != nil {
		return err
	}
	info, err := os.Lstat(source)
	if err != nil {
		return fmt.Errorf("planner output %q: %w", planPath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("planner output %q must be a regular file", planPath)
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("planner output %q: %w", planPath, err)
	}
	destination := filepath.Join(root, filepath.FromSlash(planPath))
	if err := ensureContained(root, destination); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(destination, data, 0o644); err != nil {
		return err
	}
	return validatePlanFile(root, planPath)
}

func latestRunID(ctx context.Context, client *backend.Client, targetName string) string {
	runs, err := client.Runs.List(ctx, backend.RunQuery{Target: targetName, Limit: 1})
	if err != nil || len(runs) == 0 {
		return ""
	}
	return runs[0].ID
}

func validatePlanFile(root string, planPath string) error {
	if err := validateProjectRelativePlanPath(planPath); err != nil {
		return err
	}
	path := filepath.Join(root, filepath.FromSlash(planPath))
	if err := ensureContained(root, path); err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	_, diagnostics := plan.Parse(planPath, data)
	if len(diagnostics) > 0 {
		return fmt.Errorf("plan %q has %d diagnostics", planPath, len(diagnostics))
	}
	return nil
}

func validateProjectRelativePlanPath(planPath string) error {
	if planPath == "" || filepath.IsAbs(planPath) || strings.Contains(planPath, "\\") {
		return fmt.Errorf("plan path %q must be project-relative", planPath)
	}
	cleaned := filepath.Clean(filepath.FromSlash(planPath))
	if cleaned == "." || cleaned == ".." ||
		strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return fmt.Errorf("plan path %q must stay inside the project", planPath)
	}
	return nil
}

func ensureContained(root string, path string) error {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(rootAbs, pathAbs)
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path %q escapes %q", path, root)
	}
	return nil
}
