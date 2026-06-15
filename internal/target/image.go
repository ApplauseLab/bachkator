package target

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/applauselab/bachkator/internal/model"
	"github.com/applauselab/bachkator/internal/runenv"
)

type imageHandler struct{}

func (imageHandler) Type() model.TargetType { return model.TargetTypeImage }

func (imageHandler) Runnable(spec model.TargetSpec) bool {
	body, ok := spec.Body.(model.ImageSpec)
	return ok && hasOCIImageFields(body)
}

func (imageHandler) Describe(_ context.Context, req DescribeRequest) (RunDescription, error) {
	body, ok := req.Spec.Body.(model.ImageSpec)
	if !ok {
		return RunDescription{}, fmt.Errorf(
			"target %q has %s body, want image",
			req.Spec.Name,
			req.Spec.TargetType(),
		)
	}
	spec := req.Spec
	spec.Body = body
	commands, err := OCICommands(spec)
	if err != nil {
		return RunDescription{}, err
	}
	commandStrings := make([]string, 0, len(commands))
	for _, command := range commands {
		commandStrings = append(commandStrings, strings.Join(command, " "))
	}
	return RunDescription{Operation: strings.Join(commandStrings, " && ")}, nil
}

func (h imageHandler) Execute(ctx context.Context, req ExecuteRequest) error {
	cmds, err := h.commands(ctx, req.Spec)
	if err != nil {
		return err
	}
	for _, cmd := range cmds {
		cmd.Dir = req.WorkDir
		cmd.Env = runenv.List(req.Env)
		cmd.Stdout = req.Stdout
		cmd.Stderr = req.Stderr
		if err := cmd.Run(); err != nil {
			return err
		}
	}
	return nil
}

func (imageHandler) FingerprintParts(body model.TargetBody) map[string]string {
	image, _ := body.(model.ImageSpec)
	return map[string]string{
		"builder":    image.Builder,
		"image":      image.Image,
		"tags":       strings.Join(image.Tags, "\x00"),
		"dockerfile": image.Dockerfile,
		"context":    image.Context,
		"platform":   image.Platform,
		"push":       fmt.Sprint(image.Push),
		"build-args": strings.Join(image.BuildArgs, "\x00"),
	}
}

func (imageHandler) CompositeChildren(model.TargetBody) []CompositeChild { return nil }

func (imageHandler) commands(ctx context.Context, spec model.TargetSpec) ([]*exec.Cmd, error) {
	commands, err := OCICommands(spec)
	if err != nil {
		return nil, err
	}
	cmds := make([]*exec.Cmd, 0, len(commands))
	for _, command := range commands {
		cmds = append(cmds, exec.CommandContext(ctx, command[0], command[1:]...))
	}
	return cmds, nil
}

func OCICommands(spec model.TargetSpec) ([][]string, error) {
	build, err := OCIBuildCommand(spec)
	if err != nil {
		return nil, err
	}
	commands := [][]string{build}
	body, _ := spec.Body.(model.ImageSpec)
	if !body.Push {
		return commands, nil
	}
	builder := BuilderName(spec)
	for _, tag := range ImageTags(spec) {
		switch builder {
		case "container":
			commands = append(commands, []string{builder, "image", "push", tag})
		default:
			commands = append(commands, []string{builder, "push", tag})
		}
	}
	return commands, nil
}

func OCIBuildCommand(spec model.TargetSpec) ([]string, error) {
	body, ok := spec.Body.(model.ImageSpec)
	if !ok {
		return nil, fmt.Errorf("target %q has %s body, want image", spec.Name, spec.TargetType())
	}
	_ = body
	if shell, ok := spec.Body.(model.ShellSpec); ok &&
		(shell.Shell != "" || len(shell.Command) > 0) {
		return nil, fmt.Errorf(
			"target %q is an image target and must use image fields, not command or shell",
			spec.Name,
		)
	}
	builder := BuilderName(spec)
	contextDir := body.Context
	if contextDir == "" {
		contextDir = "."
	}
	dockerfile := body.Dockerfile
	if dockerfile == "" {
		dockerfile = "Dockerfile"
	}

	args := []string{builder, "build"}
	if body.Platform != "" {
		args = append(args, "--platform", body.Platform)
	}
	args = append(args, "--file", dockerfile)
	for _, tag := range ImageTags(spec) {
		args = append(args, "--tag", tag)
	}
	for _, buildArg := range body.BuildArgs {
		args = append(args, "--build-arg", buildArg)
	}
	args = append(args, contextDir)
	return args, nil
}

func BuilderName(spec model.TargetSpec) string {
	body, _ := spec.Body.(model.ImageSpec)
	builder := body.Builder
	if builder == "" {
		builder = os.Getenv("OCI_BUILDER")
	}
	if builder == "" {
		builder = "docker"
	}
	return builder
}

func ImageName(spec model.TargetSpec) string {
	body, _ := spec.Body.(model.ImageSpec)
	if body.Image != "" {
		return body.Image
	}
	return strings.TrimPrefix(spec.Name, "image/")
}

func ImageTags(spec model.TargetSpec) []string {
	body, _ := spec.Body.(model.ImageSpec)
	image := ImageName(spec)
	if len(body.Tags) == 0 {
		return []string{image}
	}
	tags := make([]string, 0, len(body.Tags))
	for _, tag := range body.Tags {
		if strings.Contains(tag, ":") || strings.Contains(tag, "/") {
			tags = append(tags, tag)
			continue
		}
		tags = append(tags, image+":"+tag)
	}
	return tags
}

func hasOCIImageFields(body model.ImageSpec) bool {
	return body.Builder != "" || body.Image != "" || len(body.Tags) > 0 || body.Dockerfile != "" ||
		body.Context != "" ||
		body.Platform != "" ||
		body.Push ||
		len(body.BuildArgs) > 0
}
