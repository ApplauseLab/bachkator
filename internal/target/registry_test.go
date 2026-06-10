package target

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/applause/bachkator/internal/model"
)

type testTargetHandler struct {
	targetType model.TargetType
}

func (h testTargetHandler) Type() model.TargetType { return h.targetType }

func (testTargetHandler) Runnable(model.TargetSpec) bool { return false }

func (testTargetHandler) Describe(context.Context, DescribeRequest) (RunDescription, error) {
	return RunDescription{}, nil
}

func (testTargetHandler) Execute(context.Context, ExecuteRequest) error {
	return nil
}

func (testTargetHandler) FingerprintParts(model.TargetBody) map[string]string { return nil }

func TestTargetRegistryRejectsDuplicateType(t *testing.T) {
	registry := NewTargetRegistry()
	if err := registry.Register(testTargetHandler{targetType: model.TargetTypeShell}); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(testTargetHandler{targetType: model.TargetTypeShell}); err == nil {
		t.Fatal("duplicate target type registered without error")
	}
}

func TestTargetRegistryReportsMissingHandler(t *testing.T) {
	registry := NewTargetRegistry()
	if _, err := registry.Handler(model.TargetTypeShell); err == nil {
		t.Fatal("missing target handler returned no error")
	}
}

func TestBuiltinTargetRegistryWiresShellImagePipelineAndGroup(t *testing.T) {
	registry := BuiltinTargetRegistry()
	for _, targetType := range []model.TargetType{
		model.TargetTypeShell,
		model.TargetTypeImage,
		model.TargetTypePipeline,
		model.TargetTypeGroup,
	} {
		if _, err := registry.Handler(targetType); err != nil {
			t.Fatalf("handler for %q: %v", targetType, err)
		}
	}

	shell, err := registry.Handler(model.TargetTypeShell)
	if err != nil {
		t.Fatal(err)
	}
	desc, err := shell.Describe(
		context.Background(),
		DescribeRequest{
			Spec: model.TargetSpec{
				Name: "shell/test",
				Body: model.ShellSpec{Command: []string{"go", "test", "$PKG"}},
			},
			Env: map[string]string{"PKG": "./..."},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if desc.Operation != "go test ./..." {
		t.Fatalf("shell operation = %q", desc.Operation)
	}

	image, err := registry.Handler(model.TargetTypeImage)
	if err != nil {
		t.Fatal(err)
	}
	desc, err = image.Describe(
		context.Background(),
		DescribeRequest{
			Spec: model.TargetSpec{Name: "image/app", Body: model.ImageSpec{Tags: []string{"dev"}}},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(desc.Operation, "docker build") ||
		!strings.Contains(desc.Operation, "--tag app:dev") {
		t.Fatalf("image operation = %q", desc.Operation)
	}

	pipeline, err := registry.Handler(model.TargetTypePipeline)
	if err != nil {
		t.Fatal(err)
	}
	desc, err = pipeline.Describe(
		context.Background(),
		DescribeRequest{
			Spec: model.TargetSpec{
				Name: "pipeline/all",
				Body: model.PipelineSpec{Steps: []string{"shell/a", "shell/b"}},
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if desc.Operation != "pipeline: shell/a -> shell/b" {
		t.Fatalf("pipeline operation = %q", desc.Operation)
	}

	group, err := registry.Handler(model.TargetTypeGroup)
	if err != nil {
		t.Fatal(err)
	}
	desc, err = group.Describe(
		context.Background(),
		DescribeRequest{
			Spec: model.TargetSpec{
				Name: "group/ci",
				Body: model.GroupSpec{Targets: []string{"shell/lint", "shell/test"}},
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if desc.Operation != "group: shell/lint, shell/test" {
		t.Fatalf("group operation = %q", desc.Operation)
	}
}

func TestOCIImageTargetBuildsContainerCommand(t *testing.T) {
	spec := model.TargetSpec{
		Name: "image/atelier-api",
		Body: model.ImageSpec{
			Builder:    "container",
			Image:      "atelier-api",
			Tags:       []string{"latest", "registry.example.com/atelier-api:sha"},
			Dockerfile: "Dockerfile.api",
			Context:    ".",
			Platform:   "linux/amd64",
			BuildArgs:  []string{"FOO=bar"},
		},
	}

	command, err := OCIBuildCommand(spec)
	if err != nil {
		t.Fatal(err)
	}
	want := "container build --platform linux/amd64 --file Dockerfile.api --tag atelier-api:latest --tag registry.example.com/atelier-api:sha --build-arg FOO=bar ."
	if got := strings.Join(command, " "); got != want {
		t.Fatalf("command = %q, want %q", got, want)
	}
}

func TestOCIImageTargetPushesContainerTags(t *testing.T) {
	spec := model.TargetSpec{
		Name: "image/atelier-api",
		Body: model.ImageSpec{
			Builder: "container",
			Image:   "atelier-api",
			Tags:    []string{"latest", "registry.example.com/atelier-api:sha"},
			Push:    true,
		},
	}

	commands, err := OCICommands(spec)
	if err != nil {
		t.Fatal(err)
	}
	got := commandStrings(commands)
	want := []string{
		"container build --file Dockerfile --tag atelier-api:latest --tag registry.example.com/atelier-api:sha .",
		"container image push atelier-api:latest",
		"container image push registry.example.com/atelier-api:sha",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("commands = %#v, want %#v", got, want)
	}
}

func TestOCIImageTargetPushesDockerTags(t *testing.T) {
	spec := model.TargetSpec{
		Name: "image/atelier-api",
		Body: model.ImageSpec{
			Builder: "docker",
			Image:   "atelier-api",
			Tags:    []string{"registry.example.com/atelier-api:sha"},
			Push:    true,
		},
	}

	commands, err := OCICommands(spec)
	if err != nil {
		t.Fatal(err)
	}
	got := commandStrings(commands)
	want := []string{
		"docker build --file Dockerfile --tag registry.example.com/atelier-api:sha .",
		"docker push registry.example.com/atelier-api:sha",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("commands = %#v, want %#v", got, want)
	}
}

func commandStrings(commands [][]string) []string {
	values := make([]string, 0, len(commands))
	for _, command := range commands {
		values = append(values, strings.Join(command, " "))
	}
	return values
}
