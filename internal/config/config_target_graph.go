package config

import "fmt"

func wireProducedInputs(project *Project) error {
	for _, target := range project.Targets {
		spec := target.Spec()
		for _, produced := range spec.Cache.Produces {
			if !hasInputOrResource(project, produced) {
				return fmt.Errorf(
					"target %q produces missing input/resource %q",
					spec.Name,
					produced,
				)
			}
			if owner, exists := project.Producers[produced]; exists {
				return fmt.Errorf("input %q produced by both %q and %q", produced, owner, spec.Name)
			}
			project.Producers[produced] = spec.Name
		}
	}
	for _, target := range project.Targets {
		spec := target.Spec()
		for _, input := range spec.Cache.Inputs {
			if producer, exists := project.Producers[input]; exists && producer != target.Name {
				target.DependsOn = appendUnique(target.DependsOn, producer)
			}
		}
	}
	for _, target := range project.Targets {
		spec := target.Spec()
		for _, dep := range target.DependsOn {
			if _, exists := project.Targets[dep]; !exists {
				return fmt.Errorf("target %q depends on missing target %q", spec.Name, dep)
			}
		}
		for _, child := range targetKind(target).CompositeChildrenByKind() {
			if _, exists := project.Targets[child.Target]; !exists {
				return fmt.Errorf(
					"target %q has missing %s %q",
					spec.Name,
					child.Kind,
					child.Target,
				)
			}
		}
	}
	if err := validateCompositeTargetCycles(project); err != nil {
		return err
	}
	if project.DefaultTarget != "" {
		project.DefaultTarget, _ = project.ResolveTargetName(project.DefaultTarget)
		if _, exists := project.Targets[project.DefaultTarget]; !exists {
			return fmt.Errorf("default target %q does not exist", project.DefaultTarget)
		}
	}
	return nil
}

func validateCompositeTargetCycles(project *Project) error {
	visiting := map[string]bool{}
	visited := map[string]bool{}
	var visit func(string) error
	visit = func(name string) error {
		if visiting[name] {
			return fmt.Errorf("composite target cycle includes %q", name)
		}
		if visited[name] {
			return nil
		}
		target := project.Targets[name]
		if target == nil {
			return nil
		}
		visiting[name] = true
		for _, child := range compositeChildren(target) {
			if err := visit(child); err != nil {
				return err
			}
		}
		visiting[name] = false
		visited[name] = true
		return nil
	}
	for name, target := range project.Targets {
		if len(compositeChildren(target)) > 0 {
			if err := visit(name); err != nil {
				return err
			}
		}
	}
	return nil
}

func compositeChildren(target *Target) []string {
	return targetKind(target).CompositeChildren()
}
