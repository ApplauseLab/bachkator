package config

import (
	"strings"

	"github.com/applause/bachkator/internal/model"
)

type TargetType = model.TargetType

const (
	TargetTypeShell    = model.TargetTypeShell
	TargetTypeImage    = model.TargetTypeImage
	TargetTypePipeline = model.TargetTypePipeline
	TargetTypeGroup    = model.TargetTypeGroup
)

type TargetSpec = model.TargetSpec
type TargetMetadata = model.TargetMetadata
type TargetRuntime = model.TargetRuntime
type RetryPolicy = model.RetryPolicy
type TargetQuality = model.TargetQuality
type QualityGateSpec = model.QualityGateSpec
type TargetCache = model.TargetCache
type TargetBody = model.TargetBody
type ShellSpec = model.ShellSpec
type ImageSpec = model.ImageSpec
type PipelineSpec = model.PipelineSpec
type GroupSpec = model.GroupSpec
type TargetContract = model.TargetContract
type CompletionCheckSpec = model.CompletionCheckSpec

func (t *Target) Spec() TargetSpec {
	return TargetSpec{
		Name: t.Name,
		Metadata: TargetMetadata{
			Description:          t.Description,
			When:                 t.When,
			Cost:                 t.Cost,
			Remote:               t.Remote,
			Destructive:          t.Destructive,
			RequiresConfirmation: t.RequiresConfirmation,
		},
		Runtime: TargetRuntime{
			Quiet:      t.Quiet,
			Lock:       t.Lock,
			Timeout:    t.TimeoutDuration,
			Retry:      t.RetryPolicy,
			Env:        append([]string(nil), t.Env...),
			Tools:      toolRequirementSpecs(t.Tools),
			Preflights: preflightCheckSpecs(t.Preflights),
		},
		Quality: TargetQuality{
			Reports: qualityReportSpecs(t.Reports),
			Gates:   qualityGateSpecs(t.QualityGates),
		},
		Cache: TargetCache{
			Inputs:       append([]string(nil), t.Inputs...),
			Outputs:      append([]string(nil), t.Outputs...),
			NamedOutputs: outputMapSpec(t.OutputMap),
			Produces:     append([]string(nil), t.Produces...),
		},
		Contract: TargetContract{
			SuccessWhen: completionCheckSpecs(t.SuccessWhen),
			FailWhen:    completionCheckSpecs(t.FailWhen),
		},
		Body: t.body(),
	}
}

func (t *Target) body() TargetBody {
	switch t.targetType() {
	case TargetTypeImage:
		return ImageSpec{
			Builder:    t.Builder,
			Image:      t.Image,
			Tags:       append([]string(nil), t.Tags...),
			Dockerfile: t.Dockerfile,
			Context:    t.Context,
			Platform:   t.Platform,
			Push:       t.Push,
			BuildArgs:  append([]string(nil), t.BuildArgs...),
		}
	case TargetTypePipeline:
		return PipelineSpec{
			Steps: append([]string(nil), t.Steps...),
		}
	case TargetTypeGroup:
		return GroupSpec{
			Targets: append([]string(nil), t.Targets...),
		}
	default:
		return ShellSpec{
			Command: append([]string(nil), t.Command...),
			Shell:   t.Shell,
			WorkDir: t.WorkDir,
		}
	}
}

func outputMapSpec(outputs map[string]string) map[string]string {
	if len(outputs) == 0 {
		return nil
	}
	spec := make(map[string]string, len(outputs))
	for name, path := range outputs {
		spec[name] = path
	}
	return spec
}

func qualityReportSpecs(reports []QualityReportDeclaration) []QualityReportDeclaration {
	if len(reports) == 0 {
		return nil
	}
	return append([]QualityReportDeclaration(nil), reports...)
}

func qualityGateSpecs(gates []*QualityGate) []QualityGateSpec {
	if len(gates) == 0 {
		return nil
	}
	specs := make([]QualityGateSpec, 0, len(gates))
	for _, gate := range gates {
		if gate == nil {
			continue
		}
		spec := QualityGateSpec{Metric: gate.Metric}
		if gate.Min != nil {
			min := *gate.Min
			spec.Min = &min
		}
		if gate.Max != nil {
			max := *gate.Max
			spec.Max = &max
		}
		specs = append(specs, spec)
	}
	return specs
}

func toolRequirementSpecs(tools []ToolRequirement) []ToolRequirement {
	if len(tools) == 0 {
		return nil
	}
	specs := make([]ToolRequirement, 0, len(tools))
	for _, tool := range tools {
		tool.Command = append([]string(nil), tool.Command...)
		specs = append(specs, tool)
	}
	return specs
}

func preflightCheckSpecs(preflights []PreflightCheck) []model.PreflightCheck {
	if len(preflights) == 0 {
		return nil
	}
	specs := make([]model.PreflightCheck, 0, len(preflights))
	for _, preflight := range preflights {
		specs = append(specs, model.PreflightCheck{
			Name:    preflight.Name,
			Kind:    preflight.Kind,
			Command: append([]string(nil), preflight.Command...),
			Fix:     preflight.Fix,
		})
	}
	return specs
}

func completionCheckSpecs(checks []*CompletionCheck) []CompletionCheckSpec {
	if len(checks) == 0 {
		return nil
	}
	specs := make([]CompletionCheckSpec, 0, len(checks))
	for _, check := range checks {
		if check == nil {
			continue
		}
		specs = append(specs, CompletionCheckSpec{
			OutputContains: check.OutputContains,
			FileExists:     check.FileExists,
			Command:        append([]string(nil), check.Command...),
		})
	}
	return specs
}

func (t *Target) targetType() TargetType {
	switch {
	case strings.HasPrefix(t.Name, "image/"):
		return TargetTypeImage
	case strings.HasPrefix(t.Name, "group/"):
		return TargetTypeGroup
	case strings.HasPrefix(t.Name, "pipeline/"):
		return TargetTypePipeline
	case len(t.Steps) > 0:
		return TargetTypePipeline
	default:
		return TargetTypeShell
	}
}
