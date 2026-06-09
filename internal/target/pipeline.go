package target

import (
	"context"
	"fmt"
	"strings"

	"github.com/applause/bachkator/internal/model"
)

type pipelineHandler struct{}

func (pipelineHandler) Type() model.TargetType { return model.TargetTypePipeline }

func (pipelineHandler) Runnable(model.TargetSpec) bool { return false }

func (pipelineHandler) Describe(_ context.Context, req DescribeRequest) (RunDescription, error) {
	body, ok := req.Spec.Body.(model.PipelineSpec)
	if !ok {
		return RunDescription{}, fmt.Errorf(
			"target %q has %s body, want pipeline",
			req.Spec.Name,
			req.Spec.TargetType(),
		)
	}
	return RunDescription{Operation: "pipeline: " + strings.Join(body.Steps, " -> ")}, nil
}

func (pipelineHandler) Execute(context.Context, ExecuteRequest) error {
	return fmt.Errorf("pipeline targets are orchestrated by runner")
}

func (pipelineHandler) FingerprintParts(body model.TargetBody) map[string]string {
	pipeline, _ := body.(model.PipelineSpec)
	return map[string]string{
		"steps": strings.Join(pipeline.Steps, "\x00"),
	}
}
