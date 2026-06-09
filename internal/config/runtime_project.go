package config

import "github.com/applause/bachkator/internal/model"

func RuntimeProject(project *Project) *model.RunProject {
	if project == nil {
		return nil
	}
	runtimeProject := &model.RunProject{
		DefaultTarget:    project.DefaultTarget,
		Root:             project.Root,
		StatePath:        project.StatePath,
		Env:              append([]string(nil), project.Env...),
		SelectedProfiles: append([]string(nil), project.SelectedProfiles...),
		ProfileEnv:       append([]string(nil), project.ProfileEnv...),
		Inputs:           cloneInputs(project.Inputs),
		Resources:        cloneResources(project.Resources),
		Producers:        cloneStringMap(project.Producers),
		Targets:          make(map[string]*model.RunTarget, len(project.Targets)),
		Aliases:          cloneAliases(project.Aliases),
	}
	for name, target := range project.Targets {
		runtimeProject.Targets[name] = runtimeTarget(target)
	}
	return runtimeProject
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
