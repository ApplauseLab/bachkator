package config

import (
	"fmt"
	"time"

	"github.com/applauselab/bachkator/internal/model"
)

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
		if report.Format == "" && report.Parser == "" {
			return fmt.Errorf(
				"target %q report %q must set format or parser",
				spec.Name,
				report.Kind,
			)
		}
		if report.Format != "" && report.Parser != "" {
			return fmt.Errorf(
				"target %q report %q must set format or parser, not both",
				spec.Name,
				report.Kind,
			)
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
	for _, policy := range spec.Quality.RegoPolicies {
		if policy.Path == "" {
			return fmt.Errorf("target %q rego_policy must set path", spec.Name)
		}
		if policy.Package == "" && policy.Allow == "" {
			return fmt.Errorf(
				"target %q rego_policy %q must set allow when package is omitted",
				spec.Name,
				policy.Path,
			)
		}
		if policy.Package == "" && policy.Findings == "" {
			return fmt.Errorf(
				"target %q rego_policy %q must set findings when package is omitted",
				spec.Name,
				policy.Path,
			)
		}
	}
	if len(target.Improve) > 1 {
		return fmt.Errorf("target %q must have at most one improve block", spec.Name)
	}
	if err := validateImprovePolicy(spec); err != nil {
		return err
	}
	return nil
}

func validateImprovePolicy(spec model.TargetSpec) error {
	agent, ok := spec.Body.(model.AgentSpec)
	if !ok {
		return nil
	}
	if agent.Improve.MaxAttempts == 0 && agent.Improve.Until == "" {
		return nil
	}
	if agent.Improve.MaxAttempts < 1 {
		return fmt.Errorf("target %q improve max_attempts must be greater than zero", spec.Name)
	}
	if agent.Improve.Until != "policy.passed" {
		return fmt.Errorf("target %q improve until must be policy.passed", spec.Name)
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
	policy := model.RetryPolicy{
		Attempts:                  retry.Attempts,
		RetryOnQualityGateFailure: retry.RetryOnQualityGateFailure,
	}
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
