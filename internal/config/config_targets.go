package config

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

func registerTargets(
	project *Project,
	shells []*Target,
	agents []*Target,
	images []*Target,
	pipelines []*Target,
	groups []*Target,
	variables map[string]string,
) error {
	targetBaseEnv := projectRuntimeEnv(project)
	targetRefs := targetRefEvalContext(shells, images, pipelines, groups, agents)
	for _, shell := range shells {
		shell.Name = "shell/" + shell.Name
		if err := resolveTargetExtraAttributes(shell, targetRefs); err != nil {
			return targetSourceError(shell, err)
		}
		if err := validateTargetMetadata(shell); err != nil {
			return targetSourceError(shell, err)
		}
		if err := resolveTargetEnv(shell, variables, targetBaseEnv, project.Root); err != nil {
			return targetSourceError(shell, err)
		}
		if _, exists := project.Targets[shell.Name]; exists {
			return fmt.Errorf("duplicate target %q", shell.Name)
		}
		project.Targets[shell.Name] = shell
	}
	for _, image := range images {
		image.Name = "image/" + image.Name
		if err := resolveTargetExtraAttributes(image, targetRefs); err != nil {
			return targetSourceError(image, err)
		}
		if err := validateTargetMetadata(image); err != nil {
			return targetSourceError(image, err)
		}
		if err := resolveTargetEnv(image, variables, targetBaseEnv, project.Root); err != nil {
			return targetSourceError(image, err)
		}
		image.BuildArgs = append(image.BuildArgs, buildArgList(image.BuildArgMap)...)
		if _, exists := project.Targets[image.Name]; exists {
			return fmt.Errorf("duplicate target %q", image.Name)
		}
		project.Targets[image.Name] = image
	}
	for _, pipeline := range pipelines {
		pipeline.Name = "pipeline/" + pipeline.Name
		if err := resolveTargetExtraAttributes(pipeline, targetRefs); err != nil {
			return targetSourceError(pipeline, err)
		}
		if err := validateTargetMetadata(pipeline); err != nil {
			return targetSourceError(pipeline, err)
		}
		if _, exists := project.Targets[pipeline.Name]; exists {
			return fmt.Errorf("duplicate target %q", pipeline.Name)
		}
		project.Targets[pipeline.Name] = pipeline
	}
	for _, group := range groups {
		group.Name = "group/" + group.Name
		if err := resolveTargetExtraAttributes(group, targetRefs); err != nil {
			return targetSourceError(group, err)
		}
		if err := validateTargetMetadata(group); err != nil {
			return targetSourceError(group, err)
		}
		if _, exists := project.Targets[group.Name]; exists {
			return fmt.Errorf("duplicate target %q", group.Name)
		}
		project.Targets[group.Name] = group
	}
	for _, agent := range agents {
		agent.Name = "agent/" + agent.Name
		if err := resolveTargetExtraAttributes(agent, targetRefs); err != nil {
			return targetSourceError(agent, err)
		}
		if err := applyAgentTemplate(project, agent); err != nil {
			return targetSourceError(agent, err)
		}
		if err := validateTargetMetadata(agent); err != nil {
			return targetSourceError(agent, err)
		}
		if err := resolveTargetEnv(agent, variables, targetBaseEnv, project.Root); err != nil {
			return targetSourceError(agent, err)
		}
		if err := resolveAgentReferences(project, agent); err != nil {
			return targetSourceError(agent, err)
		}
		if _, exists := project.Targets[agent.Name]; exists {
			return fmt.Errorf("duplicate target %q", agent.Name)
		}
		project.Targets[agent.Name] = agent
	}
	if err := attachAgentPolicies(project); err != nil {
		return err
	}
	if err := attachMergeSubjects(project); err != nil {
		return err
	}
	if err := validateSubjectPolicies(project); err != nil {
		return err
	}
	return nil
}

func targetSourceError(target *Target, err error) error {
	if target == nil || target.Remain == nil || err == nil {
		return err
	}
	rangeInfo := target.Remain.MissingItemRange()
	if rangeInfo.Filename == "" {
		return err
	}
	return fmt.Errorf("%s: %w", rangeInfo.String(), err)
}

func resolveTargetExtraAttributes(target *Target, targetRefs *hcl.EvalContext) error {
	if target.Remain == nil {
		return nil
	}
	content, _, diags := target.Remain.PartialContent(
		&hcl.BodySchema{
			Attributes: []hcl.AttributeSchema{
				{Name: "tools"},
				{Name: "preflights"},
				{Name: "outputs"},
				{Name: "depends_on"},
				{Name: "steps"},
				{Name: "targets"},
				{Name: "subject"},
			},
		},
	)
	if diags.HasErrors() {
		return fmt.Errorf("target %q extra attributes: %s", target.Name, diags.Error())
	}
	if attr, ok := content.Attributes["tools"]; ok {
		value, diags := attr.Expr.Value(nil)
		if diags.HasErrors() {
			return fmt.Errorf("target %q tools: %s", target.Name, diags.Error())
		}
		if !value.CanIterateElements() {
			return fmt.Errorf("target %q tools must be a list", target.Name)
		}
		for _, element := range value.AsValueSlice() {
			if !element.Type().IsObjectType() && !element.Type().IsMapType() {
				return fmt.Errorf("target %q tools entries must be objects", target.Name)
			}
			target.Tools = append(target.Tools, toolRequirementFromValue(element))
		}
	}
	if attr, ok := content.Attributes["preflights"]; ok {
		value, diags := attr.Expr.Value(nil)
		if diags.HasErrors() {
			return fmt.Errorf("target %q preflights: %s", target.Name, diags.Error())
		}
		if !value.CanIterateElements() {
			return fmt.Errorf("target %q preflights must be a list", target.Name)
		}
		for _, element := range value.AsValueSlice() {
			if !element.Type().IsObjectType() && !element.Type().IsMapType() {
				return fmt.Errorf("target %q preflights entries must be objects", target.Name)
			}
			target.Preflights = append(target.Preflights, preflightCheckFromValue(element))
		}
	}
	if attr, ok := content.Attributes["outputs"]; ok {
		outputs, outputMap, err := decodeTargetOutputs(attr)
		if err != nil {
			return fmt.Errorf("target %q outputs: %w", target.Name, err)
		}
		target.Outputs = outputs
		target.OutputMap = outputMap
	}
	if attr, ok := content.Attributes["depends_on"]; ok {
		dependsOn, err := decodeTargetRefList(attr, targetRefs)
		if err != nil {
			return fmt.Errorf("target %q depends_on: %w", target.Name, err)
		}
		target.DependsOn = dependsOn
	}
	if attr, ok := content.Attributes["steps"]; ok {
		steps, err := decodeTargetRefList(attr, targetRefs)
		if err != nil {
			return fmt.Errorf("target %q steps: %w", target.Name, err)
		}
		target.Steps = steps
	}
	if attr, ok := content.Attributes["targets"]; ok {
		targets, err := decodeTargetRefList(attr, targetRefs)
		if err != nil {
			return fmt.Errorf("target %q targets: %w", target.Name, err)
		}
		target.Targets = targets
	}
	if attr, ok := content.Attributes["subject"]; ok {
		subject, ok, err := evalStringExpr(attr, targetRefs)
		if err != nil {
			return fmt.Errorf("target %q subject: %w", target.Name, err)
		}
		if !ok {
			return fmt.Errorf("target %q subject: must reference a target", target.Name)
		}
		target.Subject = subject
	}
	return nil
}

func decodeTargetOutputs(attr *hcl.Attribute) ([]string, map[string]string, error) {
	value, diags := attr.Expr.Value(nil)
	if diags.HasErrors() {
		return nil, nil, fmt.Errorf("%s", diags.Error())
	}
	if value.Type().IsObjectType() || value.Type().IsMapType() {
		attrs := value.AsValueMap()
		outputs := make([]string, 0, len(attrs))
		outputMap := make(map[string]string, len(attrs))
		for name, output := range attrs {
			if output.Type() != cty.String {
				return nil, nil, fmt.Errorf("map values must be strings")
			}
			path := output.AsString()
			outputs = append(outputs, path)
			outputMap[name] = path
		}
		return outputs, outputMap, nil
	}
	if !value.CanIterateElements() {
		return nil, nil, fmt.Errorf("must be a list or object")
	}
	outputs := []string{}
	for _, item := range value.AsValueSlice() {
		if item.Type() != cty.String {
			return nil, nil, fmt.Errorf("list values must be strings")
		}
		outputs = append(outputs, item.AsString())
	}
	return outputs, nil, nil
}

func toolRequirementFromValue(value cty.Value) ToolRequirement {
	attrs := value.AsValueMap()
	return ToolRequirement{
		Name:    stringAttr(attrs, "name"),
		Command: stringListAttr(attrs, "command"),
		Version: stringAttr(attrs, "version"),
		Fix:     stringAttr(attrs, "fix"),
	}
}

func preflightCheckFromValue(value cty.Value) PreflightCheck {
	attrs := value.AsValueMap()
	return PreflightCheck{
		Name:    stringAttr(attrs, "name"),
		Kind:    stringAttr(attrs, "kind"),
		Command: stringListAttr(attrs, "command"),
		Fix:     stringAttr(attrs, "fix"),
	}
}

func stringAttr(attrs map[string]cty.Value, name string) string {
	value, ok := attrs[name]
	if !ok || value.IsNull() || value.Type() != cty.String {
		return ""
	}
	return value.AsString()
}

func stringListAttr(attrs map[string]cty.Value, name string) []string {
	value, ok := attrs[name]
	if !ok || value.IsNull() || !value.CanIterateElements() {
		return nil
	}
	values := value.AsValueSlice()
	strings := make([]string, 0, len(values))
	for _, item := range values {
		if item.Type() == cty.String {
			strings = append(strings, item.AsString())
		}
	}
	return strings
}
