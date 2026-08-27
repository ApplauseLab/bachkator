package factorydaemon

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/applauselab/bachkator/internal/backend"
	"github.com/applauselab/bachkator/internal/config"
	"github.com/applauselab/bachkator/internal/model"
)

// The planner output path must be absolute and workspace-scoped so providers
// cannot resolve it against the main checkout (BACH_PROJECT_ROOT) and trip the
// provider-hygiene check.
func TestMaterializePlanningTargetScopesPlanOutputToWorkspace(t *testing.T) {
	root := t.TempDir()
	s := Service{
		ConfigProject: &config.Project{Root: root},
		RuntimeProject: &model.RunProject{
			AgentTemplates: map[string]*model.AgentTemplate{
				"agent_template/planner": {},
			},
		},
		Factory: &config.Factory{Name: "todo"},
	}
	item := backend.FactoryWorkItem{
		ID:       "01jt0000000000000000000000",
		Factory:  "todo",
		Workflow: "ship",
		Title:    "Ship billing webhook",
	}
	workflow := &config.FactoryWorkflow{
		Name: "ship",
		Plan: []*config.FactoryPlanPhase{{AgentTemplate: "agent_template.planner"}},
	}

	name, workspace, project, err := s.materializePlanningTarget(
		item, workflow, "plans/factory/"+item.ID+".md",
	)
	if err != nil {
		t.Fatalf("materializePlanningTarget: %v", err)
	}
	if name != "agent/factory."+item.ID+".plan" {
		t.Fatalf("unexpected target name %q", name)
	}

	target := project.Targets[name]
	if target == nil {
		t.Fatalf("target %q not materialized", name)
	}

	var output string
	for _, entry := range target.Env {
		if value, ok := strings.CutPrefix(entry, "BACH_PLAN_OUTPUT_PATH="); ok {
			output = value
		}
	}
	if output == "" {
		t.Fatal("BACH_PLAN_OUTPUT_PATH missing from planning target env")
	}

	wantDir := workspace
	if !filepath.IsAbs(wantDir) {
		t.Fatalf("materializePlanningTarget returned relative workspace %q", wantDir)
	}
	if !strings.HasPrefix(output, wantDir+string(filepath.Separator)) {
		t.Fatalf("plan output %q escapes planner workspace %q", output, wantDir)
	}
	if !filepath.IsAbs(output) {
		t.Fatalf("plan output %q is not absolute", output)
	}
}
