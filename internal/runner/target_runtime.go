package runner

import (
	"context"

	targetpkg "github.com/applauselab/bachkator/internal/target"
)

func targetLabel(target *Target) string {
	return target.Spec().Label()
}

func targetOperation(
	ctx context.Context,
	targets TargetHandlers,
	target *Target,
	env map[string]string,
) (targetpkg.RunDescription, error) {
	spec := target.Spec()
	handler, err := targets.Handler(spec.TargetType())
	if err != nil {
		return targetpkg.RunDescription{}, err
	}
	return handler.Describe(ctx, targetpkg.DescribeRequest{Spec: spec, Env: env})
}

func executeTarget(
	ctx context.Context,
	targets TargetHandlers,
	target *Target,
	req targetpkg.ExecuteRequest,
) error {
	spec := target.Spec()
	handler, err := targets.Handler(spec.TargetType())
	if err != nil {
		return err
	}
	req.Spec = spec
	return handler.Execute(ctx, req)
}

func targetRunnable(targets TargetHandlers, target *Target) bool {
	spec := target.Spec()
	handler, err := targets.Handler(spec.TargetType())
	return err == nil && handler.Runnable(spec)
}

func targetCompositeChildren(
	targets TargetHandlers,
	target *Target,
) ([]targetpkg.CompositeChild, error) {
	spec := target.Spec()
	handler, err := targets.Handler(spec.TargetType())
	if err != nil {
		return nil, err
	}
	return handler.CompositeChildren(spec.Body), nil
}
