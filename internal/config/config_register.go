package config

import (
	"fmt"
	"time"

	"github.com/applause/bachkator/internal/model"
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

func registerInputs(project *Project, inputs []*Input) error {
	for _, input := range inputs {
		if input.Src != "" && len(input.Srcs) > 0 {
			return fmt.Errorf("input %q %q must use src or srcs, not both", input.Kind, input.Name)
		}
		key := input.Key()
		if _, exists := project.Inputs[key]; exists {
			return fmt.Errorf("duplicate input %q", key)
		}
		project.Inputs[key] = input
	}
	return nil
}

func registerResources(project *Project, resources []*Resource) error {
	for _, resource := range resources {
		key := resource.Key()
		if _, exists := project.Resources[key]; exists {
			return fmt.Errorf("duplicate resource %q", key)
		}
		project.Resources[key] = resource
	}
	return nil
}

func registerTargets(
	project *Project,
	shells []*Target,
	images []*Target,
	pipelines []*Target,
	groups []*Target,
	variables map[string]string,
) error {
	targetBaseEnv := projectRuntimeEnv(project)
	targetRefs := targetRefEvalContext(shells, images, pipelines, groups)
	for _, shell := range shells {
		shell.Name = "shell/" + shell.Name
		if err := resolveTargetExtraAttributes(shell, targetRefs); err != nil {
			return err
		}
		if err := validateTargetMetadata(shell); err != nil {
			return err
		}
		if err := resolveTargetEnv(shell, variables, targetBaseEnv, project.Root); err != nil {
			return err
		}
		if _, exists := project.Targets[shell.Name]; exists {
			return fmt.Errorf("duplicate target %q", shell.Name)
		}
		project.Targets[shell.Name] = shell
	}
	for _, image := range images {
		image.Name = "image/" + image.Name
		if err := resolveTargetExtraAttributes(image, targetRefs); err != nil {
			return err
		}
		if err := validateTargetMetadata(image); err != nil {
			return err
		}
		if err := resolveTargetEnv(image, variables, targetBaseEnv, project.Root); err != nil {
			return err
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
			return err
		}
		if err := validateTargetMetadata(pipeline); err != nil {
			return err
		}
		if _, exists := project.Targets[pipeline.Name]; exists {
			return fmt.Errorf("duplicate target %q", pipeline.Name)
		}
		project.Targets[pipeline.Name] = pipeline
	}
	for _, group := range groups {
		group.Name = "group/" + group.Name
		if err := resolveTargetExtraAttributes(group, targetRefs); err != nil {
			return err
		}
		if err := validateTargetMetadata(group); err != nil {
			return err
		}
		if _, exists := project.Targets[group.Name]; exists {
			return fmt.Errorf("duplicate target %q", group.Name)
		}
		project.Targets[group.Name] = group
	}
	return nil
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

func registerQualityConfigs(project *Project, qualities []*QualityConfig) error {
	for _, quality := range qualities {
		canonical, err := canonicalTargetRef(quality.Target)
		if err != nil {
			return fmt.Errorf("quality block target: %w", err)
		}
		quality.Target = canonical
		if err := resolveQualityReports(quality); err != nil {
			return err
		}
		target, ok := project.Targets[quality.Target]
		if !ok {
			return fmt.Errorf("quality block references unknown target %q", quality.Target)
		}
		if len(target.Reports) > 0 || len(target.QualityGates) > 0 {
			return fmt.Errorf("duplicate quality block for target %q", quality.Target)
		}
		target.Reports = append([]QualityReportDeclaration(nil), quality.Reports...)
		target.Reports = append(
			target.Reports,
			qualityReportsFromBlocks("tests", "junit-xml", quality.JUnit)...)
		target.Reports = append(
			target.Reports,
			qualityReportsFromBlocks("coverage", "lcov", quality.Coverage)...)
		target.Reports = append(
			target.Reports,
			qualityReportsFromBlocks("lint", "checkstyle-xml", quality.Lint)...)
		target.Reports = append(
			target.Reports,
			qualityReportsFromBlocks("complexity", "gocyclo", quality.Complexity)...)
		target.QualityGates = append([]*QualityGate(nil), quality.QualityGates...)
		if err := validateTargetMetadata(target); err != nil {
			return err
		}
	}
	return nil
}

func qualityReportsFromBlocks(
	kind string,
	defaultFormat string,
	blocks []*QualityReportBlock,
) []QualityReportDeclaration {
	reports := make([]QualityReportDeclaration, 0, len(blocks))
	for _, block := range blocks {
		if block == nil {
			continue
		}
		format := block.Format
		if format == "" {
			format = defaultFormat
		}
		reports = append(
			reports,
			QualityReportDeclaration{Kind: kind, Format: format, Path: block.Path},
		)
	}
	return reports
}

func resolveQualityReports(quality *QualityConfig) error {
	if quality.Remain == nil {
		return nil
	}
	content, _, diags := quality.Remain.PartialContent(
		&hcl.BodySchema{Attributes: []hcl.AttributeSchema{{Name: "reports"}}},
	)
	if diags.HasErrors() {
		return fmt.Errorf("quality %q reports: %s", quality.Target, diags.Error())
	}
	attr, ok := content.Attributes["reports"]
	if !ok {
		return nil
	}
	value, diags := attr.Expr.Value(nil)
	if diags.HasErrors() {
		return fmt.Errorf("quality %q reports: %s", quality.Target, diags.Error())
	}
	if !value.CanIterateElements() {
		return fmt.Errorf("quality %q reports must be a list", quality.Target)
	}
	for _, element := range value.AsValueSlice() {
		if !element.Type().IsObjectType() && !element.Type().IsMapType() {
			return fmt.Errorf("quality %q reports entries must be objects", quality.Target)
		}
		quality.Reports = append(quality.Reports, qualityReportFromValue(element))
	}
	return nil
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

func qualityReportFromValue(value cty.Value) QualityReportDeclaration {
	attrs := value.AsValueMap()
	return QualityReportDeclaration{
		Kind:   stringAttr(attrs, "kind"),
		Format: stringAttr(attrs, "format"),
		Path:   stringAttr(attrs, "path"),
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

func registerAliases(project *Project, aliases []*Alias) error {
	for _, alias := range aliases {
		canonical, err := canonicalTargetOrAliasRef(alias.Target)
		if err != nil {
			return fmt.Errorf("alias %q target: %w", alias.Name, err)
		}
		alias.Target = canonical
		if _, exists := project.Targets[alias.Name]; exists {
			return fmt.Errorf("alias %q conflicts with target", alias.Name)
		}
		if _, exists := project.Aliases[alias.Name]; exists {
			return fmt.Errorf("duplicate alias %q", alias.Name)
		}
		project.Aliases[alias.Name] = alias
	}
	for _, alias := range aliases {
		if _, exists := project.Aliases[alias.Target]; exists {
			return fmt.Errorf(
				"alias %q points to alias %q; alias chains are not supported",
				alias.Name,
				alias.Target,
			)
		}
		if _, exists := project.Targets[alias.Target]; !exists {
			return fmt.Errorf("alias %q points to unknown target %q", alias.Name, alias.Target)
		}
	}
	return nil
}

func (p *Project) ResolveTargetName(name string) (string, *Alias) {
	if alias := p.Aliases[name]; alias != nil {
		return alias.Target, alias
	}
	return name, nil
}

func validateTargetMetadata(target *Target) error {
	if err := resolveTargetRuntimePolicy(target); err != nil {
		return err
	}
	spec := target.Spec()
	switch spec.Metadata.Cost {
	case "", "low", "medium", "high":
	default:
		return fmt.Errorf(
			"target %q has invalid cost %q; want low, medium, or high",
			spec.Name,
			spec.Metadata.Cost,
		)
	}
	for _, check := range spec.Contract.SuccessWhen {
		if err := validateCompletionCheck(spec.Name, "success_when", check); err != nil {
			return err
		}
	}
	for _, check := range spec.Contract.FailWhen {
		if err := validateCompletionCheck(spec.Name, "fail_when", check); err != nil {
			return err
		}
	}
	for _, tool := range spec.Runtime.Tools {
		if tool.Name == "" {
			return fmt.Errorf("target %q tool requirement must set name", spec.Name)
		}
		if len(tool.Command) > 0 && tool.Command[0] == "" {
			return fmt.Errorf(
				"target %q tool %q command must not start with an empty string",
				spec.Name,
				tool.Name,
			)
		}
	}
	for _, preflight := range spec.Runtime.Preflights {
		if preflight.Name == "" && preflight.Kind == "" {
			return fmt.Errorf("target %q preflight must set name or kind", spec.Name)
		}
		if len(preflight.Command) == 0 {
			return fmt.Errorf(
				"target %q preflight %q must set command",
				spec.Name,
				preflight.Label(),
			)
		}
		if preflight.Command[0] == "" {
			return fmt.Errorf(
				"target %q preflight %q command must not start with an empty string",
				spec.Name,
				preflight.Label(),
			)
		}
	}
	for _, report := range spec.Quality.Reports {
		if report.Kind == "" {
			return fmt.Errorf("target %q report must set kind", spec.Name)
		}
		if report.Format == "" {
			return fmt.Errorf("target %q report %q must set format", spec.Name, report.Kind)
		}
		if report.Path == "" {
			return fmt.Errorf("target %q report %q must set path", spec.Name, report.Kind)
		}
	}
	for _, gate := range spec.Quality.Gates {
		if gate.Metric == "" {
			return fmt.Errorf("target %q quality_gate must set metric", spec.Name)
		}
		if gate.Min == nil && gate.Max == nil {
			return fmt.Errorf(
				"target %q quality_gate %q must set min or max",
				spec.Name,
				gate.Metric,
			)
		}
	}
	return nil
}

func resolveTargetRuntimePolicy(target *Target) error {
	if target.Timeout != "" {
		timeout, err := time.ParseDuration(target.Timeout)
		if err != nil {
			return fmt.Errorf(
				"target %q timeout %q is invalid: %w",
				target.Name,
				target.Timeout,
				err,
			)
		}
		if timeout <= 0 {
			return fmt.Errorf("target %q timeout must be greater than zero", target.Name)
		}
		target.TimeoutDuration = timeout
	}
	if len(target.Retry) > 1 {
		return fmt.Errorf("target %q must have at most one retry block", target.Name)
	}
	if len(target.Retry) == 0 || target.Retry[0] == nil {
		target.RetryPolicy = model.RetryPolicy{}
		return nil
	}
	retry := target.Retry[0]
	if retry.Attempts < 1 {
		return fmt.Errorf("target %q retry attempts must be greater than zero", target.Name)
	}
	policy := model.RetryPolicy{Attempts: retry.Attempts}
	if retry.Backoff != "" {
		backoff, err := time.ParseDuration(retry.Backoff)
		if err != nil {
			return fmt.Errorf(
				"target %q retry backoff %q is invalid: %w",
				target.Name,
				retry.Backoff,
				err,
			)
		}
		if backoff < 0 {
			return fmt.Errorf("target %q retry backoff must not be negative", target.Name)
		}
		retry.BackoffDuration = backoff
		policy.Backoff = backoff
	}
	target.RetryPolicy = policy
	return nil
}

func validateCompletionCheck(targetName string, blockName string, check CompletionCheckSpec) error {
	conditions := 0
	if check.OutputContains != "" {
		conditions++
	}
	if check.FileExists != "" {
		conditions++
	}
	if len(check.Command) > 0 {
		conditions++
	}
	if conditions != 1 {
		return fmt.Errorf(
			"target %q %s must set exactly one of output_contains, file_exists, or command",
			targetName,
			blockName,
		)
	}
	return nil
}

func wireProducedInputs(project *Project) error {
	for _, target := range project.Targets {
		spec := target.Spec()
		for _, produced := range spec.Cache.Produces {
			if !hasInputOrResource(project, produced) {
				return fmt.Errorf(
					"target %q produces missing input/resource %q",
					spec.Name,
					produced,
				)
			}
			if owner, exists := project.Producers[produced]; exists {
				return fmt.Errorf("input %q produced by both %q and %q", produced, owner, spec.Name)
			}
			project.Producers[produced] = spec.Name
		}
	}
	for _, target := range project.Targets {
		spec := target.Spec()
		for _, input := range spec.Cache.Inputs {
			if producer, exists := project.Producers[input]; exists && producer != target.Name {
				target.DependsOn = appendUnique(target.DependsOn, producer)
			}
		}
	}
	for _, target := range project.Targets {
		spec := target.Spec()
		for _, dep := range target.DependsOn {
			if _, exists := project.Targets[dep]; !exists {
				return fmt.Errorf("target %q depends on missing target %q", spec.Name, dep)
			}
		}
		pipeline, _ := spec.Body.(model.PipelineSpec)
		for _, step := range pipeline.Steps {
			if _, exists := project.Targets[step]; !exists {
				return fmt.Errorf("target %q has missing pipeline step %q", spec.Name, step)
			}
		}
		group, _ := spec.Body.(model.GroupSpec)
		for _, member := range group.Targets {
			if _, exists := project.Targets[member]; !exists {
				return fmt.Errorf("target %q has missing group member %q", spec.Name, member)
			}
		}
	}
	if err := validateCompositeTargetCycles(project); err != nil {
		return err
	}
	if project.DefaultTarget != "" {
		project.DefaultTarget, _ = project.ResolveTargetName(project.DefaultTarget)
		if _, exists := project.Targets[project.DefaultTarget]; !exists {
			return fmt.Errorf("default target %q does not exist", project.DefaultTarget)
		}
	}
	return nil
}

func validateCompositeTargetCycles(project *Project) error {
	visiting := map[string]bool{}
	visited := map[string]bool{}
	var visit func(string) error
	visit = func(name string) error {
		if visiting[name] {
			return fmt.Errorf("composite target cycle includes %q", name)
		}
		if visited[name] {
			return nil
		}
		target := project.Targets[name]
		if target == nil {
			return nil
		}
		visiting[name] = true
		for _, child := range compositeChildren(target) {
			if err := visit(child); err != nil {
				return err
			}
		}
		visiting[name] = false
		visited[name] = true
		return nil
	}
	for name, target := range project.Targets {
		if len(compositeChildren(target)) > 0 {
			if err := visit(name); err != nil {
				return err
			}
		}
	}
	return nil
}

func compositeChildren(target *Target) []string {
	if target == nil {
		return nil
	}
	switch body := target.Spec().Body.(type) {
	case model.PipelineSpec:
		return body.Steps
	case model.GroupSpec:
		return body.Targets
	default:
		return nil
	}
}
