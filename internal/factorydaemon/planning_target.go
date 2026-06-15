package factorydaemon

import (
	"fmt"
	"path/filepath"

	"github.com/applauselab/bachkator/internal/backend"
	"github.com/applauselab/bachkator/internal/config"
	"github.com/applauselab/bachkator/internal/model"
)

func (s Service) materializePlanningTarget(
	item backend.FactoryWorkItem,
	workflow *config.FactoryWorkflow,
	planPath string,
) (string, string, *model.RunProject, error) {
	templateRef := canonicalTemplateRef(workflow.Plan[0].AgentTemplate)
	template := s.RuntimeProject.AgentTemplates[templateRef]
	if template == nil {
		return "", "", nil, fmt.Errorf(
			"unknown planning agent template %q",
			workflow.Plan[0].AgentTemplate,
		)
	}
	project := cloneProject(s.RuntimeProject)
	targetName := "agent/factory." + item.ID + ".plan"
	workspace := ".bach/agents/factory/" + item.ID + "/plan"
	branch := "bach/factory/" + item.ID + "/plan"
	project.Targets[targetName] = &model.RunTarget{
		Name: targetName,
		Env: []string{
			"BACH_FACTORY_NAME=" + s.Factory.Name,
			"BACH_WORKFLOW_NAME=" + workflow.Name,
			"BACH_WORK_ITEM_ID=" + item.ID,
			"BACH_WORK_ITEM_TITLE=" + item.Title,
			"BACH_PLAN_OUTPUT_PATH=" + planPath,
		},
		SpecValue: model.TargetSpec{
			Name: targetName,
			Metadata: model.TargetMetadata{
				Description:          "generated Factory planner for " + item.ID,
				Remote:               true,
				Destructive:          true,
				RequiresConfirmation: true,
			},
			Runtime: model.TargetRuntime{
				Lock: "factory:" + s.Factory.Name + ":" + item.ID + ":plan",
			},
			Body: model.AgentSpec{
				Mode:      "plan",
				Template:  templateRef,
				Provider:  template.Provider,
				Role:      template.Role,
				Prompt:    template.Prompt,
				Plan:      planPath,
				Workspace: model.AgentWorkspace{Mode: "clone", Path: workspace},
				Git:       model.AgentGit{Branch: branch},
			},
		},
	}
	return targetName, filepath.Join(
		s.ConfigProject.Root,
		filepath.FromSlash(workspace),
	), project, nil
}
