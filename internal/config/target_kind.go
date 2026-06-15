package config

import (
	"strings"

	"github.com/applauselab/bachkator/internal/model"
)

type targetKindAdapter struct {
	target *Target
}

func targetKind(target *Target) targetKindAdapter {
	return targetKindAdapter{target: target}
}

func (k targetKindAdapter) Type() TargetType {
	if k.target == nil {
		return TargetTypeShell
	}
	switch {
	case strings.HasPrefix(k.target.Name, "image/"):
		return TargetTypeImage
	case strings.HasPrefix(k.target.Name, "agent/"):
		return TargetTypeAgent
	case strings.HasPrefix(k.target.Name, "group/"):
		return TargetTypeGroup
	case strings.HasPrefix(k.target.Name, "pipeline/"):
		return TargetTypePipeline
	case len(k.target.Steps) > 0:
		return TargetTypePipeline
	default:
		return TargetTypeShell
	}
}

func (k targetKindAdapter) Is(targetType TargetType) bool {
	return k.Type() == targetType
}

func (k targetKindAdapter) AgentSpec() (model.AgentSpec, bool) {
	if k.target == nil {
		return model.AgentSpec{}, false
	}
	agent, ok := k.target.Spec().Body.(model.AgentSpec)
	return agent, ok
}

func (k targetKindAdapter) CompositeChildren() []string {
	if k.target == nil {
		return nil
	}
	switch body := k.target.Spec().Body.(type) {
	case model.PipelineSpec:
		return body.Steps
	case model.GroupSpec:
		return body.Targets
	default:
		return nil
	}
}

func (k targetKindAdapter) CompositeChildrenByKind() []compositeChildRef {
	if k.target == nil {
		return nil
	}
	spec := k.target.Spec()
	switch body := spec.Body.(type) {
	case model.PipelineSpec:
		children := make([]compositeChildRef, 0, len(body.Steps))
		for _, child := range body.Steps {
			children = append(children, compositeChildRef{Target: child, Kind: "pipeline step"})
		}
		return children
	case model.GroupSpec:
		children := make([]compositeChildRef, 0, len(body.Targets))
		for _, child := range body.Targets {
			children = append(children, compositeChildRef{Target: child, Kind: "group member"})
		}
		return children
	default:
		return nil
	}
}

type compositeChildRef struct {
	Target string
	Kind   string
}
