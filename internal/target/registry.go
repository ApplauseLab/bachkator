package target

import (
	"context"
	"fmt"
	"io"

	"github.com/applause/bachkator/internal/model"
)

type TargetHandler interface {
	Type() model.TargetType
	Runnable(model.TargetSpec) bool
	Describe(context.Context, DescribeRequest) (RunDescription, error)
	Execute(context.Context, ExecuteRequest) error
	FingerprintParts(model.TargetBody) map[string]string
}

type DescribeRequest struct {
	Spec model.TargetSpec
	Env  map[string]string
}

type ExecuteRequest struct {
	Spec    model.TargetSpec
	Env     map[string]string
	WorkDir string
	Stdout  io.Writer
	Stderr  io.Writer
}

type RunDescription struct {
	Operation string
	WorkDir   string
}

type TargetRegistry struct {
	handlers map[model.TargetType]TargetHandler
}

func NewTargetRegistry() *TargetRegistry {
	return &TargetRegistry{handlers: map[model.TargetType]TargetHandler{}}
}

func (r *TargetRegistry) Register(handler TargetHandler) error {
	if handler == nil {
		return fmt.Errorf("target handler is nil")
	}
	targetType := handler.Type()
	if targetType == "" {
		return fmt.Errorf("target handler has no type")
	}
	if _, exists := r.handlers[targetType]; exists {
		return fmt.Errorf("target type %q already registered", targetType)
	}
	r.handlers[targetType] = handler
	return nil
}

func (r *TargetRegistry) Handler(targetType model.TargetType) (TargetHandler, error) {
	handler, ok := r.handlers[targetType]
	if !ok {
		return nil, fmt.Errorf("no target handler registered for %q", targetType)
	}
	return handler, nil
}

func BuiltinTargetRegistry() *TargetRegistry {
	registry := NewTargetRegistry()
	mustRegisterTargetHandler(registry, shellHandler{})
	mustRegisterTargetHandler(registry, imageHandler{})
	mustRegisterTargetHandler(registry, pipelineHandler{})
	mustRegisterTargetHandler(registry, groupHandler{})
	return registry
}

func mustRegisterTargetHandler(registry *TargetRegistry, handler TargetHandler) {
	if err := registry.Register(handler); err != nil {
		panic(err)
	}
}
