package target

import (
	"context"
	"fmt"
	"strings"

	"github.com/applause/bachkator/internal/model"
)

type groupHandler struct{}

func (groupHandler) Type() model.TargetType { return model.TargetTypeGroup }

func (groupHandler) Runnable(model.TargetSpec) bool { return false }

func (groupHandler) Describe(_ context.Context, req DescribeRequest) (RunDescription, error) {
	body, ok := req.Spec.Body.(model.GroupSpec)
	if !ok {
		return RunDescription{}, fmt.Errorf(
			"target %q has %s body, want group",
			req.Spec.Name,
			req.Spec.TargetType(),
		)
	}
	return RunDescription{Operation: "group: " + strings.Join(body.Targets, ", ")}, nil
}

func (groupHandler) Execute(context.Context, ExecuteRequest) error {
	return fmt.Errorf("group targets are orchestrated by runner")
}

func (groupHandler) FingerprintParts(body model.TargetBody) map[string]string {
	group, _ := body.(model.GroupSpec)
	return map[string]string{
		"targets": strings.Join(group.Targets, "\x00"),
	}
}
