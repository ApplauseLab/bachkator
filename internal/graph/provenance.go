package graph

import (
	"path/filepath"
	"sort"
	"strings"

	targetpkg "github.com/applauselab/bachkator/internal/target"
)

type PathProvenance struct {
	Path      string
	Generated bool
	Source    bool
	Producers []ProvenanceTarget
	Consumers []ProvenanceTarget
	Status    string
	Reasons   []string
}

type ProvenanceTarget struct {
	Target            string
	Operation         string
	RegenerateCommand string
	Outputs           []string
	Inputs            []string
}

func Provenance(project *Project, paths []string) []PathProvenance {
	return ProvenanceWithHandlers(project, paths, targetpkg.BuiltinTargetRegistry())
}

func ProvenanceWithHandlers(
	project *Project,
	paths []string,
	targets TargetHandlers,
) []PathProvenance {
	if project == nil {
		return nil
	}

	index := newProvenanceIndex(project, targets)
	records := make([]PathProvenance, 0, len(paths))
	for _, path := range paths {
		queryPath := normalizeProvenancePath(project.Root, path)
		if queryPath == "" {
			continue
		}
		producers := index.producers(queryPath)
		consumers := index.consumers(queryPath)
		records = append(records, PathProvenance{
			Path:      queryPath,
			Generated: len(producers) > 0,
			Source:    len(consumers) > 0 && len(producers) == 0,
			Producers: producers,
			Consumers: consumers,
			Status:    "unknown",
		})
	}
	return records
}

type provenanceIndex struct {
	producersByTarget map[string]ProvenanceTarget
	consumerByTarget  map[string]ProvenanceTarget
}

func newProvenanceIndex(project *Project, targets TargetHandlers) provenanceIndex {
	names := make([]string, 0, len(project.Targets))
	for name := range project.Targets {
		names = append(names, name)
	}
	sort.Strings(names)

	index := provenanceIndex{
		producersByTarget: map[string]ProvenanceTarget{},
		consumerByTarget:  map[string]ProvenanceTarget{},
	}
	for _, name := range names {
		target := project.Targets[name]
		if target == nil {
			continue
		}
		spec := target.Spec()
		inputs := normalizeAffectedInputs(resolveInputs(project, spec.Cache.Inputs))
		outputs := normalizeAffectedInputs(spec.Cache.Outputs)
		entry := ProvenanceTarget{
			Target:            name,
			Operation:         describeOperation(spec, targets),
			RegenerateCommand: "bach run " + name,
			Outputs:           outputs,
			Inputs:            inputs,
		}
		if len(outputs) > 0 {
			index.producersByTarget[name] = entry
		}
		if len(inputs) > 0 {
			index.consumerByTarget[name] = entry
		}
	}
	return index
}

func (i provenanceIndex) producers(path string) []ProvenanceTarget {
	return i.matches(
		path,
		i.producersByTarget,
		func(target ProvenanceTarget) []string { return target.Outputs },
	)
}

func (i provenanceIndex) consumers(path string) []ProvenanceTarget {
	return i.matches(
		path,
		i.consumerByTarget,
		func(target ProvenanceTarget) []string { return target.Inputs },
	)
}

func (i provenanceIndex) matches(
	path string,
	targets map[string]ProvenanceTarget,
	paths func(ProvenanceTarget) []string,
) []ProvenanceTarget {
	names := make([]string, 0, len(targets))
	for name := range targets {
		names = append(names, name)
	}
	sort.Strings(names)

	var matches []ProvenanceTarget
	for _, name := range names {
		target := targets[name]
		for _, candidate := range paths(target) {
			if filepath.IsAbs(path) && !filepath.IsAbs(candidate) {
				continue
			}
			if pathMatchesInput(path, candidate) {
				matches = append(matches, target)
				break
			}
		}
	}
	return matches
}

func normalizeProvenancePath(root string, path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if filepath.IsAbs(path) && root != "" {
		if rel, err := filepath.Rel(root, path); err == nil && rel != ".." &&
			!strings.HasPrefix(rel, "../") {
			path = rel
		}
	}
	path = filepath.ToSlash(filepath.Clean(path))
	path = strings.TrimPrefix(path, "./")
	if path == "." {
		return ""
	}
	return path
}
