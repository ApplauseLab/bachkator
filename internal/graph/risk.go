package graph

import (
	"fmt"

	targetpkg "github.com/applauselab/bachkator/internal/target"
)

func TargetRisk(project *Project, name string) (Risk, error) {
	return TargetRiskWithHandlers(project, name, targetpkg.BuiltinTargetRegistry())
}

func TargetRiskWithHandlers(project *Project, name string, targets TargetHandlers) (Risk, error) {
	return collectTargetRisk(project, name, targets, map[string]bool{}, map[string]Risk{})
}

func TargetRiskLabels(project *Project, name string) []string {
	risk, err := TargetRisk(project, name)
	if err != nil {
		target := project.Targets[name]
		if target == nil {
			return nil
		}
		return target.Spec().RiskLabels()
	}
	return risk.Labels()
}

func collectTargetRisk(
	project *Project,
	name string,
	targets TargetHandlers,
	visiting map[string]bool,
	memo map[string]Risk,
) (Risk, error) {
	if risk, ok := memo[name]; ok {
		return risk, nil
	}
	if visiting[name] {
		return Risk{}, fmt.Errorf("dependency cycle includes %q", name)
	}
	target, ok := project.Targets[name]
	if !ok {
		return Risk{}, fmt.Errorf("unknown target %q", name)
	}
	visiting[name] = true
	spec := target.Spec()
	risk := Risk{
		Remote:               spec.Metadata.Remote,
		Destructive:          spec.Metadata.Destructive,
		RequiresConfirmation: spec.Metadata.RequiresConfirmation,
	}
	children := append([]string{}, target.DependsOn...)
	children = append(children, compositeChildrenOfKind(spec, targets, "pipeline_step")...)
	for _, child := range children {
		childRisk, err := collectTargetRisk(project, child, targets, visiting, memo)
		if err != nil {
			return Risk{}, err
		}
		risk.Remote = risk.Remote || childRisk.Remote
		risk.Destructive = risk.Destructive || childRisk.Destructive
		risk.RequiresConfirmation = risk.RequiresConfirmation || childRisk.RequiresConfirmation
	}
	visiting[name] = false
	memo[name] = risk
	return risk, nil
}
