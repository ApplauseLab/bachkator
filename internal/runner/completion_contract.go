package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/applauselab/bachkator/internal/model"
	"github.com/applauselab/bachkator/internal/runenv"
)

func evaluateCompletionContract(
	ctx context.Context,
	project *Project,
	target *Target,
	workdir string,
	env map[string]string,
	logPath string,
	logFile io.Writer,
) error {
	spec := target.Spec()
	for _, check := range spec.Contract.FailWhen {
		matched, detail, err := evaluateCompletionCheck(
			ctx,
			project,
			check,
			workdir,
			env,
			logPath,
			logFile,
		)
		if err != nil {
			return err
		}
		if matched {
			return fmt.Errorf("target %q fail_when matched: %s", target.Name, detail)
		}
	}
	for _, check := range spec.Contract.SuccessWhen {
		matched, detail, err := evaluateCompletionCheck(
			ctx,
			project,
			check,
			workdir,
			env,
			logPath,
			logFile,
		)
		if err != nil {
			return err
		}
		if !matched {
			return fmt.Errorf("target %q success_when not satisfied: %s", target.Name, detail)
		}
	}
	return nil
}

func evaluateCompletionCheck(
	ctx context.Context,
	project *Project,
	check model.CompletionCheckSpec,
	workdir string,
	env map[string]string,
	logPath string,
	logFile io.Writer,
) (bool, string, error) {
	if check.OutputContains != "" {
		matched, err := logContains(logPath, check.OutputContains)
		return matched, "output_contains " + strconvQuote(check.OutputContains), err
	}
	if check.FileExists != "" {
		path := absPath(project.Root, check.FileExists)
		_, err := os.Stat(path)
		if err == nil {
			return true, "file_exists " + strconvQuote(check.FileExists), nil
		}
		if os.IsNotExist(err) {
			return false, "file_exists " + strconvQuote(check.FileExists), nil
		}
		return false, "", err
	}
	return runCompletionCommand(ctx, check.Command, workdir, env, logFile)
}

func logContains(logPath string, needle string) (bool, error) {
	contents, err := os.ReadFile(logPath)
	if err != nil {
		return false, err
	}
	return strings.Contains(string(contents), needle), nil
}

func runCompletionCommand(
	ctx context.Context,
	command []string,
	workdir string,
	env map[string]string,
	logFile io.Writer,
) (bool, string, error) {
	if len(command) == 0 {
		return false, "command []", nil
	}
	expanded := runenv.ExpandSlice(command, env)
	cmd := exec.CommandContext(ctx, expanded[0], expanded[1:]...)
	cmd.Dir = workdir
	cmd.Env = runenv.List(env)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	logf(logFile, "[contract] %s\n", strings.Join(expanded, " "))
	if err := cmd.Run(); err != nil {
		return false, "command " + strings.Join(expanded, " "), nil
	}
	return true, "command " + strings.Join(expanded, " "), nil
}

func strconvQuote(value string) string {
	return "\"" + strings.ReplaceAll(value, "\"", "\\\"") + "\""
}
