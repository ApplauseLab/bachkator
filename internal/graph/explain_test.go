package graph

import (
	"context"
	"testing"

	"github.com/applauselab/bachkator/internal/model"
	targetpkg "github.com/applauselab/bachkator/internal/target"
)

type fakeTargetHandlers struct {
	handler targetpkg.TargetHandler
}

func (f fakeTargetHandlers) Handler(model.TargetType) (targetpkg.TargetHandler, error) {
	return f.handler, nil
}

type fakeTargetHandler struct {
	operation  string
	childShell string
	children   []targetpkg.CompositeChild
}

func (fakeTargetHandler) Type() model.TargetType { return model.TargetTypeShell }

func (fakeTargetHandler) Runnable(model.TargetSpec) bool { return false }

func (h fakeTargetHandler) Describe(
	context.Context,
	targetpkg.DescribeRequest,
) (targetpkg.RunDescription, error) {
	return targetpkg.RunDescription{Operation: h.operation}, nil
}

func (fakeTargetHandler) Execute(context.Context, targetpkg.ExecuteRequest) error { return nil }

func (fakeTargetHandler) FingerprintParts(model.TargetBody) map[string]string { return nil }

func (h fakeTargetHandler) CompositeChildren(
	targetBody model.TargetBody,
) []targetpkg.CompositeChild {
	if h.childShell != "" {
		body, _ := targetBody.(model.ShellSpec)
		if body.Shell != h.childShell {
			return nil
		}
	}
	return h.children
}

func TestExplainBuildsTargetMetadataRecord(t *testing.T) {
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

	got, err := BuildExplainRecord(project, "shell/test-api")
	if err != nil {
		t.Fatal(err)
	}
	if got.Target != "shell/test-api" ||
		got.Description != "Run API tests" ||
		got.When != "after API edits" ||
		got.Cost != "medium" {
		t.Fatalf("record metadata = %#v", got)
	}
	if !sameStrings(got.Risks, []string{"remote", "requires_confirmation"}) {
		t.Fatalf("risks = %#v", got.Risks)
	}
	if !sameStrings(got.DependsOn, []string{"shell/install"}) ||
		!sameStrings(got.Inputs, []string{"packages/api/src"}) ||
		!sameStrings(got.Outputs, []string{"coverage/api.out"}) ||
		!sameStrings(got.RequiredTools, []string{"bun"}) ||
		!sameStrings(got.Preflights, []string{
			"package registry session via sh -c true - Refresh package registry auth.",
		}) {
		t.Fatalf("record lists = %#v", got)
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

	got, err := BuildExplainRecord(project, "pipeline/deploy")
	if err != nil {
		t.Fatal(err)
	}
	if !sameStrings(got.Risks, []string{"remote", "destructive", "requires_confirmation"}) {
		t.Fatalf("risks = %#v", got.Risks)
	}
}

func TestExplainWithHandlersUsesCompositeChildrenForStepsAndRisk(t *testing.T) {
	project := &model.RunProject{Targets: map[string]*model.RunTarget{
		"parent": {
			Name: "parent",
			SpecValue: model.TargetSpec{
				Name: "parent",
				Body: model.ShellSpec{Shell: "parent"},
			},
		},
		"child": {
			Name: "child",
			SpecValue: model.TargetSpec{
				Name: "child",
				Metadata: model.TargetMetadata{
					Remote: true,
				},
				Body: model.ShellSpec{Shell: "child"},
			},
		},
	}}

	got, err := ExplainRecordWithHandlers(
		project,
		"parent",
		fakeTargetHandlers{
			handler: fakeTargetHandler{
				childShell: "parent",
				children: []targetpkg.CompositeChild{
					{Target: "child", Kind: "pipeline_step"},
				},
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !sameStrings(got.Risks, []string{"remote"}) || !sameStrings(got.Steps, []string{"child"}) {
		t.Fatalf("record = %#v", got)
	}
}

func TestExplainRejectsUnknownTarget(t *testing.T) {
	_, err := BuildExplainRecord(
		&model.RunProject{Targets: map[string]*model.RunTarget{}},
		"shell/missing",
	)
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

	got, err := BuildExplainRecord(project, "old-test")
	if err != nil {
		t.Fatal(err)
	}
	if got.Alias != "old-test" ||
		got.CanonicalTarget != "shell/test" ||
		got.Deprecated != "Use shell/test." ||
		got.Target != "shell/test" {
		t.Fatalf("alias record = %#v", got)
	}
}

func sameStrings(got []string, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range got {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}
