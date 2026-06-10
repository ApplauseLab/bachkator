package runner

import (
	"time"

	"github.com/applause/bachkator/internal/model"
)

type targetOption func(*Target)

func shellTarget(name string, command string, options ...targetOption) *Target {
	target := &Target{
		Name: name,
		SpecValue: model.TargetSpec{
			Name: name,
			Body: model.ShellSpec{Shell: command},
		},
	}
	for _, option := range options {
		option(target)
	}
	return target
}

func commandTarget(name string, command []string, options ...targetOption) *Target {
	target := &Target{
		Name: name,
		SpecValue: model.TargetSpec{
			Name: name,
			Body: model.ShellSpec{Command: append([]string(nil), command...)},
		},
	}
	for _, option := range options {
		option(target)
	}
	return target
}

func imageTarget(name string, image string, tags []string, options ...targetOption) *Target {
	target := &Target{
		Name: name,
		SpecValue: model.TargetSpec{
			Name: name,
			Body: model.ImageSpec{Image: image, Tags: append([]string(nil), tags...)},
		},
	}
	for _, option := range options {
		option(target)
	}
	return target
}

func pipelineTarget(name string, steps []string, options ...targetOption) *Target {
	target := &Target{
		Name: name,
		SpecValue: model.TargetSpec{
			Name: name,
			Body: model.PipelineSpec{Steps: append([]string(nil), steps...)},
		},
	}
	for _, option := range options {
		option(target)
	}
	return target
}

func groupTarget(name string, targets []string, options ...targetOption) *Target {
	target := &Target{
		Name: name,
		SpecValue: model.TargetSpec{
			Name: name,
			Body: model.GroupSpec{Targets: append([]string(nil), targets...)},
		},
	}
	for _, option := range options {
		option(target)
	}
	return target
}

func withInputs(inputs ...string) targetOption {
	return func(target *Target) {
		target.SpecValue.Cache.Inputs = append([]string(nil), inputs...)
	}
}

func withOutputs(outputs ...string) targetOption {
	return func(target *Target) {
		target.Outputs = append([]string(nil), outputs...)
		target.SpecValue.Cache.Outputs = append([]string(nil), outputs...)
	}
}

func withDependsOn(deps ...string) targetOption {
	return func(target *Target) {
		target.DependsOn = append([]string(nil), deps...)
	}
}

func withEnv(env ...string) targetOption {
	return func(target *Target) {
		target.Env = append([]string(nil), env...)
		target.SpecValue.Runtime.Env = append([]string(nil), env...)
	}
}

func withTool(tool model.ToolRequirement) targetOption {
	return func(target *Target) {
		target.SpecValue.Runtime.Tools = append(target.SpecValue.Runtime.Tools, tool)
	}
}

func withPreflight(preflight model.PreflightCheck) targetOption {
	return func(target *Target) {
		target.SpecValue.Runtime.Preflights = append(target.SpecValue.Runtime.Preflights, preflight)
	}
}

func withRetry(policy model.RetryPolicy) targetOption {
	return func(target *Target) {
		target.SpecValue.Runtime.Retry = policy
	}
}

func withTimeout(timeout time.Duration) targetOption {
	return func(target *Target) {
		target.SpecValue.Runtime.Timeout = timeout
	}
}

func withLock(lock string) targetOption {
	return func(target *Target) {
		target.SpecValue.Runtime.Lock = lock
	}
}

func withQuiet() targetOption {
	return func(target *Target) {
		target.SpecValue.Runtime.Quiet = true
	}
}

func withRisk(remote, destructive, confirmation bool) targetOption {
	return func(target *Target) {
		target.SpecValue.Metadata.Remote = remote
		target.SpecValue.Metadata.Destructive = destructive
		target.SpecValue.Metadata.RequiresConfirmation = confirmation
	}
}

func withSuccess(checks ...model.CompletionCheckSpec) targetOption {
	return func(target *Target) {
		target.SpecValue.Contract.SuccessWhen = append([]model.CompletionCheckSpec(nil), checks...)
	}
}

func withFail(checks ...model.CompletionCheckSpec) targetOption {
	return func(target *Target) {
		target.SpecValue.Contract.FailWhen = append([]model.CompletionCheckSpec(nil), checks...)
	}
}

func withQualityGate(gates ...model.QualityGateSpec) targetOption {
	return func(target *Target) {
		target.SpecValue.Quality.Gates = append([]model.QualityGateSpec(nil), gates...)
	}
}
