package runner

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/applauselab/bachkator/internal/model"
	targetpkg "github.com/applauselab/bachkator/internal/target"
)

type fakeTargetHandlers struct {
	handler targetpkg.TargetHandler
}

func (f fakeTargetHandlers) Handler(model.TargetType) (targetpkg.TargetHandler, error) {
	return f.handler, nil
}

type fakeTargetHandler struct {
	operation        string
	fingerprintParts map[string]string
	childShell       string
	children         []targetpkg.CompositeChild
}

func (fakeTargetHandler) Type() model.TargetType { return model.TargetTypeShell }

func (fakeTargetHandler) Runnable(model.TargetSpec) bool { return false }

func (h fakeTargetHandler) Describe(
	context.Context,
	targetpkg.DescribeRequest,
) (targetpkg.RunDescription, error) {
	operation := h.operation
	if operation == "" {
		operation = "fake injected operation"
	}
	return targetpkg.RunDescription{Operation: operation}, nil
}

func (fakeTargetHandler) Execute(context.Context, targetpkg.ExecuteRequest) error { return nil }

func (h fakeTargetHandler) FingerprintParts(model.TargetBody) map[string]string {
	return h.fingerprintParts
}

func (h fakeTargetHandler) CompositeChildren(
	targetBody model.TargetBody,
) []targetpkg.CompositeChild {
	if h.childShell != "" {
		body, _ := targetBody.(model.ShellSpec)
		if body.Shell != h.childShell {
			return nil
		}
	}
	return h.children
}

func TestBuildPlanUsesInjectedTargetHandlerCompositeChildren(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"parent": shellTarget("parent", "printf parent"),
			"child":  shellTarget("child", "printf child"),
		},
	}
	plan, err := BuildPlanForTargetsWithHandlers(
		project,
		[]string{"parent"},
		fakeTargetHandlers{
			handler: fakeTargetHandler{
				childShell: "printf parent",
				children:   []targetpkg.CompositeChild{{Target: "child", Kind: "fake_child"}},
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Target("child") == nil {
		t.Fatalf("expected injected composite child to be included in plan closure")
	}
	if len(plan.CompositeEdges) != 1 || plan.CompositeEdges[0].Kind != "fake_child" {
		t.Fatalf("unexpected composite edges: %#v", plan.CompositeEdges)
	}
}

func TestRunnerUsesInjectedTargetHandlers(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"shell/test": shellTarget("shell/test", "printf builtin"),
		},
	}

	var out bytes.Buffer
	r := Runner{
		DryRun:  true,
		Stdout:  &out,
		Stderr:  &out,
		Targets: fakeTargetHandlers{handler: fakeTargetHandler{}},
	}
	if err := r.Run(context.Background(), project, "shell/test"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "fake injected operation") {
		t.Fatalf("output = %q, want fake operation", out.String())
	}
}

func TestTargetFingerprintUsesInjectedHandlerParts(t *testing.T) {
	t.Parallel()

	project := &Project{Root: t.TempDir()}
	target := shellTarget("shell/test", "printf builtin")
	first, _, err := targetFingerprintParts(
		fakeTargetHandlers{handler: fakeTargetHandler{
			fingerprintParts: map[string]string{"fake": "one"},
		}},
		project,
		target,
		nil,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := targetFingerprintParts(
		fakeTargetHandlers{handler: fakeTargetHandler{
			fingerprintParts: map[string]string{"fake": "two"},
		}},
		project,
		target,
		nil,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("fingerprint should change when injected handler parts change")
	}
}
