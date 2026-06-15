package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func registerInputs(project *Project, inputs []*Input) error {
	for _, input := range inputs {
		if input.Src != "" && len(input.Srcs) > 0 {
			return fmt.Errorf("input %q %q must use src or srcs, not both", input.Kind, input.Name)
		}
		key := input.Key()
		if _, exists := project.Inputs[key]; exists {
			return fmt.Errorf("duplicate input %q", key)
		}
		project.Inputs[key] = input
	}
	return nil
}

func registerResources(project *Project, resources []*Resource) error {
	for _, resource := range resources {
		key := resource.Key()
		if _, exists := project.Resources[key]; exists {
			return fmt.Errorf("duplicate resource %q", key)
		}
		project.Resources[key] = resource
	}
	return nil
}

func registerProviders(project *Project, providers []*Provider) error {
	for _, provider := range providers {
		switch provider.Type {
		case "agent":
			if len(provider.Command) == 0 {
				return fmt.Errorf("provider %q command must not be empty", provider.Name)
			}
		case "opencode":
			if len(provider.Command) != 0 {
				return fmt.Errorf(
					"provider %q command is not supported for opencode providers",
					provider.Name,
				)
			}
		default:
			return fmt.Errorf(
				"provider %q type must be %q or %q",
				provider.Name,
				"agent",
				"opencode",
			)
		}
		key := "provider/" + provider.Name
		if _, exists := project.Providers[key]; exists {
			return fmt.Errorf("duplicate provider %q", provider.Name)
		}
		project.Providers[key] = provider
	}
	return nil
}

func registerPrompts(project *Project, prompts []*Prompt) error {
	for _, prompt := range prompts {
		if prompt.Name == "" {
			return fmt.Errorf("prompt block must have a name")
		}
		if prompt.Path == "" {
			return fmt.Errorf("prompt %q path is required", prompt.Name)
		}
		if err := validateProjectRelativePath("prompt path", prompt.Path); err != nil {
			return fmt.Errorf("prompt %q path %q: %w", prompt.Name, prompt.Path, err)
		}
		key := prompt.Name
		if _, exists := project.Prompts[key]; exists {
			return fmt.Errorf("duplicate prompt %q", prompt.Name)
		}
		path := prompt.Path
		if !filepath.IsAbs(path) {
			path = filepath.Join(project.Root, path)
		}
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("prompt %q path %q: %w", prompt.Name, prompt.Path, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("prompt %q path %q must be a regular file", prompt.Name, prompt.Path)
		}
		root, err := filepath.EvalSymlinks(project.Root)
		if err != nil {
			return fmt.Errorf("project root %q: %w", project.Root, err)
		}
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			return fmt.Errorf("prompt %q path %q: %w", prompt.Name, prompt.Path, err)
		}
		rel, err := filepath.Rel(root, resolved)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf(
				"prompt %q path %q must stay within project root",
				prompt.Name,
				prompt.Path,
			)
		}
		project.Prompts[key] = prompt
	}
	return nil
}

func registerAliases(project *Project, aliases []*Alias) error {
	for _, alias := range aliases {
		canonical, err := canonicalTargetOrAliasRef(alias.Target)
		if err != nil {
			return fmt.Errorf("alias %q target: %w", alias.Name, err)
		}
		alias.Target = canonical
		if _, exists := project.Targets[alias.Name]; exists {
			return fmt.Errorf("alias %q conflicts with target", alias.Name)
		}
		if _, exists := project.Aliases[alias.Name]; exists {
			return fmt.Errorf("duplicate alias %q", alias.Name)
		}
		project.Aliases[alias.Name] = alias
	}
	for _, alias := range aliases {
		if _, exists := project.Aliases[alias.Target]; exists {
			return fmt.Errorf(
				"alias %q points to alias %q; alias chains are not supported",
				alias.Name,
				alias.Target,
			)
		}
		if _, exists := project.Targets[alias.Target]; !exists {
			return fmt.Errorf("alias %q points to unknown target %q", alias.Name, alias.Target)
		}
	}
	return nil
}

func canonicalPrimitiveRef(ref, kind string) string {
	ref = strings.ReplaceAll(ref, ".", "/")
	if strings.HasPrefix(ref, kind+"/") {
		return ref
	}
	return kind + "/" + ref
}

func appendUniqueString(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func validateProjectRelativePath(label, path string) error {
	if path == "" {
		return nil
	}
	cleaned := filepath.Clean(path)
	if filepath.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return fmt.Errorf("%s must stay under the project root", label)
	}
	return nil
}

func (p *Project) ResolveTargetName(name string) (string, *Alias) {
	if alias := p.Aliases[name]; alias != nil {
		return alias.Target, alias
	}
	if strings.Contains(name, ".") {
		canonical, err := canonicalTargetRef(name)
		if err == nil {
			return canonical, nil
		}
	}
	return name, nil
}
