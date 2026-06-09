package graph

import (
	"fmt"
	"strings"

	"github.com/applause/bachkator/internal/model"
)

func Explain(project *Project, name string) (string, error) {
	canonicalName, alias := project.ResolveTargetName(name)
	name = canonicalName
	target := project.Targets[name]
	if target == nil {
		return "", fmt.Errorf("unknown target %q", name)
	}
	spec := target.Spec()
	risk, err := TargetRisk(project, name)
	if err != nil {
		return "", err
	}

	var out strings.Builder
	if alias != nil {
		writeField(&out, "Alias", alias.Name)
		writeField(&out, "Canonical target", alias.Target)
		writeField(&out, "Deprecated", alias.Deprecated)
	}
	writeField(&out, "Target", spec.Name)
	writeField(&out, "Description", spec.Metadata.Description)
	writeField(&out, "When", spec.Metadata.When)
	writeField(&out, "Cost", spec.Metadata.Cost)
	writeField(&out, "Risks", strings.Join(risk.Labels(), ", "))
	writeList(&out, "Depends on", target.DependsOn)
	pipeline, _ := spec.Body.(model.PipelineSpec)
	writeList(&out, "Steps", pipeline.Steps)
	writeList(&out, "Inputs", spec.Cache.Inputs)
	writeList(&out, "Outputs", spec.Cache.Outputs)
	writeList(&out, "Produces", spec.Cache.Produces)
	writeList(&out, "Required tools", requiredToolLabels(project, name))
	writeList(&out, "Preflights", preflightLabels(project, name))
	return out.String(), nil
}

func requiredToolLabels(project *Project, name string) []string {
	target := project.Targets[name]
	if target == nil {
		return nil
	}
	spec := target.Spec()
	labels := make([]string, 0, len(spec.Runtime.Tools))
	for _, tool := range spec.Runtime.Tools {
		labels = append(labels, formatToolRequirement(tool))
	}
	return labels
}

func formatToolRequirement(tool model.ToolRequirement) string {
	label := tool.Name
	if tool.Version != "" {
		label += " (" + tool.Version + ")"
	}
	if len(tool.Command) > 0 {
		label += " via " + strings.Join(tool.Command, " ")
	}
	if tool.Fix != "" {
		label += " - " + tool.Fix
	}
	return label
}

func preflightLabels(project *Project, name string) []string {
	target := project.Targets[name]
	if target == nil {
		return nil
	}
	spec := target.Spec()
	labels := make([]string, 0, len(spec.Runtime.Preflights))
	for _, preflight := range spec.Runtime.Preflights {
		label := preflight.Label() + " via " + strings.Join(preflight.Command, " ")
		if preflight.Fix != "" {
			label += " - " + preflight.Fix
		}
		labels = append(labels, label)
	}
	return labels
}

func writeField(out *strings.Builder, label string, value string) {
	if value == "" {
		value = "-"
	}
	fmt.Fprintf(out, "%s: %s\n", label, value)
}

func writeList(out *strings.Builder, label string, values []string) {
	if len(values) == 0 {
		writeField(out, label, "")
		return
	}
	fmt.Fprintf(out, "%s:\n", label)
	for _, value := range values {
		fmt.Fprintf(out, "  - %s\n", value)
	}
}
