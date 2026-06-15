package config

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/applauselab/bachkator/internal/bacherr"
)

const (
	factoryIdentifierPatternText = `^[A-Za-z0-9_][A-Za-z0-9_.-]*$`
)

var factoryIdentifierPattern = regexp.MustCompile(factoryIdentifierPatternText)

func registerFactories(project *Project, factories []*Factory) error {
	for _, factory := range factories {
		if err := validateFactory(factory); err != nil {
			return err
		}
		if _, exists := project.Factories[factory.Name]; exists {
			return fmt.Errorf("duplicate factory %q", factory.Name)
		}
		project.Factories[factory.Name] = factory
	}
	return nil
}

func validateFactory(factory *Factory) error {
	if factory == nil {
		return fmt.Errorf("factory block is nil")
	}
	if !factoryIdentifierPattern.MatchString(factory.Name) {
		return fmt.Errorf("factory %q name must be a simple identifier", factory.Name)
	}
	if len(factory.Workflows) == 0 {
		return fmt.Errorf("factory %q must declare at least one workflow", factory.Name)
	}
	workflows := map[string]struct{}{}
	for _, workflow := range factory.Workflows {
		if workflow == nil {
			return fmt.Errorf("factory %q workflow block is nil", factory.Name)
		}
		if !factoryIdentifierPattern.MatchString(workflow.Name) {
			return fmt.Errorf(
				"factory %q workflow %q name must be a simple identifier",
				factory.Name,
				workflow.Name,
			)
		}
		if _, exists := workflows[workflow.Name]; exists {
			return fmt.Errorf("factory %q has duplicate workflow %q", factory.Name, workflow.Name)
		}
		if err := validateFactoryWorkflow(factory.Name, workflow); err != nil {
			return err
		}
		workflows[workflow.Name] = struct{}{}
	}
	if len(factory.Triggers) > 1 {
		return fmt.Errorf("factory %q must have at most one triggers block", factory.Name)
	}
	if len(factory.Triggers) == 0 {
		return nil
	}
	if len(factory.Triggers[0].Manual) > 1 {
		return fmt.Errorf("factory %q must have at most one manual trigger", factory.Name)
	}
	if err := validateProviderTriggers(
		factory.Name,
		factory.Workflows,
		factory.Triggers[0].Provider,
	); err != nil {
		return err
	}
	return nil
}

func validateFactoryWorkflow(factoryName string, workflow *FactoryWorkflow) error {
	if len(workflow.Plan) > 1 {
		return fmt.Errorf(
			"factory %q workflow %q must have at most one plan block",
			factoryName,
			workflow.Name,
		)
	}
	if len(workflow.Plan) == 1 {
		plan := workflow.Plan[0]
		if plan == nil {
			return fmt.Errorf(
				"factory %q workflow %q plan block is nil",
				factoryName,
				workflow.Name,
			)
		}
		if plan.AgentTemplate == "" {
			return fmt.Errorf(
				"factory %q workflow %q plan.agent_template is required",
				factoryName,
				workflow.Name,
			)
		}
		if plan.Path == "" {
			return fmt.Errorf(
				"factory %q workflow %q plan.path is required",
				factoryName,
				workflow.Name,
			)
		}
		if !cleanProjectRelativePath(plan.Path) {
			return fmt.Errorf(
				"factory %q workflow %q plan.path must be project-relative",
				factoryName,
				workflow.Name,
			)
		}
	}
	if len(workflow.Implement) > 1 {
		return fmt.Errorf(
			"factory %q workflow %q must have at most one implement block",
			factoryName,
			workflow.Name,
		)
	}
	if len(workflow.Implement) == 1 {
		implement := workflow.Implement[0]
		if implement == nil {
			return fmt.Errorf(
				"factory %q workflow %q implement block is nil",
				factoryName,
				workflow.Name,
			)
		}
		if implement.AgentTemplate == "" {
			return fmt.Errorf(
				"factory %q workflow %q implement.agent_template is required",
				factoryName,
				workflow.Name,
			)
		}
		if implement.RequiresApproval != nil {
			return fmt.Errorf(
				"factory %q workflow %q implement does not support requires_approval",
				factoryName,
				workflow.Name,
			)
		}
	}
	if len(workflow.Merge) > 1 {
		return fmt.Errorf(
			"factory %q workflow %q must have at most one merge block",
			factoryName,
			workflow.Name,
		)
	}
	if len(workflow.Merge) == 1 && workflow.Merge[0] != nil && workflow.Merge[0].Target == "" {
		return fmt.Errorf(
			"factory %q workflow %q merge.target is required",
			factoryName,
			workflow.Name,
		)
	}
	if len(workflow.Merge) == 1 && workflow.Merge[0] != nil &&
		workflow.Merge[0].RequiresApproval != nil {
		return fmt.Errorf(
			"factory %q workflow %q merge does not support requires_approval",
			factoryName,
			workflow.Name,
		)
	}
	for _, phase := range workflow.Deploy {
		if phase == nil {
			return fmt.Errorf(
				"factory %q workflow %q deploy block is nil",
				factoryName,
				workflow.Name,
			)
		}
		if !factoryIdentifierPattern.MatchString(phase.Name) {
			return fmt.Errorf(
				"factory %q workflow %q deploy %q name must be a simple identifier",
				factoryName,
				workflow.Name,
				phase.Name,
			)
		}
		if phase.Target == "" {
			return fmt.Errorf(
				"factory %q workflow %q deploy %q target is required",
				factoryName,
				workflow.Name,
				phase.Name,
			)
		}
	}
	for _, phase := range workflow.Verify {
		if phase == nil {
			return fmt.Errorf(
				"factory %q workflow %q verify block is nil",
				factoryName,
				workflow.Name,
			)
		}
		if !factoryIdentifierPattern.MatchString(phase.Name) {
			return fmt.Errorf(
				"factory %q workflow %q verify %q name must be a simple identifier",
				factoryName,
				workflow.Name,
				phase.Name,
			)
		}
		if phase.Target == "" {
			return fmt.Errorf(
				"factory %q workflow %q verify %q target is required",
				factoryName,
				workflow.Name,
				phase.Name,
			)
		}
		if phase.RequiresApproval != nil {
			return fmt.Errorf(
				"factory %q workflow %q verify %q does not support requires_approval",
				factoryName,
				workflow.Name,
				phase.Name,
			)
		}
	}
	return nil
}

func cleanProjectRelativePath(value string) bool {
	if value == "" || filepath.IsAbs(value) || strings.Contains(value, "\\") {
		return false
	}
	cleaned := filepath.Clean(value)
	return cleaned != "." && cleaned != ".." && !strings.HasPrefix(cleaned, "../")
}

func (w *FactoryWorkflow) DaemonExecutable() bool {
	return w != nil && len(w.Plan) == 1 && len(w.Implement) == 1
}

func (w *FactoryWorkflow) PlanRequiresApproval() bool {
	if w == nil || len(w.Plan) == 0 || w.Plan[0] == nil {
		return false
	}
	p := w.Plan[0].RequiresApproval
	return p == nil || *p
}

func (w *FactoryWorkflow) DeployRequiresApproval(name string) bool {
	if w == nil {
		return false
	}
	for _, phase := range w.Deploy {
		if phase != nil && phase.Name == name {
			p := phase.RequiresApproval
			return p != nil && *p
		}
	}
	return false
}

func (w *FactoryWorkflow) PhaseExists(phase string) bool {
	if w == nil {
		return false
	}
	if phase == FactoryPhasePlan {
		return len(w.Plan) == 1 && w.Plan[0] != nil
	}
	if strings.HasPrefix(phase, "deploy.") {
		name := strings.TrimPrefix(phase, "deploy.")
		for _, p := range w.Deploy {
			if p != nil && p.Name == name {
				return true
			}
		}
		return false
	}
	if phase == FactoryPhaseImplement {
		return len(w.Implement) == 1 && w.Implement[0] != nil
	}
	if phase == FactoryPhaseMerge {
		return len(w.Merge) == 1 && w.Merge[0] != nil
	}
	if strings.HasPrefix(phase, "verify.") {
		name := strings.TrimPrefix(phase, "verify.")
		for _, p := range w.Verify {
			if p != nil && p.Name == name {
				return true
			}
		}
	}
	return false
}

func validateFactoryPhaseReferences(project *Project) error {
	for _, factory := range project.Factories {
		for _, workflow := range factory.Workflows {
			if err := validateFactoryWorkflowReferences(
				project,
				factory.Name,
				workflow,
			); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateFactoryWorkflowReferences(
	project *Project,
	factoryName string,
	workflow *FactoryWorkflow,
) error {
	if len(workflow.Plan) == 1 {
		if err := validateFactoryAgentTemplateRef(
			project,
			workflow.Plan[0].AgentTemplate,
		); err != nil {
			return fmt.Errorf("factory %q workflow %q plan: %w", factoryName, workflow.Name, err)
		}
	}
	if len(workflow.Implement) == 1 {
		if err := validateFactoryAgentTemplateRef(
			project,
			workflow.Implement[0].AgentTemplate,
		); err != nil {
			return fmt.Errorf(
				"factory %q workflow %q implement: %w",
				factoryName,
				workflow.Name,
				err,
			)
		}
	}
	if len(workflow.Merge) == 1 {
		if err := validateFactoryTargetRef(project, workflow.Merge[0].Target); err != nil {
			return fmt.Errorf("factory %q workflow %q merge: %w", factoryName, workflow.Name, err)
		}
	}
	for _, phase := range workflow.Deploy {
		if err := validateFactoryTargetRef(project, phase.Target); err != nil {
			return fmt.Errorf(
				"factory %q workflow %q deploy %q: %w",
				factoryName,
				workflow.Name,
				phase.Name,
				err,
			)
		}
	}
	for _, phase := range workflow.Verify {
		if err := validateFactoryTargetRef(project, phase.Target); err != nil {
			return fmt.Errorf(
				"factory %q workflow %q verify %q: %w",
				factoryName,
				workflow.Name,
				phase.Name,
				err,
			)
		}
	}
	return nil
}

func validateFactoryAgentTemplateRef(project *Project, ref string) error {
	key := canonicalAgentTemplateRef(ref)
	if project.AgentTemplates[key] == nil {
		return fmt.Errorf("unknown agent template %q", ref)
	}
	return nil
}

func validateFactoryTargetRef(project *Project, ref string) error {
	canonical, _ := project.ResolveTargetName(ref)
	if project.Targets[canonical] == nil {
		return fmt.Errorf("unknown target %q", ref)
	}
	return nil
}

func canonicalAgentTemplateRef(ref string) string {
	if strings.HasPrefix(ref, "agent_template/") {
		return ref
	}
	return "agent_template/" + strings.TrimPrefix(ref, "agent_template.")
}

func (f *Factory) HasWorkflow(name string) bool {
	if f == nil {
		return false
	}
	for _, workflow := range f.Workflows {
		if workflow != nil && workflow.Name == name {
			return true
		}
	}
	return false
}

func (f *Factory) ManualEnabled() bool {
	if f == nil || len(f.Triggers) == 0 || f.Triggers[0] == nil {
		return false
	}
	return len(f.Triggers[0].Manual) > 0
}

func (f *Factory) ResolveWorkflow(requested string) (string, error) {
	if f == nil {
		return "", fmt.Errorf("factory is nil")
	}
	if requested != "" {
		if !f.HasWorkflow(requested) {
			return "", bacherr.NotFoundf("factory %q workflow %q", f.Name, requested)
		}
		return requested, nil
	}
	if len(f.Workflows) == 1 && f.Workflows[0] != nil {
		return f.Workflows[0].Name, nil
	}
	return "", bacherr.ValidationFailedf(
		"factory %q has multiple workflows; --workflow is required",
		f.Name,
	)
}
