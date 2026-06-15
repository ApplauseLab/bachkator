package graph

import (
	"context"
	"fmt"
	"strings"

	"github.com/applauselab/bachkator/internal/model"
	targetpkg "github.com/applauselab/bachkator/internal/target"
)

type TargetHandlers interface {
	Handler(model.TargetType) (targetpkg.TargetHandler, error)
}

func BuildExplainRecord(project *Project, name string) (ExplainRecord, error) {
	return ExplainRecordWithHandlers(project, name, targetpkg.BuiltinTargetRegistry())
}

func ExplainRecordWithHandlers(
	project *Project,
	name string,
	targets TargetHandlers,
) (ExplainRecord, error) {
	canonicalName, alias := project.ResolveTargetName(name)
	name = canonicalName
	target := project.Targets[name]
	if target == nil {
		return ExplainRecord{}, fmt.Errorf("unknown target %q", name)
	}
	spec := target.Spec()
	risk, err := TargetRiskWithHandlers(project, name, targets)
	if err != nil {
		return ExplainRecord{}, err
	}
	record := ExplainRecord{
		Target:        spec.Name,
		Description:   spec.Metadata.Description,
		When:          spec.Metadata.When,
		Cost:          spec.Metadata.Cost,
		Risks:         risk.Labels(),
		DependsOn:     append([]string(nil), target.DependsOn...),
		Steps:         compositeChildrenOfKind(spec, targets, "pipeline_step"),
		Inputs:        append([]string(nil), spec.Cache.Inputs...),
		Outputs:       append([]string(nil), spec.Cache.Outputs...),
		Produces:      append([]string(nil), spec.Cache.Produces...),
		RequiredTools: requiredToolLabels(project, name),
		Preflights:    preflightLabels(project, name),
	}
	if alias != nil {
		record.Alias = alias.Name
		record.CanonicalTarget = alias.Target
		record.Deprecated = alias.Deprecated
	}
	return record, nil
}

func compositeChildrenOfKind(spec model.TargetSpec, targets TargetHandlers, kind string) []string {
	handler, err := targets.Handler(spec.TargetType())
	if err != nil {
		return nil
	}
	children := handler.CompositeChildren(spec.Body)
	out := make([]string, 0, len(children))
	for _, child := range children {
		if child.Kind == kind {
			out = append(out, child.Target)
		}
	}
	return out
}

func describeOperation(spec model.TargetSpec, targets TargetHandlers) string {
	handler, err := targets.Handler(spec.TargetType())
	if err != nil {
		return ""
	}
	description, err := handler.Describe(
		context.Background(),
		targetpkg.DescribeRequest{Spec: spec, Env: map[string]string{}},
	)
	if err != nil {
		return ""
	}
	return description.Operation
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
