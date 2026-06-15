package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/applauselab/bachkator/internal/bacherr"
	"github.com/applauselab/bachkator/internal/cli"
)

type initProviderRunner func(context.Context, string, []string, io.Writer, io.Writer) error

func initProject(
	ctx context.Context,
	opts cli.InitOptions,
	stdout io.Writer,
	stderr io.Writer,
) error {
	return initProjectWithRunner(ctx, opts, stdout, stderr, runInitProviderCommand)
}

func initProjectWithRunner(
	ctx context.Context,
	opts cli.InitOptions,
	stdout io.Writer,
	stderr io.Writer,
	runner initProviderRunner,
) error {
	configPath := opts.ConfigPath
	if configPath == "" {
		configPath = "Bachfile"
	}
	absConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		return err
	}
	projectRoot := filepath.Dir(absConfigPath)
	agentsPath := filepath.Join(projectRoot, "AGENTS.md")

	switch opts.Provider {
	case "":
		return initPlain(opts, absConfigPath, agentsPath, projectRoot, stdout)
	case "opencode":
		return initWithOpenCode(
			ctx,
			opts,
			absConfigPath,
			agentsPath,
			projectRoot,
			stdout,
			stderr,
			runner,
		)
	default:
		return bacherr.Unsupportedf("init provider %q", opts.Provider)
	}
}

func initPlain(
	opts cli.InitOptions,
	configPath string,
	agentsPath string,
	projectRoot string,
	stdout io.Writer,
) error {
	if err := ensureInitFilesDoNotExist(configPath, agentsPath); err != nil {
		return err
	}
	if opts.DryRun {
		_, err := fmt.Fprintf(stdout, "would create %s\nwould create %s\n", configPath, agentsPath)
		return err
	}
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		return err
	}
	projectName := sanitizeProjectName(filepath.Base(projectRoot))
	if err := writeInitFiles(
		configPath,
		[]byte(starterBachfile(projectName)),
		agentsPath,
		[]byte(starterAgentsFile()),
	); err != nil {
		return err
	}
	_, err := fmt.Fprintf(stdout, "created %s\ncreated %s\n", configPath, agentsPath)
	return err
}

func initWithOpenCode(
	ctx context.Context,
	opts cli.InitOptions,
	configPath string,
	agentsPath string,
	projectRoot string,
	stdout io.Writer,
	stderr io.Writer,
	runner initProviderRunner,
) error {
	if err := ensureInitFilesDoNotExist(configPath, agentsPath); err != nil {
		return err
	}
	if opts.DryRun {
		_, err := fmt.Fprintf(
			stdout,
			"would run opencode run <prompt> in %s\nexpected outputs: %s, %s\n",
			projectRoot,
			configPath,
			agentsPath,
		)
		return err
	}
	promptDir := filepath.Join(projectRoot, ".bach", "init")
	if err := os.MkdirAll(promptDir, 0o755); err != nil {
		return err
	}
	stagingDir := filepath.Join(promptDir, "outputs")
	if err := os.RemoveAll(stagingDir); err != nil {
		return err
	}
	if err := os.MkdirAll(stagingDir, 0o755); err != nil {
		return err
	}
	stagedConfigPath := filepath.Join(stagingDir, "Bachfile")
	stagedAgentsPath := filepath.Join(stagingDir, "AGENTS.md")
	promptPath := filepath.Join(promptDir, "opencode-prompt.md")
	promptContents := []byte(opencodeInitPrompt(
		stagedConfigPath,
		stagedAgentsPath,
		configPath,
		agentsPath,
	))
	if err := os.WriteFile(promptPath, promptContents, 0o644); err != nil {
		return err
	}
	providerArgs := []string{"opencode", "run", string(promptContents)}
	if err := runner(ctx, projectRoot, providerArgs, stdout, stderr); err != nil {
		return err
	}
	if missing := missingInitFiles(stagedConfigPath, stagedAgentsPath); len(missing) > 0 {
		return bacherr.MissingInputf(
			"init provider completed but staged files are missing: %s",
			strings.Join(missing, ", "),
		)
	}
	stagedConfig, err := os.ReadFile(stagedConfigPath)
	if err != nil {
		return err
	}
	stagedAgents, err := os.ReadFile(stagedAgentsPath)
	if err != nil {
		return err
	}
	if err := writeInitFiles(configPath, stagedConfig, agentsPath, stagedAgents); err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "created %s\ncreated %s\n", configPath, agentsPath)
	return err
}

func runInitProviderCommand(
	ctx context.Context,
	workdir string,
	args []string,
	stdout io.Writer,
	stderr io.Writer,
) error {
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = workdir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

func ensureInitFilesDoNotExist(paths ...string) error {
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("refusing to overwrite existing file %s", path)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func writeInitFiles(
	configPath string,
	configContents []byte,
	agentsPath string,
	agentsContents []byte,
) error {
	if err := writeNewFile(configPath, configContents); err != nil {
		return err
	}
	if err := writeNewFile(agentsPath, agentsContents); err != nil {
		_ = os.Remove(configPath)
		return err
	}
	return nil
}

func writeNewFile(path string, contents []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("refusing to overwrite existing file %s", path)
		}
		return err
	}
	if _, err := file.Write(contents); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return err
	}
	return file.Close()
}

func missingInitFiles(paths ...string) []string {
	missing := []string{}
	for _, path := range paths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			missing = append(missing, path)
		}
	}
	return missing
}

func sanitizeProjectName(name string) string {
	var out strings.Builder
	previousDash := false
	for _, r := range strings.ToLower(name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			out.WriteRune(r)
			previousDash = false
			continue
		}
		if !previousDash && out.Len() > 0 {
			out.WriteByte('-')
			previousDash = true
		}
	}
	result := strings.Trim(out.String(), "-")
	if result == "" {
		return "project"
	}
	return result
}

func starterBachfile(projectName string) string {
	return fmt.Sprintf(`project %q {
	root = "."
}
`, projectName)
}

func starterAgentsFile() string {
	return `# Agent Instructions

Use Bach for project operations in this repository.

- Start by running ` + "`bach list`" + `.
- Inspect expensive or side-effecting targets with ` + "`bach run --dry-run <target>`" + `.
- Use ` + "`bach affected`" + ` after edits to choose focused validation.
- Use ` + "`bach reference`" + ` and ` + "`bach reference <topic>`" + ` before guessing Bachfile syntax.
- Prefer configured Bach targets over raw package-manager, test, lint, build, docs, or release commands.
- If no useful targets exist yet, add targets to the Bachfile before documenting standalone commands.
- Do not commit ` + "`.bach/`" + ` artifacts.
`
}

func opencodeInitPrompt(
	stagedConfigPath string,
	stagedAgentsPath string,
	finalConfigPath string,
	finalAgentsPath string,
) string {
	return strings.Join([]string{
		"# Bach Init Provider Task",
		"",
		"Inspect this repository and create staged Bach adoption files.",
		"",
		"Required staged outputs:",
		"",
		fmt.Sprintf("- %s", stagedConfigPath),
		fmt.Sprintf("- %s", stagedAgentsPath),
		"",
		"Bach will install those staged files to:",
		"",
		fmt.Sprintf("- %s", finalConfigPath),
		fmt.Sprintf("- %s", finalAgentsPath),
		"",
		"Rules:",
		"",
		"- Do not make destructive edits.",
		"- Do not overwrite existing files unless the user explicitly asked for that in this session.",
		"- Start by inspecting project manifests such as Makefile, package-manager manifests, language manifests, CI workflows, scripts, and README setup sections.",
		"- Query `bach reference` and relevant `bach reference <topic>` entries before writing Bachfile syntax.",
		"- Create initial `shell` targets only for commands that are actually supported by the project.",
		"- Mark remote, destructive, or confirmation-requiring operations appropriately.",
		"- Add inputs and outputs where obvious, but do not overfit.",
		"- If no operations are discoverable, create a minimal Bachfile with only a `project` block.",
		"- Create AGENTS.md with project-specific Bach target names once targets are known.",
		"- Run `bach validate` or `bach list` after writing files if available, and report any blocker clearly.",
		"",
	}, "\n")
}
