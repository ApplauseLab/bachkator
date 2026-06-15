package runner

import (
	"slices"
	"testing"

	"github.com/applauselab/bachkator/internal/model"
)

func TestRunPlanDependencyClosureOrderAndEdges(t *testing.T) {
	project := &Project{Targets: map[string]*Target{
		"first": shellTarget("first", "printf first"),
		"all":   shellTarget("all", "", withDependsOn("first")),
	}}

	plan, err := BuildPlan(project, "all")
	if err != nil {
		t.Fatal(err)
	}
	assertStrings(t, plan.Order, []string{"first", "all"})
	assertEdges(t, plan.DependencyEdges, []PlanEdge{{From: "first", To: "all"}})
	assertEdges(t, plan.PipelineEdges, nil)
	assertStrings(t, plan.DependencyFingerprintOf["all"], []string{"first"})
}

func TestRunPlanIncludesPipelineStepsWithoutSchedulingThem(t *testing.T) {
	project := &Project{Targets: map[string]*Target{
		"render": shellTarget("render", "printf render"),
		"apply":  shellTarget("apply", "printf apply"),
		"deploy": pipelineTarget("deploy", []string{"render", "apply"}),
	}}

	plan, err := BuildPlan(project, "deploy")
	if err != nil {
		t.Fatal(err)
	}
	assertStrings(t, plan.Order, []string{"render", "apply", "deploy"})
	assertEdges(
		t,
		plan.PipelineEdges,
		[]PlanEdge{{From: "render", To: "deploy"}, {From: "apply", To: "deploy"}},
	)
	assertStrings(t, plan.ScheduledOrder(), []string{"deploy"})
}

func TestRunPlanKeepsDependencyAndPipelineEdgesDistinct(t *testing.T) {
	project := &Project{Targets: map[string]*Target{
		"setup":  shellTarget("setup", "printf setup"),
		"render": shellTarget("render", "printf render"),
		"deploy": pipelineTarget("deploy", []string{"render"}, withDependsOn("setup")),
	}}

	plan, err := BuildPlan(project, "deploy")
	if err != nil {
		t.Fatal(err)
	}
	assertEdges(t, plan.DependencyEdges, []PlanEdge{{From: "setup", To: "deploy"}})
	assertEdges(t, plan.PipelineEdges, []PlanEdge{{From: "render", To: "deploy"}})
	assertStrings(t, plan.ScheduledOrder(), []string{"setup", "deploy"})
}

func TestRunPlanDetectsCyclesAcrossPipelineSteps(t *testing.T) {
	project := &Project{Targets: map[string]*Target{
		"a": pipelineTarget("a", []string{"b"}),
		"b": shellTarget("b", "", withDependsOn("a")),
	}}

	_, err := BuildPlan(project, "a")
	if err == nil || err.Error() != `dependency cycle includes "a"` {
		t.Fatalf("error = %v, want cycle error", err)
	}
}

func TestRunPlanCollectsRiskToolsAndPreflightsFromDependenciesAndSteps(t *testing.T) {
	project := &Project{Targets: map[string]*Target{
		"setup": shellTarget(
			"setup",
			"printf setup",
			withRisk(true, false, false),
			withTool(ToolRequirement{Name: "kubectl"}),
			withPreflight(PreflightCheck{Name: "cloud", Command: []string{"sh", "-c", "true"}}),
		),
		"apply": shellTarget(
			"apply",
			"printf apply",
			withRisk(false, true, false),
			withTool(ToolRequirement{Name: "kubectl"}),
			withTool(ToolRequirement{Name: "aws", Fix: "login"}),
			withPreflight(
				PreflightCheck{
					Kind:    "session",
					Command: []string{"aws", "sts", "get-caller-identity"},
				},
			),
		),
		"deploy": pipelineTarget(
			"deploy",
			[]string{"apply"},
			withRisk(false, false, true),
			withDependsOn("setup"),
		),
	}}

	plan, err := BuildPlan(project, "deploy")
	if err != nil {
		t.Fatal(err)
	}
	if !plan.EffectiveRisk.Remote || !plan.EffectiveRisk.Destructive ||
		!plan.EffectiveRisk.RequiresConfirmation {
		t.Fatalf("risk = %#v, want all inherited risks", plan.EffectiveRisk)
	}
	if len(plan.Tools) != 2 || plan.Tools[0].Tool.Name != "kubectl" ||
		!slices.Equal(plan.Tools[0].Targets, []string{"apply", "setup"}) ||
		plan.Tools[1].Tool.Name != "aws" {
		t.Fatalf("tools = %#v", plan.Tools)
	}
	if len(plan.Preflights) != 2 || plan.Preflights[0].Preflight.Name != "cloud" ||
		plan.Preflights[1].Preflight.Kind != "session" {
		t.Fatalf("preflights = %#v", plan.Preflights)
	}
}

func TestRunPlanWorksWithResolvedAliases(t *testing.T) {
	project := &Project{
		Targets: map[string]*Target{"shell/test": shellTarget("shell/test", "go test ./...")},
		Aliases: map[string]*model.Alias{"test": {Name: "test", Target: "shell/test"}},
	}

	plan, err := BuildPlan(project, "test")
	if err != nil {
		t.Fatal(err)
	}
	if plan.TargetName != "shell/test" {
		t.Fatalf("target name = %q, want shell/test", plan.TargetName)
	}
}

func TestRunPlanForTargetsDeduplicatesSharedDependencies(t *testing.T) {
	project := &Project{Targets: map[string]*Target{
		"setup": shellTarget("setup", "printf setup"),
		"a":     shellTarget("a", "printf a", withDependsOn("setup")),
		"b":     shellTarget("b", "printf b", withDependsOn("setup")),
	}}

	plan, err := BuildPlanForTargets(project, []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	assertStrings(t, plan.RequestedTargets, []string{"a", "b"})
	if plan.TargetName != "a b" {
		t.Fatalf("target name = %q, want a b", plan.TargetName)
	}
	assertStrings(t, plan.Order, []string{"setup", "a", "b"})
	assertStrings(t, plan.ScheduledOrder(), []string{"setup", "a", "b"})
	assertEdges(
		t,
		plan.DependencyEdges,
		[]PlanEdge{{From: "setup", To: "a"}, {From: "setup", To: "b"}},
	)
}

func TestRunPlanForTargetsDeduplicatesRequestedAliases(t *testing.T) {
	project := &Project{
		Targets: map[string]*Target{
			"shell/test": shellTarget("shell/test", "go test ./..."),
		},
		Aliases: map[string]*model.Alias{
			"test": {Name: "test", Target: "shell/test"},
		},
	}

	plan, err := BuildPlanForTargets(project, []string{"test", "shell/test"})
	if err != nil {
		t.Fatal(err)
	}
	assertStrings(t, plan.RequestedTargets, []string{"shell/test"})
	assertStrings(t, plan.Order, []string{"shell/test"})
}

func assertStrings(t *testing.T, got []string, want []string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Fatalf("strings = %#v, want %#v", got, want)
	}
}

func assertEdges(t *testing.T, got []PlanEdge, want []PlanEdge) {
	t.Helper()
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !slices.Equal(got, want) {
		t.Fatalf("edges = %#v, want %#v", got, want)
	}
}
