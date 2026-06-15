package target

import (
	"context"
	"fmt"
	"strings"

	"github.com/applauselab/bachkator/internal/model"
)

type compositeTargetHandler struct {
	targetType     model.TargetType
	label          string
	separator      string
	fingerprintKey string
	childKind      string
	children       func(model.TargetBody) ([]string, bool)
}

func (h compositeTargetHandler) Type() model.TargetType { return h.targetType }

func (compositeTargetHandler) Runnable(model.TargetSpec) bool { return false }

func (h compositeTargetHandler) Describe(
	_ context.Context,
	req DescribeRequest,
) (RunDescription, error) {
	children, ok := h.children(req.Spec.Body)
	if !ok {
		return RunDescription{}, fmt.Errorf(
			"target %q has %s body, want %s",
			req.Spec.Name,
			req.Spec.TargetType(),
			h.label,
		)
	}
	return RunDescription{
		Operation: h.label + ": " + strings.Join(children, h.separator),
	}, nil
}

func (h compositeTargetHandler) Execute(context.Context, ExecuteRequest) error {
	return fmt.Errorf("%s targets are orchestrated by runner", h.label)
}

func (h compositeTargetHandler) FingerprintParts(body model.TargetBody) map[string]string {
	children, _ := h.children(body)
	return map[string]string{h.fingerprintKey: strings.Join(children, "\x00")}
}

func (h compositeTargetHandler) CompositeChildren(body model.TargetBody) []CompositeChild {
	values, _ := h.children(body)
	children := make([]CompositeChild, 0, len(values))
	for _, value := range values {
		children = append(children, CompositeChild{Target: value, Kind: h.childKind})
	}
	return children
}
