package app

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/applauselab/bachkator/internal/cli"
	"github.com/applauselab/bachkator/internal/clock"
	"github.com/applauselab/bachkator/internal/config"
	"github.com/applauselab/bachkator/internal/evidence"
	gitpkg "github.com/applauselab/bachkator/internal/git"
	"github.com/applauselab/bachkator/internal/model"
	"github.com/applauselab/bachkator/internal/quality"
	"github.com/applauselab/bachkator/internal/runner"
)

func splitPolicyTargets(project *config.Project, names []string) ([]string, []string) {
	policyNames := []string{}
	targetNames := []string{}
	for _, name := range names {
		if project.Policies[name] != nil {
			policyNames = append(policyNames, name)
			continue
		}
		targetNames = append(targetNames, name)
	}
	return policyNames, targetNames
}

func runPolicyTargets(
	ctx context.Context,
	project *config.Project,
	policyNames []string,
	opts cli.RunOptions,
	stdout io.Writer,
	stderr io.Writer,
	targetHandlers runner.TargetHandlers,
	qualityParsers quality.ReportParsers,
	qualityGates quality.GateEvaluators,
) error {
	for _, policyName := range policyNames {
		policy := project.Policies[policyName]
		if policy == nil {
			return fmt.Errorf("unknown policy node %q", policyName)
		}
		if opts.DryRun {
			if _, err := fmt.Fprintf(
				stdout,
				"%s generated policy fan-out subject=%s required_targets=%s\n",
				policyName,
				policy.Subject,
				strings.Join(policy.RequiredTargets, ","),
			); err != nil {
				return err
			}
		}
		if err := runPolicyTarget(
			ctx,
			project,
			policyName,
			policy,
			opts,
			stdout,
			stderr,
			targetHandlers,
			qualityParsers,
			qualityGates,
		); err != nil {
			return err
		}
	}
	return nil
}

func runPolicyTarget(
	ctx context.Context,
	project *config.Project,
	policyName string,
	policy *config.Policy,
	opts cli.RunOptions,
	stdout io.Writer,
	stderr io.Writer,
	targetHandlers runner.TargetHandlers,
	qualityParsers quality.ReportParsers,
	qualityGates quality.GateEvaluators,
) error {
	subjectProject, err := policySubjectProject(ctx, project, policyName, policy, stdout)
	if err != nil {
		return err
	}
	allowedMutationPaths := policyAllowedMutationPaths(project, policy)
	before, err := gitStatus(ctx, subjectProject.Root, allowedMutationPaths)
	if err != nil && !opts.DryRun {
		return err
	}
	r := runner.Runner{
		DryRun: opts.DryRun, PlanJSON: opts.PlanJSON, Force: opts.Force, Yes: opts.Yes,
		EnvFile: opts.EnvFile, LogOnly: opts.LogOnly, Verbose: opts.Verbose,
		Parallelism: opts.Parallelism, Stdout: stdout, Stderr: stderr,
		Targets: targetHandlers, Parsers: qualityParsers, Gates: qualityGates,
	}
	requiredErr := r.RunTargets(ctx, config.RuntimeProject(subjectProject), policy.RequiredTargets)
	if opts.DryRun {
		return requiredErr
	}
	after, statusErr := gitStatus(ctx, subjectProject.Root, allowedMutationPaths)
	mutationFinding := ""
	if statusErr == nil && before != after {
		mutationFinding = "policy-required-target-mutated-workspace"
	}
	evaluationPath, writeErr := writePolicyEvaluation(
		subjectProject,
		policyName,
		policy,
		requiredErr,
		mutationFinding,
	)
	if writeErr != nil && !opts.DryRun {
		return writeErr
	}
	if evaluationPath != "" {
		_, _ = fmt.Fprintf(stdout, "policy evaluation: %s\n", evaluationPath)
	}
	if requiredErr != nil {
		return requiredErr
	}
	if mutationFinding != "" {
		return fmt.Errorf("%s", mutationFinding)
	}
	return nil
}

func policySubjectProject(
	ctx context.Context,
	project *config.Project,
	policyName string,
	policy *config.Policy,
	stdout io.Writer,
) (*config.Project, error) {
	subjectProject := *project
	if policy.SubjectWorkspace != "" {
		workspace := policy.SubjectWorkspace
		if !filepath.IsAbs(workspace) {
			workspace = filepath.Join(project.Root, workspace)
		}
		workspace, err := filepath.Abs(workspace)
		if err != nil {
			return nil, err
		}
		subjectProject.Root = workspace
		subjectProject.StatePath = filepath.Join(workspace, ".bach", "state.db")
	}
	if policy.SubjectCommit != "" {
		subjectProject.Env = append(
			append([]string(nil), subjectProject.Env...),
			"BACH_POLICY_SUBJECT="+policy.Subject,
			"BACH_POLICY_SUBJECT_COMMIT="+policy.SubjectCommit,
			"BACH_POLICY_NODE="+policyName,
		)
		if err := validatePolicySubjectCommit(
			ctx,
			&subjectProject,
			policyName,
			policy,
			stdout,
		); err != nil {
			return nil, err
		}
	}
	return &subjectProject, nil
}

func validatePolicySubjectCommit(
	ctx context.Context,
	subjectProject *config.Project,
	policyName string,
	policy *config.Policy,
	stdout io.Writer,
) error {
	commit, err := gitpkg.Head(ctx, subjectProject.Root)
	if err != nil {
		return err
	}
	if commit == policy.SubjectCommit {
		return nil
	}
	finding := "policy-subject-commit-mismatch"
	evaluationPath, writeErr := writePolicyEvaluation(
		subjectProject,
		policyName,
		policy,
		fmt.Errorf("subject commit mismatch: got %s want %s", commit, policy.SubjectCommit),
		finding,
	)
	if writeErr != nil {
		return writeErr
	}
	if evaluationPath != "" {
		_, _ = fmt.Fprintf(stdout, "policy evaluation: %s\n", evaluationPath)
		return fmt.Errorf("%s", finding)
	}
	return fmt.Errorf("%s", finding)
}

func gitStatus(ctx context.Context, root string, allowedPaths []string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain=v1", "-z", "-uall")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	entries := strings.Split(string(out), "\x00")
	kept := make([]string, 0, len(entries))
	for i := 0; i < len(entries); i++ {
		entry := entries[i]
		if entry == "" {
			continue
		}
		if len(entry) < 4 {
			kept = append(kept, entry)
			continue
		}
		path := entry[3:]
		if policyMutationAllowed(path, allowedPaths) {
			continue
		}
		kept = append(kept, entry)
		if entry[0] == 'R' || entry[1] == 'R' || entry[0] == 'C' || entry[1] == 'C' {
			i++
		}
	}
	return strings.Join(kept, "\n"), nil
}
func policyMutationAllowed(path string, allowedPaths []string) bool {
	path = strings.TrimPrefix(path, "\"")
	path = strings.TrimSuffix(path, "\"")
	if path == ".bach" || strings.HasPrefix(path, ".bach/") {
		return true
	}
	for _, allowed := range allowedPaths {
		if path == allowed || strings.HasPrefix(path, allowed+"/") {
			return true
		}
	}
	return false
}

func policyAllowedMutationPaths(project *config.Project, policy *config.Policy) []string {
	allowed := []string{}
	for _, targetName := range policy.RequiredTargets {
		collectTargetOutputs(project, targetName, map[string]bool{}, &allowed)
	}
	return allowed
}

func collectTargetOutputs(
	project *config.Project,
	targetName string,
	seen map[string]bool,
	out *[]string,
) {
	if seen[targetName] {
		return
	}
	seen[targetName] = true
	target := project.Targets[targetName]
	if target == nil {
		return
	}
	*out = append(*out, target.Outputs...)
	for _, output := range target.OutputMap {
		*out = append(*out, output)
	}
	for _, dep := range target.DependsOn {
		collectTargetOutputs(project, dep, seen, out)
	}
	switch body := target.Spec().Body.(type) {
	case model.PipelineSpec:
		for _, step := range body.Steps {
			collectTargetOutputs(project, step, seen, out)
		}
	case model.GroupSpec:
		for _, member := range body.Targets {
			collectTargetOutputs(project, member, seen, out)
		}
	}
}

func writePolicyEvaluation(
	project *config.Project,
	policyName string,
	policy *config.Policy,
	requiredErr error,
	finding string,
) (string, error) {
	path := filepath.Join(
		project.Root,
		".bach",
		"artifacts",
		safePolicyFilename(policyName)+".json",
	)
	status := "passed"
	if requiredErr != nil || finding != "" {
		status = "failed"
	}
	evaluation := map[string]any{
		"policy": policyName, "subject": policy.Subject, "subject_workspace": project.Root,
		"subject_commit": policy.SubjectCommit, "status": status,
		"required_targets": policy.RequiredTargets, "findings": []string{},
		"created_at": clock.SystemNow().Format(time.RFC3339Nano),
	}
	if requiredErr != nil {
		evaluation["required_target_error"] = requiredErr.Error()
	}
	if finding != "" {
		evaluation["findings"] = []string{finding}
	}
	if err := evidence.WriteJSONArtifact(path, evaluation); err != nil {
		return "", err
	}
	return path, nil
}

func safePolicyFilename(name string) string {
	replacer := strings.NewReplacer("/", "_", "@", "_", " ", "_")
	return replacer.Replace(name)
}
