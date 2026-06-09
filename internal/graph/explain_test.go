package graph

import (
	"strings"
	"testing"

	"github.com/applause/bachkator/internal/model"
)

func TestExplainFormatsTargetMetadata(t *testing.T) {
	project := &model.RunProject{Targets: map[string]*model.RunTarget{
		"shell/install": {
			Name:      "shell/install",
			SpecValue: model.TargetSpec{Name: "shell/install", Body: model.ShellSpec{}},
		},
		"shell/test-api": {
			Name:      "shell/test-api",
			DependsOn: []string{"shell/install"},
			SpecValue: model.TargetSpec{
				Name: "shell/test-api",
				Metadata: model.TargetMetadata{
					Description:          "Run API tests",
					When:                 "after API edits",
					Cost:                 "medium",
					Remote:               true,
					RequiresConfirmation: true,
				},
				Runtime: model.TargetRuntime{
					Tools: []model.ToolRequirement{{Name: "bun"}},
					Preflights: []model.PreflightCheck{
						{
							Name:    "package registry session",
							Command: []string{"sh", "-c", "true"},
							Fix:     "Refresh package registry auth.",
						},
					},
				},
				Cache: model.TargetCache{
					Inputs:  []string{"packages/api/src"},
					Outputs: []string{"coverage/api.out"},
				},
				Body: model.ShellSpec{},
			},
		},
	}}

	got, err := Explain(project, "shell/test-api")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Target: shell/test-api",
		"Description: Run API tests",
		"When: after API edits",
		"Cost: medium",
		"Risks: remote, requires_confirmation",
		"  - shell/install",
		"  - packages/api/src",
		"  - coverage/api.out",
		"Required tools:\n  - bun",
		"Preflights:\n  - package registry session via sh -c true - Refresh package registry auth.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("explanation missing %q:\n%s", want, got)
		}
	}
}

func TestExplainIncludesInheritedRisk(t *testing.T) {
	project := &model.RunProject{Targets: map[string]*model.RunTarget{
		"shell/apply": {
			Name: "shell/apply",
			SpecValue: model.TargetSpec{
				Name: "shell/apply",
				Metadata: model.TargetMetadata{
					Remote:               true,
					Destructive:          true,
					RequiresConfirmation: true,
				},
				Body: model.ShellSpec{},
			},
		},
		"pipeline/deploy": {
			Name: "pipeline/deploy",
			SpecValue: model.TargetSpec{
				Name: "pipeline/deploy",
				Body: model.PipelineSpec{Steps: []string{"shell/apply"}},
			},
		},
	}}

	got, err := Explain(project, "pipeline/deploy")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "Risks: remote, destructive, requires_confirmation") {
		t.Fatalf("explanation missing inherited risks:\n%s", got)
	}
}

func TestExplainRejectsUnknownTarget(t *testing.T) {
	_, err := Explain(&model.RunProject{Targets: map[string]*model.RunTarget{}}, "shell/missing")
	if err == nil {
		t.Fatal("expected unknown target error")
	}
}

func TestExplainResolvesAlias(t *testing.T) {
	project := &model.RunProject{
		Targets: map[string]*model.RunTarget{
			"shell/test": {
				Name:      "shell/test",
				SpecValue: model.TargetSpec{Name: "shell/test", Body: model.ShellSpec{}},
			},
		},
		Aliases: map[string]*model.Alias{
			"old-test": {Name: "old-test", Target: "shell/test", Deprecated: "Use shell/test."},
		},
	}

	got, err := Explain(project, "old-test")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Alias: old-test", "Canonical target: shell/test", "Deprecated: Use shell/test.", "Target: shell/test"} {
		if !strings.Contains(got, want) {
			t.Fatalf("explanation missing %q:\n%s", want, got)
		}
	}
}
