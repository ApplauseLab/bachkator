package runner

import (
	"fmt"
	"sort"
	"strings"

	"github.com/applause/bachkator/internal/model"
)

type Plan struct {
	TargetName              string
	Targets                 map[string]*Target
	Order                   []string
	DependencyEdges         []PlanEdge
	PipelineEdges           []PlanEdge
	GroupEdges              []PlanEdge
	CompositeEdges          []PlanTypedEdge
	EffectiveRisk           PlannedRisk
	Tools                   []PlannedToolRequirement
	Preflights              []PlannedPreflightCheck
	DependencyFingerprintOf map[string][]string
}

type PlanEdge struct {
	From string
	To   string
}

type PlanTypedEdge struct {
	From string
	To   string
	Kind string
}

type PlannedRisk struct {
	Remote               bool
	Destructive          bool
	RequiresConfirmation bool
}

func (r PlannedRisk) Labels() []string {
	labels := []string{}
	if r.Remote {
		labels = append(labels, "remote")
	}
	if r.Destructive {
		labels = append(labels, "destructive")
	}
	if r.RequiresConfirmation {
		labels = append(labels, "requires_confirmation")
	}
	return labels
}

func BuildPlan(project *Project, name string) (*Plan, error) {
	canonicalName, _ := project.ResolveTargetName(name)
	planner := runPlanner{
		project:                 project,
		targets:                 map[string]*Target{},
		visiting:                map[string]bool{},
		dependencyFingerprintOf: map[string][]string{},
	}
	if err := planner.visit(canonicalName); err != nil {
		return nil, err
	}
	plan := &Plan{
		TargetName:              canonicalName,
		Targets:                 planner.targets,
		Order:                   append([]string(nil), planner.order...),
		DependencyFingerprintOf: planner.dependencyFingerprintOf,
	}
	plan.DependencyEdges = plan.edges(func(target *Target) []string { return target.DependsOn })
	plan.PipelineEdges = plan.edges(func(target *Target) []string {
		pipeline, _ := target.Spec().Body.(model.PipelineSpec)
		return pipeline.Steps
	})
	plan.GroupEdges = plan.edges(func(target *Target) []string {
		group, _ := target.Spec().Body.(model.GroupSpec)
		return group.Targets
	})
	plan.CompositeEdges = appendTypedEdges(plan.PipelineEdges, "pipeline_step")
	plan.CompositeEdges = append(
		plan.CompositeEdges,
		appendTypedEdges(plan.GroupEdges, "group_member")...,
	)
	plan.EffectiveRisk = plan.effectiveRisk()
	plan.Tools = plan.collectTools()
	plan.Preflights = plan.collectPreflights()
	return plan, nil
}

func (p *Plan) Target(name string) *Target {
	return p.Targets[name]
}

func (p *Plan) ScheduledTargets() map[string]*Target {
	// ScheduledTargets is the dependency-only closure used by legacy callers.
	// Runtime execution uses executionGraph so pipeline/group members and virtual
	// lifecycle vertices are represented explicitly.
	names := map[string]bool{}
	p.collectScheduledTarget(p.TargetName, names)
	targets := make(map[string]*Target, len(names))
	for name := range names {
		targets[name] = p.Targets[name]
	}
	return targets
}

func (p *Plan) ScheduledOrder() []string {
	targets := p.ScheduledTargets()
	order := make([]string, 0, len(targets))
	for _, name := range p.Order {
		if _, ok := targets[name]; ok {
			order = append(order, name)
		}
	}
	return order
}

func (p *Plan) collectScheduledTarget(name string, names map[string]bool) {
	if names[name] {
		return
	}
	names[name] = true
	for _, dep := range p.Targets[name].DependsOn {
		p.collectScheduledTarget(dep, names)
	}
}

func (p *Plan) DependencyFingerprints(
	targetName string,
	fingerprints map[string]string,
) map[string]string {
	inputs := map[string]string{}
	for _, dep := range p.DependencyFingerprintOf[targetName] {
		inputs[dep] = fingerprints[dep]
	}
	return inputs
}

type runPlanner struct {
	project                 *Project
	targets                 map[string]*Target
	order                   []string
	visiting                map[string]bool
	dependencyFingerprintOf map[string][]string
}

func (p *runPlanner) visit(name string) error {
	if _, ok := p.targets[name]; ok {
		return nil
	}
	if p.visiting[name] {
		return fmt.Errorf("dependency cycle includes %q", name)
	}
	target, ok := p.project.Targets[name]
	if !ok {
		return fmt.Errorf("unknown target %q", name)
	}
	p.visiting[name] = true
	for _, dep := range target.DependsOn {
		if err := p.visit(dep); err != nil {
			return err
		}
	}
	pipeline, _ := target.Spec().Body.(model.PipelineSpec)
	for _, step := range pipeline.Steps {
		if err := p.visit(step); err != nil {
			return err
		}
	}
	group, _ := target.Spec().Body.(model.GroupSpec)
	for _, member := range group.Targets {
		if err := p.visit(member); err != nil {
			return err
		}
	}
	p.visiting[name] = false
	p.targets[name] = target
	p.order = append(p.order, name)
	p.dependencyFingerprintOf[name] = append([]string(nil), target.DependsOn...)
	return nil
}

func (p *Plan) edges(children func(*Target) []string) []PlanEdge {
	orderIndex := p.orderIndex()
	edges := []PlanEdge{}
	for _, name := range p.Order {
		for _, child := range children(p.Targets[name]) {
			if _, ok := p.Targets[child]; ok {
				edges = append(edges, PlanEdge{From: child, To: name})
			}
		}
	}
	sort.SliceStable(edges, func(i, j int) bool {
		if orderIndex[edges[i].To] == orderIndex[edges[j].To] {
			return orderIndex[edges[i].From] < orderIndex[edges[j].From]
		}
		return orderIndex[edges[i].To] < orderIndex[edges[j].To]
	})
	return edges
}

func appendTypedEdges(edges []PlanEdge, kind string) []PlanTypedEdge {
	typed := make([]PlanTypedEdge, 0, len(edges))
	for _, edge := range edges {
		typed = append(typed, PlanTypedEdge{From: edge.From, To: edge.To, Kind: kind})
	}
	return typed
}

func (p *Plan) effectiveRisk() PlannedRisk {
	risk := PlannedRisk{}
	for _, name := range p.Order {
		spec := p.Targets[name].Spec()
		risk.Remote = risk.Remote || spec.Metadata.Remote
		risk.Destructive = risk.Destructive || spec.Metadata.Destructive
		risk.RequiresConfirmation = risk.RequiresConfirmation || spec.Metadata.RequiresConfirmation
	}
	return risk
}

func (p *Plan) collectTools() []PlannedToolRequirement {
	byKey := map[string]*PlannedToolRequirement{}
	keys := []string{}
	for _, name := range p.Order {
		for _, tool := range p.Targets[name].Spec().Runtime.Tools {
			key := toolKey(tool)
			entry := byKey[key]
			if entry == nil {
				entry = &PlannedToolRequirement{Tool: tool}
				byKey[key] = entry
				keys = append(keys, key)
			}
			entry.Targets = appendUniqueString(entry.Targets, name)
		}
	}
	tools := make([]PlannedToolRequirement, 0, len(keys))
	for _, key := range keys {
		entry := byKey[key]
		sort.Strings(entry.Targets)
		tools = append(tools, *entry)
	}
	return tools
}

func (p *Plan) collectPreflights() []PlannedPreflightCheck {
	byKey := map[string]*PlannedPreflightCheck{}
	keys := []string{}
	for _, name := range p.Order {
		for _, check := range p.Targets[name].Spec().Runtime.Preflights {
			key := preflightKey(check)
			entry := byKey[key]
			if entry == nil {
				entry = &PlannedPreflightCheck{Preflight: check}
				byKey[key] = entry
				keys = append(keys, key)
			}
			entry.Targets = appendUniqueString(entry.Targets, name)
		}
	}
	preflights := make([]PlannedPreflightCheck, 0, len(keys))
	for _, key := range keys {
		entry := byKey[key]
		sort.Strings(entry.Targets)
		preflights = append(preflights, *entry)
	}
	return preflights
}

func (p *Plan) orderIndex() map[string]int {
	orderIndex := make(map[string]int, len(p.Order))
	for index, name := range p.Order {
		orderIndex[name] = index
	}
	return orderIndex
}

func toolKey(tool model.ToolRequirement) string {
	return tool.Name + "\x00" + strings.Join(
		tool.Command,
		"\x00",
	) + "\x00" + tool.Version + "\x00" + tool.Fix
}

func preflightKey(preflight model.PreflightCheck) string {
	return preflight.Name + "\x00" + preflight.Kind + "\x00" + strings.Join(
		preflight.Command,
		"\x00",
	) + "\x00" + preflight.Fix
}

func appendUniqueString(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
