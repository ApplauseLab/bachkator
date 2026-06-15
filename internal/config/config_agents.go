package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/applauselab/bachkator/internal/model"
)

func resolveAgentReferences(project *Project, target *Target) error {
	if target.Mode == "" {
		target.Mode = "implement"
	}
	if err := validateAgentMode("target", target.Name, target.Mode); err != nil {
		return err
	}
	if err := validateConcreteAgentPlaceholders(target); err != nil {
		return err
	}
	if target.Provider == "" {
		return fmt.Errorf("target %q provider is required", target.Name)
	}
	provider, ok := project.Providers[canonicalPrimitiveRef(target.Provider, "provider")]
	if !ok {
		return fmt.Errorf("target %q references unknown provider %q", target.Name, target.Provider)
	}
	if provider.Type == "opencode" && target.Mode != "implement" {
		return fmt.Errorf(
			"target %q opencode provider is supported only for implement mode",
			target.Name,
		)
	}
	target.ProviderConfig = provider
	if target.Prompt != "" {
		promptRef := canonicalPrimitiveRef(target.Prompt, "prompt")
		prompt, ok := project.Prompts[promptRef]
		if !ok {
			prompt = project.Prompts[strings.TrimPrefix(promptRef, "prompt/")]
			ok = prompt != nil
		}
		if !ok {
			return fmt.Errorf("target %q references unknown prompt %q", target.Name, target.Prompt)
		}
		if err := validateProjectRelativePath("prompt path", prompt.Path); err != nil {
			return fmt.Errorf("target %q prompt %q: %w", target.Name, prompt.Name, err)
		}
		target.PromptConfig = prompt
		target.Inputs = appendUniqueString(target.Inputs, prompt.Path)
	}
	if target.Mode == "merge" {
		if target.Subject == "" {
			return fmt.Errorf("target %q subject is required for merge mode", target.Name)
		}
		subject, err := canonicalTargetRef(target.Subject)
		if err != nil {
			return fmt.Errorf("target %q subject: %w", target.Name, err)
		}
		if !strings.HasPrefix(subject, "agent/") {
			return fmt.Errorf("target %q subject must reference an agent target", target.Name)
		}
		target.Subject = subject
		if target.Lock == "" {
			target.Lock = "merge-lane"
		}
		return nil
	}
	if target.Mode != "implement" {
		if target.Policy != "" {
			return fmt.Errorf("target %q policy is supported only for implement mode", target.Name)
		}
		return nil
	}
	if target.Plan == "" {
		return fmt.Errorf("target %q plan is required", target.Name)
	}
	if err := validateProjectRelativePath("plan", target.Plan); err != nil {
		return fmt.Errorf("target %q: %w", target.Name, err)
	}
	target.Inputs = appendUniqueString(target.Inputs, target.Plan)
	if len(target.Workspace) > 1 {
		return fmt.Errorf("target %q must have at most one workspace block", target.Name)
	}
	if len(target.Git) > 1 {
		return fmt.Errorf("target %q must have at most one git block", target.Name)
	}
	if len(target.Workspace) == 0 {
		target.Workspace = []*AgentWorkspaceBlock{{
			Mode: "clone",
			Path: ".bach/agents/" + shortTargetName(target.Name),
		}}
	}
	if target.Workspace[0].Mode == "" {
		target.Workspace[0].Mode = "clone"
	}
	if target.Workspace[0].Mode != "clone" {
		return fmt.Errorf("target %q workspace mode must be %q", target.Name, "clone")
	}
	if target.Workspace[0].Path == "" {
		target.Workspace[0].Path = ".bach/agents/" + shortTargetName(target.Name)
	}
	if err := validateAgentWorkspacePath(target.Workspace[0].Path); err != nil {
		return fmt.Errorf("target %q workspace path: %w", target.Name, err)
	}
	if len(target.Git) == 0 {
		target.Git = []*AgentGitBlock{{
			Branch: "bach/agents/" + shortTargetName(target.Name),
			Commit: "required",
		}}
	}
	if target.Git[0].Branch == "" {
		target.Git[0].Branch = "bach/agents/" + shortTargetName(target.Name)
	}
	if err := validateGitBranchName(target.Git[0].Branch); err != nil {
		return fmt.Errorf("target %q git branch: %w", target.Name, err)
	}
	if target.Git[0].Commit == "" {
		target.Git[0].Commit = "required"
	}
	if target.Git[0].Commit != "required" && target.Git[0].Commit != "optional" {
		return fmt.Errorf("target %q git commit must be required or optional", target.Name)
	}
	return nil
}

func attachMergeSubjects(project *Project) error {
	for _, target := range project.Targets {
		if !targetKind(target).Is(TargetTypeAgent) || target.Mode != "merge" {
			continue
		}
		subjectTarget, ok := project.Targets[target.Subject]
		if !ok {
			return fmt.Errorf(
				"target %q references unknown subject %q",
				target.Name,
				target.Subject,
			)
		}
		if subjectTarget.Mode != "implement" {
			return fmt.Errorf(
				"target %q subject %q must be mode implement",
				target.Name,
				target.Subject,
			)
		}
		if subjectTarget.Policy == "" {
			return fmt.Errorf(
				"target %q subject %q must declare an attached policy",
				target.Name,
				target.Subject,
			)
		}
		if len(subjectTarget.Workspace) == 0 || len(subjectTarget.Git) == 0 {
			return fmt.Errorf(
				"target %q subject %q missing workspace or git",
				target.Name,
				target.Subject,
			)
		}
		target.AgentSubject = model.AgentSubject{
			Target:    target.Subject,
			Workspace: subjectTarget.Workspace[0].Path,
			Branch:    subjectTarget.Git[0].Branch,
			Plan:      subjectTarget.Plan,
			PolicyTarget: model.GeneratedPolicyTargetAddress(
				subjectTarget.AgentPolicy.Name,
				target.Subject,
			).LegacyName(),
		}
	}
	return nil
}

func attachAgentPolicies(project *Project) error {
	for _, agent := range project.Targets {
		if !targetKind(agent).Is(TargetTypeAgent) || agent.Policy == "" {
			continue
		}
		policy, ok := project.Policies[canonicalPrimitiveRef(agent.Policy, "policy")]
		if !ok {
			return fmt.Errorf("target %q references unknown policy %q", agent.Name, agent.Policy)
		}
		reviewerSpecs := make([]model.AgentSpec, 0, len(policy.Reviewers))
		for _, reviewerName := range policy.Reviewers {
			reviewer := project.Targets[reviewerName]
			if reviewer == nil {
				return fmt.Errorf(
					"policy %q references unknown reviewer %q",
					policy.Name,
					reviewerName,
				)
			}
			if reviewer.Mode != "review" {
				return fmt.Errorf(
					"policy %q reviewer %q must be mode review",
					policy.Name,
					reviewerName,
				)
			}
			reviewerSpec, ok := targetKind(reviewer).AgentSpec()
			if !ok {
				return fmt.Errorf(
					"policy %q reviewer %q must be an agent target",
					policy.Name,
					reviewerName,
				)
			}
			reviewerSpecs = append(reviewerSpecs, reviewerSpec)
		}
		agent.AgentPolicy = model.Policy{
			Name:            policy.Name,
			RequiredTargets: append([]string(nil), policy.RequiredTargets...),
			Reviewers:       append([]string(nil), policy.Reviewers...),
			ReviewerSpecs:   reviewerSpecs,
			Gates:           qualityGateSpecs(policy.QualityGates),
		}
	}
	return nil
}

func validateAgentWorkspacePath(path string) error {
	if err := validateProjectRelativePath("workspace path", path); err != nil {
		return err
	}
	cleaned := filepath.ToSlash(filepath.Clean(path))
	if cleaned != ".bach/agents" && !strings.HasPrefix(cleaned, ".bach/agents/") {
		return fmt.Errorf("must stay under .bach/agents")
	}
	return nil
}

func validateGitBranchName(branch string) error {
	if branch == "" || strings.HasPrefix(branch, "-") {
		return fmt.Errorf("must be a valid branch name")
	}
	if strings.Contains(branch, "..") || strings.Contains(branch, "//") ||
		strings.HasSuffix(branch, "/") || strings.HasSuffix(branch, ".") ||
		strings.ContainsAny(branch, " ~^:?*[\\") {
		return fmt.Errorf("must be a valid branch name")
	}
	for _, part := range strings.Split(branch, "/") {
		if part == "" || strings.HasPrefix(part, ".") || strings.HasSuffix(part, ".lock") {
			return fmt.Errorf("must be a valid branch name")
		}
	}
	return nil
}

func shortTargetName(name string) string {
	_, short, ok := strings.Cut(name, "/")
	if ok {
		return short
	}
	return name
}
