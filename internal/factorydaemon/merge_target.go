package factorydaemon

import (
	"context"
	"fmt"

	"github.com/applauselab/bachkator/internal/backend"
	"github.com/applauselab/bachkator/internal/config"
	"github.com/applauselab/bachkator/internal/model"
)

// ensureMergeApproval gates the transition out of the merge lane: the item
// waits until an approval record for the merge phase is recorded (externally
// or via `bach factory approve`).
func (s Service) ensureMergeApproval(
	ctx context.Context,
	item backend.FactoryWorkItem,
	attemptID string,
) error {
	_, ok, err := s.findApproval(ctx, item.ID, attemptID, config.FactoryPhaseMerge)
	if err != nil {
		return s.failItem(ctx, item, config.FactoryPhaseMerge, err)
	}
	if ok {
		return nil
	}
	return s.waitForApproval(ctx, item, attemptID, config.FactoryPhaseMerge, "", "")
}

// ensureImplementApproval gates the transition from implement to merge.
func (s Service) ensureImplementApproval(
	ctx context.Context,
	item backend.FactoryWorkItem,
	attemptID string,
	phaseKey string,
) error {
	_, ok, err := s.findApproval(ctx, item.ID, attemptID, phaseKey)
	if err != nil {
		return s.failItem(ctx, item, phaseKey, err)
	}
	if ok {
		return nil
	}
	return s.waitForApproval(ctx, item, attemptID, phaseKey, "", "")
}

// runMergeAgentPhase executes the merge lane through an agent template
// (mode implement) so agents — not scripts — do the integration work:
// pushing the work branch, publishing artifacts, and commenting on the PR.
func (s Service) runMergeAgentPhase(
	ctx context.Context,
	opts StartOptions,
	item backend.FactoryWorkItem,
	attemptID string,
	workflow *config.FactoryWorkflow,
	merge *config.FactoryTargetPhase,
) error {
	planPath := interpolate(workflow.Plan[0].Path, item, s.Factory.Name, workflow.Name)
	templateRef := canonicalTemplateRef(merge.AgentTemplate)
	template := s.RuntimeProject.AgentTemplates[templateRef]
	if template == nil {
		return fmt.Errorf("unknown merge agent template %q", merge.AgentTemplate)
	}
	project := cloneProject(s.RuntimeProject)
	targetName := "agent/factory." + item.ID + ".merge"
	workspace := ".bach/agents/factory/" + item.ID + "/merge"
	branch := "bach/factory/" + item.ID + "/merge"
	project.Targets[targetName] = &model.RunTarget{
		Name: targetName,
		Env: []string{
			"BACH_FACTORY_NAME=" + s.Factory.Name,
			"BACH_WORKFLOW_NAME=" + workflow.Name,
			"BACH_WORK_ITEM_ID=" + item.ID,
			"BACH_WORK_ITEM_TITLE=" + item.Title,
			"BACH_PLAN_PATH=" + planPath,
		},
		SpecValue: model.TargetSpec{
			Name: targetName,
			Metadata: model.TargetMetadata{
				Description:          "generated Factory merger for " + item.ID,
				Remote:               true,
				Destructive:          true,
				RequiresConfirmation: true,
			},
			Runtime: model.TargetRuntime{
				Lock: "factory:" + s.Factory.Name + ":" + item.ID + ":merge",
			},
			Body: model.AgentSpec{
				Mode:      "implement",
				Template:  templateRef,
				Provider:  template.Provider,
				Role:      template.Role,
				Prompt:    template.Prompt,
				Plan:      planPath,
				Workspace: model.AgentWorkspace{Mode: "clone", Path: workspace},
				Git:       model.AgentGit{Branch: branch, Commit: "optional"},
			},
		},
	}
	return s.runner(opts).RunTargets(ctx, project, []string{targetName})
}
