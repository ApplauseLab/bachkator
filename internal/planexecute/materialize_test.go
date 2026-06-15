package planexecute

import (
	"testing"

	"github.com/applauselab/bachkator/internal/model"
	"github.com/applauselab/bachkator/internal/plan"
)

func TestMaterializeProjectCreatesGeneratedAgentTarget(t *testing.T) {
	project := &model.RunProject{
		AgentTemplates: map[string]*model.AgentTemplate{
			"agent_template/feature": {
				Name: "feature",
				Mode: "implement",
				Provider: model.Provider{
					Name:    "fixture",
					Type:    "agent",
					Command: []string{"provider.sh"},
				},
				Prompt: model.Prompt{Name: "implementer", Path: "prompts/implementer.md"},
				Role:   "implementer",
				Workspace: model.AgentWorkspace{
					Mode: "clone",
					Path: ".bach/agents/plans/${plan.id}",
				},
				Git: model.AgentGit{Branch: "bach/plans/${plan.id}", Commit: "required"},
			},
		},
		Targets: map[string]*model.RunTarget{},
	}
	doc := plan.Document{
		Path:          "plans/feature.md",
		ID:            "feature",
		Title:         "Feature",
		Hash:          "sha256:feature",
		AgentTemplate: "feature",
	}
	generated, targetName, templateName, err := materializeProject(project, doc)
	if err != nil {
		t.Fatal(err)
	}
	if targetName != "agent/plan.feature" || templateName != "agent_template/feature" {
		t.Fatalf("target=%q template=%q", targetName, templateName)
	}
	target := generated.Targets[targetName]
	if target == nil {
		t.Fatalf("generated targets = %#v", generated.Targets)
	}
	agent, ok := target.SpecValue.Body.(model.AgentSpec)
	if !ok {
		t.Fatalf("body = %T", target.SpecValue.Body)
	}
	if agent.Plan != doc.Path || agent.Workspace.Path != ".bach/agents/plans/feature" ||
		agent.Git.Branch != "bach/plans/feature" {
		t.Fatalf("agent = %#v", agent)
	}
	if target.SpecValue.Runtime.Lock != "plan:feature" {
		t.Fatalf("lock = %q", target.SpecValue.Runtime.Lock)
	}
	if !target.SpecValue.Metadata.Remote || !target.SpecValue.Metadata.Destructive ||
		!target.SpecValue.Metadata.RequiresConfirmation {
		t.Fatalf("generated target must require confirmation: %#v", target.SpecValue.Metadata)
	}
}

func TestMaterializeProjectRejectsGeneratedTargetCollision(t *testing.T) {
	project := &model.RunProject{
		AgentTemplates: map[string]*model.AgentTemplate{
			"agent_template/feature": {Name: "feature"},
		},
		Targets: map[string]*model.RunTarget{"agent/plan.feature": {Name: "agent/plan.feature"}},
	}
	doc := plan.Document{Path: "plans/feature.md", ID: "feature", AgentTemplate: "feature"}
	_, _, _, err := materializeProject(project, doc)
	if err == nil {
		t.Fatal("expected collision error")
	}
}
