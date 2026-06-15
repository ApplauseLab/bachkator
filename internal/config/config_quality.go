package config

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

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
		if len(target.Reports) > 0 || len(target.RegoPolicies) > 0 || len(target.QualityGates) > 0 {
			return fmt.Errorf("duplicate quality block for target %q", quality.Target)
		}
		target.Reports = append([]QualityReportDeclaration(nil), quality.Reports...)
		reports, err := qualityReportsFromBlocks("tests", "junit-xml", quality.JUnit)
		if err != nil {
			return fmt.Errorf("quality block target %q junit: %w", quality.Target, err)
		}
		target.Reports = append(target.Reports, reports...)
		reports, err = qualityReportsFromBlocks("coverage", "lcov", quality.Coverage)
		if err != nil {
			return fmt.Errorf("quality block target %q cov: %w", quality.Target, err)
		}
		target.Reports = append(target.Reports, reports...)
		reports, err = qualityReportsFromBlocks("lint", "checkstyle-xml", quality.Lint)
		if err != nil {
			return fmt.Errorf("quality block target %q lint: %w", quality.Target, err)
		}
		target.Reports = append(target.Reports, reports...)
		reports, err = qualityReportsFromBlocks("complexity", "gocyclo", quality.Complexity)
		if err != nil {
			return fmt.Errorf("quality block target %q complexity: %w", quality.Target, err)
		}
		target.Reports = append(target.Reports, reports...)
		target.RegoPolicies = append([]*RegoPolicyBlock(nil), quality.RegoPolicies...)
		target.QualityGates = append([]*QualityGate(nil), quality.QualityGates...)
		if err := validateTargetMetadata(target); err != nil {
			return err
		}
		if err := validateQualityParsers(project, target); err != nil {
			return err
		}
	}
	return nil
}

func qualityReportsFromBlocks(
	kind string,
	defaultFormat string,
	blocks []*QualityReportBlock,
) ([]QualityReportDeclaration, error) {
	reports := make([]QualityReportDeclaration, 0, len(blocks))
	for _, block := range blocks {
		if block == nil {
			continue
		}
		parser, err := parserRefFromExpr(block.Parser)
		if err != nil {
			return nil, err
		}
		format := block.Format
		if format == "" && parser == "" {
			format = defaultFormat
		}
		reports = append(
			reports,
			QualityReportDeclaration{Kind: kind, Format: format, Parser: parser, Path: block.Path},
		)
	}
	return reports, nil
}

func parserRefFromExpr(expr hcl.Expression) (string, error) {
	if expr == nil {
		return "", nil
	}
	vars := expr.Variables()
	if len(vars) == 0 {
		return "", nil
	}
	if len(vars) != 1 || len(vars[0]) != 2 {
		return "", fmt.Errorf("parser must be a plugin reference like plugin.name")
	}
	root, ok := vars[0][0].(hcl.TraverseRoot)
	if !ok || root.Name != "plugin" {
		return "", fmt.Errorf("parser must reference plugin.name")
	}
	attr, ok := vars[0][1].(hcl.TraverseAttr)
	if !ok || attr.Name == "" {
		return "", fmt.Errorf("parser must reference plugin.name")
	}
	return attr.Name, nil
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

func qualityReportFromValue(value cty.Value) QualityReportDeclaration {
	attrs := value.AsValueMap()
	return QualityReportDeclaration{
		Kind:   stringAttr(attrs, "kind"),
		Format: stringAttr(attrs, "format"),
		Parser: stringAttr(attrs, "parser"),
		Path:   stringAttr(attrs, "path"),
	}
}

func validateQualityParsers(project *Project, target *Target) error {
	for _, report := range target.Spec().Quality.Reports {
		if report.Parser == "" {
			continue
		}
		plugin, ok := project.Plugins[report.Parser]
		if !ok {
			return fmt.Errorf(
				"target %q report %q references unknown parser plugin %q",
				target.Name,
				report.Kind,
				report.Parser,
			)
		}
		if plugin.Type != pluginTypeQuality {
			return fmt.Errorf(
				"target %q report %q parser plugin %q must have type = \"quality\"",
				target.Name,
				report.Kind,
				report.Parser,
			)
		}
	}
	return nil
}
