package planexecute

import (
	"fmt"
	"strings"

	"github.com/applauselab/bachkator/internal/model"
	"github.com/applauselab/bachkator/internal/plan"
)

func materializeProject(
	project *model.RunProject,
	doc plan.Document,
) (*model.RunProject, string, string, error) {
	if project == nil {
		return nil, "", "", fmt.Errorf("project is required")
	}
	templateKey := canonicalRef(doc.AgentTemplate, "agent_template")
	template := project.AgentTemplates[templateKey]
	if template == nil {
		return nil, "", "", fmt.Errorf("unknown agent template %q", doc.AgentTemplate)
	}
	if template.Mode != "" && template.Mode != "implement" {
		return nil, "", "", fmt.Errorf(
			"agent template %q must be implement mode",
			doc.AgentTemplate,
		)
	}
	targetName := generatedTargetName(doc)
	generated := cloneProject(project)
	if _, exists := generated.Targets[targetName]; exists {
		return nil, "", "", fmt.Errorf(
			"generated target %q conflicts with an existing target",
			targetName,
		)
	}
	workspace := interpolatePlan(template.Workspace.Path, doc)
	if workspace == "" {
		workspace = ".bach/agents/plans/" + doc.ID
	}
	branch := interpolatePlan(template.Git.Branch, doc)
	if branch == "" {
		branch = "bach/plans/" + doc.ID
	}
	commit := template.Git.Commit
	if commit == "" {
		commit = "required"
	}
	policy := template.Policy
	if doc.Policy != "" {
		policyRef := canonicalRef(doc.Policy, "policy")
		policyConfig := project.Policies[policyRef]
		if policyConfig == nil {
			return nil, "", "", fmt.Errorf("unknown policy %q", doc.Policy)
		}
		policy = *policyConfig
	}
	generated.Targets[targetName] = &model.RunTarget{
		Name: targetName,
		Env: []string{
			"BACH_PLAN_ID=" + doc.ID,
			"BACH_PLAN_TITLE=" + doc.Title,
			"BACH_PLAN_PATH=" + doc.Path,
			"BACH_PLAN_HASH=" + doc.Hash,
			"BACH_PLAN_DEPENDS_ON=" + strings.Join(doc.DependsOn, ","),
			"BACH_PLAN_REQUIRED_TARGETS=" + strings.Join(doc.RequiredTargets, ","),
		},
		SpecValue: model.TargetSpec{
			Name: targetName,
			Metadata: model.TargetMetadata{
				Description:          "generated Plan implementer for " + doc.ID,
				Remote:               true,
				Destructive:          true,
				RequiresConfirmation: true,
			},
			Runtime: model.TargetRuntime{Lock: "plan:" + doc.ID, Env: []string{}},
			Body: model.AgentSpec{
				Mode:      "implement",
				Template:  templateKey,
				Provider:  template.Provider,
				Role:      template.Role,
				Prompt:    template.Prompt,
				Plan:      doc.Path,
				Policy:    policy,
				Workspace: model.AgentWorkspace{Mode: "clone", Path: workspace},
				Git:       model.AgentGit{Branch: branch, Commit: commit},
			},
		},
	}
	if policy.Name != "" {
		policyName := model.GeneratedPolicyTargetAddress(policy.Name, targetName).LegacyName()
		generated.Targets[policyName] = generatedPolicyTarget(
			policyName,
			targetName,
			workspace,
			branch,
			doc.Path,
			policy,
		)
	}
	return generated, targetName, templateKey, nil
}

func generatedPolicyTarget(
	name string,
	subject string,
	workspace string,
	branch string,
	planPath string,
	policy model.Policy,
) *model.RunTarget {
	return &model.RunTarget{
		Name: name,
		SpecValue: model.TargetSpec{
			Name: name,
			Metadata: model.TargetMetadata{
				Description: "generated policy fan-out for " + subject,
			},
			Quality: model.TargetQuality{
				Reports: []model.QualityReportDeclaration{{
					Kind:   "policy",
					Format: "agent-report-json",
					Path:   "$BACH_RUN_DIRECTORY/policy-report.json",
				}},
				Gates: policy.Gates,
			},
			Body: model.PolicySpec{
				Policy: policy,
				Subject: model.AgentSubject{
					Target:       subject,
					Workspace:    workspace,
					Branch:       branch,
					Plan:         planPath,
					PolicyTarget: name,
				},
			},
		},
	}
}

func cloneProject(project *model.RunProject) *model.RunProject {
	clone := *project
	clone.Targets = make(map[string]*model.RunTarget, len(project.Targets)+2)
	for key, target := range project.Targets {
		clone.Targets[key] = target
	}
	return &clone
}

func canonicalRef(value string, prefix string) string {
	if strings.HasPrefix(value, prefix+"/") {
		return value
	}
	return prefix + "/" + strings.TrimPrefix(value, prefix+".")
}

func interpolatePlan(value string, doc plan.Document) string {
	value = strings.ReplaceAll(value, "${plan.id}", doc.ID)
	value = strings.ReplaceAll(value, "${work_item.id}", "plan-"+doc.ID)
	value = strings.ReplaceAll(value, "${work_item.slug}", doc.ID)
	value = strings.ReplaceAll(value, "${workflow.name}", "plan")
	value = strings.ReplaceAll(value, "${factory.name}", "plan")
	value = strings.ReplaceAll(value, "${workstream.id}", doc.ID)
	return value
}
