package graph

import (
	"path/filepath"
	"sort"
	"strings"
)

func AffectedTargets(project *Project, paths []string) []AffectedTarget {
	changed := normalizeAffectedPaths(paths)
	if len(changed) == 0 || project == nil {
		return nil
	}

	names := make([]string, 0, len(project.Targets))
	for name := range project.Targets {
		names = append(names, name)
	}
	sort.Strings(names)

	var affected []AffectedTarget
	for _, name := range names {
		target := project.Targets[name]
		matches := matchingInputs(resolveInputs(project, target.Spec().Cache.Inputs), changed)
		if len(matches) == 0 {
			continue
		}
		affected = append(affected, AffectedTarget{Name: name, Matches: matches})
	}
	return affected
}

func resolveInputs(project *Project, inputs []string) []string {
	return resolveInputsSeen(project, inputs, map[string]bool{})
}

func resolveInputsSeen(project *Project, inputs []string, seen map[string]bool) []string {
	var paths []string
	for _, input := range inputs {
		if named, ok := project.Inputs[input]; ok {
			if seen[input] {
				continue
			}
			seen[input] = true
			paths = append(paths, resolveInputsSeen(project, named.Paths(), seen)...)
			continue
		}
		if _, ok := project.Resources[input]; ok {
			continue
		}
		paths = append(paths, input)
	}
	return paths
}

func normalizeAffectedPaths(paths []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, path := range paths {
		path = strings.TrimSpace(filepath.ToSlash(filepath.Clean(path)))
		path = strings.TrimPrefix(path, "./")
		if path == "" || path == "." || seen[path] {
			continue
		}
		seen[path] = true
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}

func matchingInputs(inputs []string, changed []string) []string {
	seen := map[string]bool{}
	var matches []string
	for _, input := range normalizeAffectedInputs(inputs) {
		for _, path := range changed {
			if pathMatchesInput(path, input) {
				if !seen[input] {
					seen[input] = true
					matches = append(matches, input)
				}
				break
			}
		}
	}
	return matches
}

func normalizeAffectedInputs(inputs []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, input := range inputs {
		input = strings.TrimSpace(filepath.ToSlash(filepath.Clean(input)))
		input = strings.TrimPrefix(input, "./")
		if input == "" || seen[input] {
			continue
		}
		seen[input] = true
		out = append(out, input)
	}
	sort.Strings(out)
	return out
}

func pathMatchesInput(path string, input string) bool {
	if input == "." {
		return true
	}
	if path == input {
		return true
	}
	if strings.HasPrefix(path, input+"/") {
		return true
	}
	matched, err := filepath.Match(input, path)
	return err == nil && matched
}
