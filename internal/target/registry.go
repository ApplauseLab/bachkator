package target

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/applauselab/bachkator/internal/clock"
	"github.com/applauselab/bachkator/internal/model"
	typedregistry "github.com/applauselab/bachkator/internal/registry"
)

type TargetHandler interface {
	Type() model.TargetType
	Runnable(model.TargetSpec) bool
	Describe(context.Context, DescribeRequest) (RunDescription, error)
	Execute(context.Context, ExecuteRequest) error
	FingerprintParts(model.TargetBody) map[string]string
	CompositeChildren(model.TargetBody) []CompositeChild
}

type CompositeChild struct {
	Target string
	Kind   string
}

type DescribeRequest struct {
	Spec model.TargetSpec
	Env  map[string]string
}

type ExecuteRequest struct {
	Spec               model.TargetSpec
	Env                map[string]string
	WorkDir            string
	StatePath          string
	Stdout             io.Writer
	Stderr             io.Writer
	RunRequiredTargets func(context.Context, RequiredTargetsRequest) error
	RunPolicyTarget    func(context.Context, PolicyTargetRequest) error
	Now                clock.NowFunc
}

func (r ExecuteRequest) now() time.Time {
	return clock.UTC(r.Now)
}

type RequiredTargetsRequest struct {
	Targets       []string
	WorkDir       string
	Subject       string
	SubjectCommit string
	PolicyNode    string
}

type PolicyTargetRequest struct {
	Target string
}

type RunDescription struct {
	Operation string
	WorkDir   string
}

type TargetRegistry struct {
	handlers typedregistry.Registry[model.TargetType, TargetHandler]
}

func NewTargetRegistry() *TargetRegistry {
	return &TargetRegistry{}
}

func (r *TargetRegistry) Register(handler TargetHandler) error {
	if handler == nil {
		return fmt.Errorf("target handler is nil")
	}
	targetType := handler.Type()
	if targetType == "" {
		return fmt.Errorf("target handler has no type")
	}
	return r.handlers.Register(targetType, handler, duplicateTargetHandlerError)
}

func (r *TargetRegistry) Handler(targetType model.TargetType) (TargetHandler, error) {
	return r.handlers.Get(targetType, missingTargetHandlerError)
}

func duplicateTargetHandlerError(targetType model.TargetType) error {
	return fmt.Errorf("target type %q already registered", targetType)
}

func missingTargetHandlerError(targetType model.TargetType) error {
	return fmt.Errorf("no target handler registered for %q", targetType)
}

func BuiltinTargetRegistry() *TargetRegistry {
	registry := NewTargetRegistry()
	mustRegisterTargetHandler(registry, shellHandler{})
	mustRegisterTargetHandler(registry, agentHandler{})
	mustRegisterTargetHandler(registry, policyHandler{})
	mustRegisterTargetHandler(registry, imageHandler{})
	mustRegisterTargetHandler(registry, pipelineHandler())
	mustRegisterTargetHandler(registry, groupHandler())
	return registry
}

func mustRegisterTargetHandler(registry *TargetRegistry, handler TargetHandler) {
	if err := registry.Register(handler); err != nil {
		panic(err)
	}
}
