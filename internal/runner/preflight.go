package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/applauselab/bachkator/internal/model"
)

type PlannedToolRequirement struct {
	Tool    model.ToolRequirement
	Targets []string
}

type PlannedPreflightCheck struct {
	Preflight model.PreflightCheck
	Targets   []string
}

type ToolCheckFailure struct {
	Tool    model.ToolRequirement
	Targets []string
	Reason  string
}

type ToolCheckError struct {
	Failures []ToolCheckFailure
}

type PreflightFailure struct {
	Preflight model.PreflightCheck
	Targets   []string
	Reason    string
}

type PreflightCheckError struct {
	Failures []PreflightFailure
}

func (e ToolCheckError) Error() string {
	var out strings.Builder
	out.WriteString("required tool checks failed:\n")
	for _, failure := range e.Failures {
		fmt.Fprintf(
			&out,
			"- %s for %s: %s",
			failure.Tool.Name,
			strings.Join(failure.Targets, ", "),
			failure.Reason,
		)
		if failure.Tool.Version != "" {
			fmt.Fprintf(&out, " (version: %s)", failure.Tool.Version)
		}
		if failure.Tool.Fix != "" {
			fmt.Fprintf(&out, "; %s", failure.Tool.Fix)
		}
		out.WriteByte('\n')
	}
	return strings.TrimRight(out.String(), "\n")
}

func (e PreflightCheckError) Error() string {
	var out strings.Builder
	out.WriteString("credential/session preflights failed:\n")
	for _, failure := range e.Failures {
		fmt.Fprintf(
			&out,
			"- %s for %s: %s",
			failure.Preflight.Label(),
			strings.Join(failure.Targets, ", "),
			failure.Reason,
		)
		if failure.Preflight.Fix != "" {
			fmt.Fprintf(&out, "; %s", failure.Preflight.Fix)
		}
		out.WriteByte('\n')
	}
	return strings.TrimRight(out.String(), "\n")
}

func reportOrCheckPlannedRequiredTools(
	ctx context.Context,
	stdout io.Writer,
	dryRun bool,
	requirements []PlannedToolRequirement,
) error {
	if len(requirements) == 0 {
		return nil
	}
	if dryRun {
		for _, requirement := range requirements {
			if _, err := fmt.Fprintf(
				stdout,
				"required tool %s for %s\n",
				formatToolRequirement(requirement.Tool),
				strings.Join(requirement.Targets, ", "),
			); err != nil {
				return err
			}
		}
		return nil
	}

	var failures []ToolCheckFailure
	for _, requirement := range requirements {
		if reason := checkToolRequirement(ctx, requirement.Tool); reason != "" {
			failures = append(
				failures,
				ToolCheckFailure{
					Tool:    requirement.Tool,
					Targets: requirement.Targets,
					Reason:  reason,
				},
			)
		}
	}
	if len(failures) == 0 {
		return nil
	}
	return ToolCheckError{Failures: failures}
}

func reportOrCheckPlannedPreflights(
	ctx context.Context,
	stdout io.Writer,
	dryRun bool,
	preflights []PlannedPreflightCheck,
) error {
	if len(preflights) == 0 {
		return nil
	}
	if dryRun {
		for _, preflight := range preflights {
			if _, err := fmt.Fprintf(
				stdout,
				"preflight %s via %s for %s\n",
				preflight.Preflight.Label(),
				strings.Join(preflight.Preflight.Command, " "),
				strings.Join(preflight.Targets, ", "),
			); err != nil {
				return err
			}
		}
		return nil
	}

	var failures []PreflightFailure
	for _, preflight := range preflights {
		if reason := runPreflightCheck(ctx, preflight.Preflight); reason != "" {
			failures = append(
				failures,
				PreflightFailure{
					Preflight: preflight.Preflight,
					Targets:   preflight.Targets,
					Reason:    reason,
				},
			)
		}
	}
	if len(failures) == 0 {
		return nil
	}
	return PreflightCheckError{Failures: failures}
}

func checkToolRequirement(ctx context.Context, tool model.ToolRequirement) string {
	command := tool.Command
	if len(command) == 0 {
		if _, err := exec.LookPath(tool.Name); err != nil {
			return "missing from PATH"
		}
		return ""
	}
	if _, err := exec.LookPath(command[0]); err != nil {
		return fmt.Sprintf("probe command %q missing from PATH", command[0])
	}
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	output, err := cmd.CombinedOutput()
	if err == nil {
		return ""
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		return ctx.Err().Error()
	}
	message := strings.TrimSpace(string(output))
	if message == "" {
		message = err.Error()
	}
	return "probe failed: " + message
}

func runPreflightCheck(ctx context.Context, preflight model.PreflightCheck) string {
	command := preflight.Command
	if _, err := exec.LookPath(command[0]); err != nil {
		return fmt.Sprintf("probe command %q missing from PATH", command[0])
	}
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	err := cmd.Run()
	if err == nil {
		return ""
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		return ctx.Err().Error()
	}
	return "probe failed: " + err.Error()
}

func formatToolRequirement(tool model.ToolRequirement) string {
	label := tool.Name
	if tool.Version != "" {
		label += " (" + tool.Version + ")"
	}
	if len(tool.Command) > 0 {
		label += " via " + strings.Join(tool.Command, " ")
	}
	if tool.Fix != "" {
		label += " - " + tool.Fix
	}
	return label
}
