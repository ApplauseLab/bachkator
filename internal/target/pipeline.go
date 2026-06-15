package target

import "github.com/applauselab/bachkator/internal/model"

func pipelineHandler() TargetHandler {
	return compositeTargetHandler{
		targetType:     model.TargetTypePipeline,
		label:          "pipeline",
		separator:      " -> ",
		fingerprintKey: "steps",
		childKind:      "pipeline_step",
		children:       pipelineSteps,
	}
}

func pipelineSteps(body model.TargetBody) ([]string, bool) {
	pipeline, ok := body.(model.PipelineSpec)
	return pipeline.Steps, ok
}
