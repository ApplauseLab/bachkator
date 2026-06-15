package config

import "github.com/applauselab/bachkator/internal/model"

func RuntimeProject(project *Project) *model.RunProject {
	if project == nil {
		return nil
	}
	runtimeProject := &model.RunProject{
		DefaultTarget:    project.DefaultTarget,
		Root:             project.Root,
		StatePath:        project.StatePath,
		Backend:          runtimeBackend(project.Backend),
		Env:              append([]string(nil), project.Env...),
		SelectedProfiles: append([]string(nil), project.SelectedProfiles...),
		ProfileEnv:       append([]string(nil), project.ProfileEnv...),
		Inputs:           cloneInputs(project.Inputs),
		Resources:        cloneResources(project.Resources),
		Producers:        cloneStringMap(project.Producers),
		Plugins:          clonePlugins(project.Plugins),
		Providers:        cloneProviders(project.Providers),
		Prompts:          clonePrompts(project.Prompts),
		Policies:         clonePolicies(project.Policies),
		AgentTemplates:   cloneAgentTemplates(project.AgentTemplates),
		Targets:          make(map[string]*model.RunTarget, len(project.Targets)),
		Aliases:          cloneAliases(project.Aliases),
	}
	for name, target := range project.Targets {
		runtimeProject.Targets[name] = runtimeTarget(target)
	}
	for name, target := range project.Targets {
		if !hasAttachedPolicy(target) {
			continue
		}
		policyName := target.AgentPolicy.Name
		generatedName := model.GeneratedPolicyTargetAddress(policyName, name).LegacyName()
		runtimeProject.Targets[generatedName] = generatedPolicyTarget(generatedName, target)
	}
	return runtimeProject
}

func runtimeBackend(backend Backend) model.Backend {
	config := make(map[string]string, len(backend.Config))
	for key, value := range backend.Config {
		config[key] = value
	}
	return model.Backend{
		Type:    backend.Type,
		Command: append([]string(nil), backend.Command...),
		Config:  config,
	}
}

func hasAttachedPolicy(target *Target) bool {
	return target != nil && target.Mode == "implement" && target.AgentPolicy.Name != ""
}

func generatedPolicyTarget(name string, subject *Target) *model.RunTarget {
	workspace := ""
	branch := ""
	if len(subject.Workspace) > 0 {
		workspace = subject.Workspace[0].Path
	}
	if len(subject.Git) > 0 {
		branch = subject.Git[0].Branch
	}
	return &model.RunTarget{
		Name: name,
		Env: []string{
			"BACH_POLICY_SUBJECT=" + subject.Name,
			"BACH_POLICY_SUBJECT_WORKSPACE=" + workspace,
		},
		SpecValue: model.TargetSpec{
			Name: name,
			Metadata: model.TargetMetadata{
				Description: "generated policy fan-out for " + subject.Name,
			},
			Quality: model.TargetQuality{
				Reports: []model.QualityReportDeclaration{{
					Kind:   "policy",
					Format: "agent-report-json",
					Path:   "$BACH_RUN_DIRECTORY/policy-report.json",
				}},
				RegoPolicies: append(
					[]model.RegoPolicySpec(nil),
					subject.Spec().Quality.RegoPolicies...,
				),
				Gates: subject.AgentPolicy.Gates,
			},
			Body: model.PolicySpec{
				Policy: subject.AgentPolicy,
				Subject: model.AgentSubject{
					Target:       subject.Name,
					Workspace:    workspace,
					Branch:       branch,
					Plan:         subject.Plan,
					PolicyTarget: name,
				},
			},
		},
	}
}

func cloneProviders(providers map[string]*Provider) map[string]*model.Provider {
	if len(providers) == 0 {
		return nil
	}
	out := make(map[string]*model.Provider, len(providers))
	for key, provider := range providers {
		if provider == nil {
			continue
		}
		out[key] = &model.Provider{
			Name:    provider.Name,
			Type:    provider.Type,
			Command: append([]string(nil), provider.Command...),
		}
	}
	return out
}

func clonePrompts(prompts map[string]*Prompt) map[string]*model.Prompt {
	if len(prompts) == 0 {
		return nil
	}
	out := make(map[string]*model.Prompt, len(prompts))
	for key, prompt := range prompts {
		if prompt == nil {
			continue
		}
		out[key] = &model.Prompt{
			Name:        prompt.Name,
			Path:        prompt.Path,
			Description: prompt.Description,
			Version:     prompt.Version,
		}
	}
	return out
}

func cloneAgentTemplates(templates map[string]*AgentTemplate) map[string]*model.AgentTemplate {
	if len(templates) == 0 {
		return nil
	}
	out := make(map[string]*model.AgentTemplate, len(templates))
	for key, template := range templates {
		if template == nil {
			continue
		}
		out[key] = &model.AgentTemplate{
			Name:      template.Name,
			Mode:      template.Mode,
			Provider:  providerSpec(template.ProviderConfig),
			Role:      template.Role,
			Prompt:    promptSpec(template.PromptConfig),
			Policy:    template.AgentPolicy,
			Workspace: agentWorkspaceSpec(template.Workspace),
			Git:       agentGitSpec(template.Git),
		}
	}
	return out
}

func clonePlugins(plugins map[string]*Plugin) map[string]*model.Plugin {
	if len(plugins) == 0 {
		return nil
	}
	out := make(map[string]*model.Plugin, len(plugins))
	for key, plugin := range plugins {
		if plugin == nil {
			continue
		}
		sources := make(map[string][]string, len(plugin.Sources))
		for name, values := range plugin.Sources {
			sources[name] = append([]string(nil), values...)
		}
		out[key] = &model.Plugin{
			Name:    plugin.Name,
			Type:    plugin.Type,
			Command: append([]string(nil), plugin.Command...),
			Shell:   plugin.Shell,
			WorkDir: plugin.WorkDir,
			Inputs:  append([]string(nil), plugin.Inputs...),
			Env:     append([]string(nil), plugin.Env...),
			Sources: sources,
			Timeout: plugin.TimeoutDuration,
		}
	}
	return out
}

func runtimeTarget(target *Target) *model.RunTarget {
	if target == nil {
		return nil
	}
	spec := target.Spec()
	return &model.RunTarget{
		Name:      target.Name,
		DependsOn: append([]string(nil), target.DependsOn...),
		Env:       append([]string(nil), target.Env...),
		Outputs:   append([]string(nil), target.Outputs...),
		OutputMap: cloneStringMap(target.OutputMap),
		SpecValue: spec,
	}
}

func clonePolicies(policies map[string]*Policy) map[string]*model.Policy {
	if len(policies) == 0 {
		return nil
	}
	out := make(map[string]*model.Policy, len(policies))
	for key, policy := range policies {
		if policy == nil {
			continue
		}
		out[key] = &model.Policy{
			Name:             policy.Name,
			Subject:          policy.Subject,
			SubjectWorkspace: policy.SubjectWorkspace,
			SubjectCommit:    policy.SubjectCommit,
			RequiredTargets:  append([]string(nil), policy.RequiredTargets...),
			Reviewers:        append([]string(nil), policy.Reviewers...),
			Gates:            qualityGateSpecs(policy.QualityGates),
		}
	}
	return out
}

func cloneInputs(inputs map[string]*Input) map[string]*model.Input {
	if len(inputs) == 0 {
		return nil
	}
	out := make(map[string]*model.Input, len(inputs))
	for key, input := range inputs {
		if input == nil {
			continue
		}
		out[key] = &model.Input{
			Kind: input.Kind,
			Name: input.Name,
			Src:  input.Src,
			Srcs: append([]string(nil), input.Srcs...),
		}
	}
	return out
}

func cloneResources(resources map[string]*Resource) map[string]*model.Resource {
	if len(resources) == 0 {
		return nil
	}
	out := make(map[string]*model.Resource, len(resources))
	for key, resource := range resources {
		if resource == nil {
			continue
		}
		out[key] = &model.Resource{Name: resource.Name}
	}
	return out
}

func cloneAliases(aliases map[string]*Alias) map[string]*model.Alias {
	if len(aliases) == 0 {
		return nil
	}
	out := make(map[string]*model.Alias, len(aliases))
	for key, alias := range aliases {
		if alias == nil {
			continue
		}
		out[key] = &model.Alias{
			Name:       alias.Name,
			Target:     alias.Target,
			Deprecated: alias.Deprecated,
		}
	}
	return out
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}
