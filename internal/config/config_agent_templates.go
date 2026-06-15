package config

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/applauselab/bachkator/internal/model"
)

var agentTemplatePlaceholderPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

var allowedAgentTemplatePlaceholders = map[string]struct{}{
	"work_item.id":   {},
	"work_item.slug": {},
	"plan.id":        {},
	"workstream.id":  {},
	"factory.name":   {},
	"workflow.name":  {},
}

func registerAgentTemplates(project *Project, templates []*AgentTemplate) error {
	for _, template := range templates {
		if template.Name == "" {
			return fmt.Errorf("agent_template block must have a name")
		}
		key := canonicalPrimitiveRef(template.Name, "agent_template")
		if _, exists := project.AgentTemplates[key]; exists {
			return fmt.Errorf("duplicate agent_template %q", template.Name)
		}
		if err := resolveAgentTemplateReferences(project, template); err != nil {
			return err
		}
		project.AgentTemplates[key] = template
	}
	return nil
}

func resolveAgentTemplateReferences(project *Project, template *AgentTemplate) error {
	if template.Mode == "" {
		template.Mode = "implement"
	}
	if err := validateAgentMode("agent_template", template.Name, template.Mode); err != nil {
		return err
	}
	if template.Provider == "" {
		return fmt.Errorf("agent_template %q provider is required", template.Name)
	}
	if err := rejectAgentTemplatePlaceholders(
		template.Name,
		"provider",
		template.Provider,
	); err != nil {
		return err
	}
	provider, ok := project.Providers[canonicalPrimitiveRef(template.Provider, "provider")]
	if !ok {
		return fmt.Errorf(
			"agent_template %q references unknown provider %q",
			template.Name,
			template.Provider,
		)
	}
	if provider.Type == "opencode" && template.Mode != "implement" {
		return fmt.Errorf(
			"agent_template %q opencode provider is supported only for implement mode",
			template.Name,
		)
	}
	template.ProviderConfig = provider
	if err := resolveAgentTemplatePrompt(project, template); err != nil {
		return err
	}
	if err := resolveAgentTemplatePolicy(project, template); err != nil {
		return err
	}
	if err := validateAgentTemplateBlocks(template); err != nil {
		return err
	}
	return rejectAgentTemplatePlaceholders(template.Name, "role", template.Role)
}

func resolveAgentTemplatePrompt(project *Project, template *AgentTemplate) error {
	if template.Prompt == "" {
		return nil
	}
	if err := rejectAgentTemplatePlaceholders(
		template.Name,
		"prompt",
		template.Prompt,
	); err != nil {
		return err
	}
	promptRef := canonicalPrimitiveRef(template.Prompt, "prompt")
	prompt, ok := project.Prompts[promptRef]
	if !ok {
		prompt = project.Prompts[strings.TrimPrefix(promptRef, "prompt/")]
		ok = prompt != nil
	}
	if !ok {
		return fmt.Errorf(
			"agent_template %q references unknown prompt %q",
			template.Name,
			template.Prompt,
		)
	}
	template.PromptConfig = prompt
	return nil
}

func resolveAgentTemplatePolicy(project *Project, template *AgentTemplate) error {
	if template.Policy == "" {
		return nil
	}
	if err := rejectAgentTemplatePlaceholders(
		template.Name,
		"policy",
		template.Policy,
	); err != nil {
		return err
	}
	if template.Mode != "implement" {
		return fmt.Errorf(
			"agent_template %q policy is supported only for implement mode",
			template.Name,
		)
	}
	policy, ok := project.Policies[canonicalPrimitiveRef(template.Policy, "policy")]
	if !ok {
		return fmt.Errorf(
			"agent_template %q references unknown policy %q",
			template.Name,
			template.Policy,
		)
	}
	template.AgentPolicy = model.Policy{
		Name:             policy.Name,
		Subject:          policy.Subject,
		SubjectWorkspace: policy.SubjectWorkspace,
		SubjectCommit:    policy.SubjectCommit,
		RequiredTargets:  append([]string(nil), policy.RequiredTargets...),
		Reviewers:        append([]string(nil), policy.Reviewers...),
		Gates:            qualityGateSpecs(policy.QualityGates),
	}
	return nil
}

func validateAgentTemplateBlocks(template *AgentTemplate) error {
	if len(template.Workspace) > 1 {
		return fmt.Errorf("agent_template %q must have at most one workspace block", template.Name)
	}
	if len(template.Git) > 1 {
		return fmt.Errorf("agent_template %q must have at most one git block", template.Name)
	}
	if len(template.Workspace) > 0 {
		workspace := template.Workspace[0]
		if workspace.Mode == "" {
			workspace.Mode = "clone"
		}
		if err := rejectAgentTemplatePlaceholders(
			template.Name,
			"workspace.mode",
			workspace.Mode,
		); err != nil {
			return err
		}
		if workspace.Mode != "clone" {
			return fmt.Errorf("agent_template %q workspace mode must be %q", template.Name, "clone")
		}
		if workspace.Path != "" {
			if err := allowAgentTemplatePlaceholders(
				template.Name,
				"workspace.path",
				workspace.Path,
			); err != nil {
				return err
			}
			if err := validateAgentWorkspacePath(workspace.Path); err != nil {
				return fmt.Errorf("agent_template %q workspace path: %w", template.Name, err)
			}
		}
	}
	if len(template.Git) == 0 {
		return nil
	}
	git := template.Git[0]
	if git.Branch != "" {
		if err := allowAgentTemplatePlaceholders(
			template.Name,
			"git.branch",
			git.Branch,
		); err != nil {
			return err
		}
		if err := validateGitBranchName(git.Branch); err != nil {
			return fmt.Errorf("agent_template %q git branch: %w", template.Name, err)
		}
	}
	if err := rejectAgentTemplatePlaceholders(template.Name, "git.commit", git.Commit); err != nil {
		return err
	}
	if git.Commit != "" && git.Commit != "required" && git.Commit != "optional" {
		return fmt.Errorf(
			"agent_template %q git commit must be required or optional",
			template.Name,
		)
	}
	return nil
}

func applyAgentTemplate(project *Project, agent *Target) error {
	if agent.Template == "" {
		return nil
	}
	ref := canonicalPrimitiveRef(agent.Template, "agent_template")
	template, ok := project.AgentTemplates[ref]
	if !ok {
		return fmt.Errorf(
			"target %q references unknown agent_template %q",
			agent.Name,
			agent.Template,
		)
	}
	agent.Template = ref
	if agent.Mode == "" {
		agent.Mode = template.Mode
	}
	if agent.Provider == "" {
		agent.Provider = template.Provider
	}
	if agent.Role == "" {
		agent.Role = template.Role
	}
	if agent.Prompt == "" {
		agent.Prompt = template.Prompt
	}
	if agent.Policy == "" {
		agent.Policy = template.Policy
	}
	if len(agent.Workspace) == 0 {
		agent.Workspace = cloneAgentWorkspaceBlocks(template.Workspace)
	}
	if len(agent.Git) == 0 {
		agent.Git = cloneAgentGitBlocks(template.Git)
	}
	return nil
}

func validateConcreteAgentPlaceholders(agent *Target) error {
	fields := map[string]string{
		"template": agent.Template,
		"mode":     agent.Mode,
		"provider": agent.Provider,
		"role":     agent.Role,
		"prompt":   agent.Prompt,
		"plan":     agent.Plan,
		"subject":  agent.Subject,
		"policy":   agent.Policy,
	}
	for field, value := range fields {
		if containsAgentTemplatePlaceholder(value) {
			return fmt.Errorf(
				"target %q %s must not contain agent template placeholders",
				agent.Name,
				field,
			)
		}
	}
	for _, workspace := range agent.Workspace {
		if workspace == nil {
			continue
		}
		if containsAgentTemplatePlaceholder(workspace.Mode) {
			return fmt.Errorf(
				"target %q workspace.mode must not contain agent template placeholders",
				agent.Name,
			)
		}
		if containsAgentTemplatePlaceholder(workspace.Path) {
			return fmt.Errorf(
				"target %q workspace.path must not contain agent template placeholders",
				agent.Name,
			)
		}
	}
	for _, git := range agent.Git {
		if git == nil {
			continue
		}
		if containsAgentTemplatePlaceholder(git.Branch) {
			return fmt.Errorf(
				"target %q git.branch must not contain agent template placeholders",
				agent.Name,
			)
		}
		if containsAgentTemplatePlaceholder(git.Commit) {
			return fmt.Errorf(
				"target %q git.commit must not contain agent template placeholders",
				agent.Name,
			)
		}
	}
	return nil
}

func cloneAgentWorkspaceBlocks(blocks []*AgentWorkspaceBlock) []*AgentWorkspaceBlock {
	if len(blocks) == 0 {
		return nil
	}
	out := make([]*AgentWorkspaceBlock, 0, len(blocks))
	for _, block := range blocks {
		if block == nil {
			out = append(out, nil)
			continue
		}
		out = append(out, &AgentWorkspaceBlock{Mode: block.Mode, Path: block.Path})
	}
	return out
}

func cloneAgentGitBlocks(blocks []*AgentGitBlock) []*AgentGitBlock {
	if len(blocks) == 0 {
		return nil
	}
	out := make([]*AgentGitBlock, 0, len(blocks))
	for _, block := range blocks {
		if block == nil {
			out = append(out, nil)
			continue
		}
		out = append(out, &AgentGitBlock{Branch: block.Branch, Commit: block.Commit})
	}
	return out
}

func allowAgentTemplatePlaceholders(templateName string, field string, value string) error {
	for _, match := range agentTemplatePlaceholderPattern.FindAllStringSubmatch(value, -1) {
		if _, ok := allowedAgentTemplatePlaceholders[match[1]]; !ok {
			return fmt.Errorf(
				"agent_template %q %s contains unsupported placeholder %q",
				templateName,
				field,
				match[0],
			)
		}
	}
	return nil
}

func rejectAgentTemplatePlaceholders(templateName string, field string, value string) error {
	if !containsAgentTemplatePlaceholder(value) {
		return nil
	}
	return fmt.Errorf("agent_template %q %s must not contain placeholders", templateName, field)
}

func containsAgentTemplatePlaceholder(value string) bool {
	return agentTemplatePlaceholderPattern.MatchString(value)
}

func validateAgentMode(kind string, name string, mode string) error {
	if mode == "implement" || mode == "review" || mode == "merge" {
		return nil
	}
	return fmt.Errorf("%s %q mode must be implement, review, or merge", kind, name)
}
