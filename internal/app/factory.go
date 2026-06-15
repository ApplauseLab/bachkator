package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/applauselab/bachkator/internal/bacherr"
	"github.com/applauselab/bachkator/internal/backend"
	"github.com/applauselab/bachkator/internal/cli"
	"github.com/applauselab/bachkator/internal/config"
	factorypkg "github.com/applauselab/bachkator/internal/factory"
	"github.com/applauselab/bachkator/internal/factorydaemon"
	"github.com/applauselab/bachkator/internal/plan"
)

func (a App) submitFactory(
	ctx context.Context,
	project *cli.Project,
	factoryName string,
	opts cli.FactorySubmitOptions,
) (factorypkg.SubmitResult, error) {
	configProject, factoryConfig, workflow, err := a.resolveManualFactory(
		project,
		factoryName,
		opts.Workflow,
	)
	if err != nil {
		return factorypkg.SubmitResult{}, err
	}
	service := factoryService(configProject)
	return service.Submit(ctx, factorypkg.SubmitOptions{
		Factory:           factoryConfig.Name,
		Workflow:          workflow,
		Title:             opts.Title,
		Body:              opts.Body,
		BodyFile:          opts.BodyFile,
		Priority:          opts.Priority,
		Labels:            opts.Labels,
		DedupeKey:         opts.DedupeKey,
		SubmittedPlanPath: opts.Plan,
	})
}

func (a App) listFactory(
	ctx context.Context,
	project *cli.Project,
	factoryName string,
	opts cli.FactoryListOptions,
) ([]factorypkg.WorkItem, error) {
	configProject, factoryConfig, workflow, err := a.resolveFactoryWorkflow(
		project,
		factoryName,
		opts.Workflow,
		false,
	)
	if err != nil {
		return nil, err
	}
	service := factoryService(configProject)
	return service.List(ctx, factorypkg.ListOptions{
		Factory:  factoryConfig.Name,
		Workflow: workflow,
		Status:   opts.Status,
	})
}

func (a App) inspectFactory(
	ctx context.Context,
	project *cli.Project,
	factoryName string,
	id string,
) (factorypkg.WorkItem, error) {
	configProject, factoryConfig, _, err := a.resolveFactoryWorkflow(
		project,
		factoryName,
		"",
		false,
	)
	if err != nil {
		return factorypkg.WorkItem{}, err
	}
	_ = factoryConfig
	return factoryService(configProject).GetWithApprovals(ctx, factoryName, id)
}

func (a App) cancelFactory(
	ctx context.Context,
	project *cli.Project,
	factoryName string,
	id string,
	opts cli.FactoryCancelOptions,
) (factorypkg.WorkItem, error) {
	configProject, factoryConfig, _, err := a.resolveFactoryWorkflow(
		project,
		factoryName,
		"",
		false,
	)
	if err != nil {
		return factorypkg.WorkItem{}, err
	}
	return factoryService(configProject).Cancel(ctx, factorypkg.CancelOptions{
		Factory: factoryConfig.Name,
		ID:      id,
		Reason:  opts.Reason,
	})
}

func (a App) approveFactory(
	ctx context.Context,
	project *cli.Project,
	factoryName string,
	id string,
	opts cli.FactoryApproveOptions,
) (factorypkg.ApproveResult, error) {
	configProject, factoryConfig, _, err := a.resolveFactoryWorkflow(
		project,
		factoryName,
		"",
		false,
	)
	if err != nil {
		return factorypkg.ApproveResult{}, err
	}
	service := factoryService(configProject)
	item, err := service.Get(ctx, factoryConfig.Name, id)
	if err != nil {
		return factorypkg.ApproveResult{}, err
	}
	workflow, err := factoryConfig.ResolveWorkflow(item.Workflow)
	if err != nil {
		return factorypkg.ApproveResult{}, err
	}
	workflowConfig := factoryConfig.Workflows[0]
	for _, w := range factoryConfig.Workflows {
		if w != nil && w.Name == workflow {
			workflowConfig = w
			break
		}
	}
	if !workflowConfig.PhaseExists(opts.Phase) {
		return factorypkg.ApproveResult{}, bacherr.ValidationFailedf(
			"phase %q is not defined in workflow %q",
			opts.Phase,
			item.Workflow,
		)
	}
	planPath := ""
	planHash := ""
	switch opts.Phase {
	case config.FactoryPhasePlan:
		if !workflowConfig.PlanRequiresApproval() {
			return factorypkg.ApproveResult{}, bacherr.ValidationFailedf(
				"plan phase does not require approval in workflow %q",
				item.Workflow,
			)
		}
		planPath, planHash, err = factoryPlanApprovalEvidence(
			configProject.Root,
			factoryConfig.Name,
			workflowConfig.Name,
			item,
			workflowConfig.Plan[0].Path,
		)
		if err != nil {
			return factorypkg.ApproveResult{}, err
		}
	default:
		if strings.HasPrefix(opts.Phase, "deploy.") {
			name := strings.TrimPrefix(opts.Phase, "deploy.")
			if !workflowConfig.DeployRequiresApproval(name) {
				return factorypkg.ApproveResult{}, bacherr.ValidationFailedf(
					"deploy %q does not require approval in workflow %q",
					name,
					item.Workflow,
				)
			}
		} else {
			return factorypkg.ApproveResult{}, bacherr.ValidationFailedf(
				"phase %q cannot be approved",
				opts.Phase,
			)
		}
	}
	approver, source := resolveApproverIdentity()
	return service.Approve(ctx, factorypkg.ApproveOptions{
		Factory:        factoryConfig.Name,
		ID:             id,
		Phase:          opts.Phase,
		PlanPath:       planPath,
		PlanHash:       planHash,
		Reason:         opts.Reason,
		Approver:       approver,
		ApproverSource: source,
	})
}

func factoryPlanApprovalEvidence(
	root string,
	factory string,
	workflow string,
	item factorypkg.WorkItem,
	template string,
) (string, string, error) {
	planPath := strings.NewReplacer(
		"${work_item.id}", item.ID,
		"${work_item.slug}", item.ID,
		"${factory.name}", factory,
		"${workflow.name}", workflow,
	).Replace(template)
	absPath := filepath.Join(root, filepath.FromSlash(planPath))
	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", "", err
	}
	doc, diagnostics := plan.Parse(planPath, data)
	if len(diagnostics) > 0 {
		return "", "", bacherr.ValidationFailedf(
			"plan %q has %d diagnostics",
			planPath,
			len(diagnostics),
		)
	}
	return planPath, doc.Hash, nil
}

func resolveApproverIdentity() (string, string) {
	name := strings.TrimSpace(gitConfigValue("user.name"))
	email := strings.TrimSpace(gitConfigValue("user.email"))
	if name != "" {
		if email != "" {
			return fmt.Sprintf("%s <%s>", name, email), "git"
		}
		return name, "git"
	}
	if user := os.Getenv("USER"); user != "" {
		return user, "env"
	}
	if user, err := user.Current(); err == nil && user.Username != "" {
		return user.Username, "os_user"
	}
	return "", "unknown"
}

func gitConfigValue(key string) string {
	out, err := exec.Command("git", "config", key).Output()
	if err != nil {
		return ""
	}
	return string(out)
}

func (a App) startFactory(
	ctx context.Context,
	project *cli.Project,
	factoryName string,
	opts cli.FactoryStartOptions,
) (factorydaemon.StartResult, error) {
	configProject, factoryConfig, _, err := a.resolveFactoryWorkflow(
		project,
		factoryName,
		"",
		false,
	)
	if err != nil {
		return factorydaemon.StartResult{}, err
	}
	return a.factoryDaemonService(configProject, factoryConfig, opts).
		Start(ctx, factorydaemon.StartOptions{
			Yes:           opts.Yes,
			Force:         opts.Force,
			LogOnly:       opts.LogOnly,
			Verbose:       opts.Verbose,
			Parallelism:   opts.Parallelism,
			PollInterval:  opts.PollInterval,
			RenewInterval: opts.RenewInterval,
			LeaseTTL:      opts.LeaseTTL,
		})
}

func (a App) statusFactory(
	ctx context.Context,
	project *cli.Project,
	factoryName string,
) (factorydaemon.StatusResult, error) {
	configProject, factoryConfig, _, err := a.resolveFactoryWorkflow(
		project,
		factoryName,
		"",
		false,
	)
	if err != nil {
		return factorydaemon.StatusResult{}, err
	}
	return a.factoryDaemonService(configProject, factoryConfig, cli.FactoryStartOptions{}).
		Status(ctx)
}

func (a App) resolveManualFactory(
	project *cli.Project,
	factoryName string,
	requestedWorkflow string,
) (*config.Project, *config.Factory, string, error) {
	configProject, factoryConfig, workflow, err := a.resolveFactoryWorkflow(
		project,
		factoryName,
		requestedWorkflow,
		true,
	)
	if err != nil {
		return nil, nil, "", err
	}
	if !factoryConfig.ManualEnabled() {
		return nil, nil, "", fmt.Errorf("factory %q has no manual trigger", factoryName)
	}
	return configProject, factoryConfig, workflow, nil
}

func (a App) resolveFactoryWorkflow(
	project *cli.Project,
	factoryName string,
	requestedWorkflow string,
	resolveWorkflow bool,
) (*config.Project, *config.Factory, string, error) {
	configProject, err := a.configProject(project)
	if err != nil {
		return nil, nil, "", err
	}
	factoryConfig := configProject.Factories[factoryName]
	if factoryConfig == nil {
		return nil, nil, "", bacherr.NotFoundf("factory %q", factoryName)
	}
	if !resolveWorkflow && requestedWorkflow == "" {
		return configProject, factoryConfig, "", nil
	}
	workflow, err := factoryConfig.ResolveWorkflow(requestedWorkflow)
	if err != nil {
		return nil, nil, "", err
	}
	return configProject, factoryConfig, workflow, nil
}

func factoryService(project *config.Project) factorypkg.Service {
	runtimeProject := config.RuntimeProject(project)
	client := backend.NewProjectClient(project.Root, project.StatePath, runtimeProject.Backend)
	return factorypkg.Service{
		Root:  project.Root,
		Queue: factorypkg.BackendQueue{Client: &client.Factory},
	}
}

func (a App) factoryDaemonService(
	project *config.Project,
	factoryConfig *config.Factory,
	opts cli.FactoryStartOptions,
) factorydaemon.Service {
	runtimeProject := config.RuntimeProject(project)
	client := backend.NewProjectClient(project.Root, project.StatePath, runtimeProject.Backend)
	return factorydaemon.Service{
		ConfigProject:  project,
		RuntimeProject: runtimeProject,
		Factory:        factoryConfig,
		Backend:        client,
		Targets:        a.targetHandlers,
		Parsers:        a.qualityParsers,
		Gates:          a.qualityGates,
		Stdout:         opts.Stdout,
		Stderr:         opts.Stderr,
	}
}
