package app

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/applauselab/bachkator/internal/cli"
)

func TestPlainInitWritesStarterFiles(t *testing.T) {
	dir := t.TempDir()
	var stdout bytes.Buffer

	if err := initProjectWithRunner(
		context.Background(),
		cli.InitOptions{ConfigPath: filepath.Join(dir, "Bachfile")},
		&stdout,
		&bytes.Buffer{},
		nil,
	); err != nil {
		t.Fatal(err)
	}

	bachfile := readTestFile(t, filepath.Join(dir, "Bachfile"))
	if !strings.Contains(bachfile, `project "`+filepath.Base(dir)+`"`) ||
		!strings.Contains(bachfile, `root = "."`) || strings.Contains(bachfile, `state =`) {
		t.Fatalf("unexpected Bachfile:\n%s", bachfile)
	}
	agents := readTestFile(t, filepath.Join(dir, "AGENTS.md"))
	if !strings.Contains(agents, "bach list") ||
		!strings.Contains(agents, "bach reference <topic>") {
		t.Fatalf("unexpected AGENTS.md:\n%s", agents)
	}
	if !strings.Contains(stdout.String(), "created") {
		t.Fatalf("stdout = %q, want created paths", stdout.String())
	}
}

func TestPlainInitDryRunDoesNotWriteFiles(t *testing.T) {
	dir := t.TempDir()
	var stdout bytes.Buffer

	if err := initProjectWithRunner(
		context.Background(),
		cli.InitOptions{ConfigPath: filepath.Join(dir, "Bachfile"), DryRun: true},
		&stdout,
		&bytes.Buffer{},
		nil,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "Bachfile")); !os.IsNotExist(err) {
		t.Fatalf("Bachfile stat error = %v, want missing", err)
	}
	if !strings.Contains(stdout.String(), "would create") {
		t.Fatalf("stdout = %q, want dry-run plan", stdout.String())
	}
}

func TestPlainInitDryRunRefusesExistingFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := initProjectWithRunner(
		context.Background(),
		cli.InitOptions{ConfigPath: filepath.Join(dir, "Bachfile"), DryRun: true},
		&bytes.Buffer{},
		&bytes.Buffer{},
		nil,
	)
	if err == nil || !strings.Contains(err.Error(), "refusing to overwrite existing file") {
		t.Fatalf("init error = %v, want overwrite refusal", err)
	}
}

func TestInitRefusesExistingFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Bachfile"), []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := initProjectWithRunner(
		context.Background(),
		cli.InitOptions{ConfigPath: filepath.Join(dir, "Bachfile")},
		&bytes.Buffer{},
		&bytes.Buffer{},
		nil,
	)
	if err == nil || !strings.Contains(err.Error(), "refusing to overwrite existing file") {
		t.Fatalf("init error = %v, want overwrite refusal", err)
	}
}

func TestPlainInitCleansUpPartialWrite(t *testing.T) {
	dir := t.TempDir()
	bachfilePath := filepath.Join(dir, "Bachfile")

	err := writeInitFiles(
		bachfilePath,
		[]byte(starterBachfile("example")),
		filepath.Join(dir, "missing", "AGENTS.md"),
		[]byte(starterAgentsFile()),
	)
	if err == nil {
		t.Fatal("init succeeded, want AGENTS.md write failure")
	}
	if _, err := os.Stat(bachfilePath); !os.IsNotExist(err) {
		t.Fatalf("Bachfile stat error = %v, want cleanup", err)
	}
}

func TestUnknownInitProviderFails(t *testing.T) {
	err := initProjectWithRunner(
		context.Background(),
		cli.InitOptions{ConfigPath: filepath.Join(t.TempDir(), "Bachfile"), Provider: "other"},
		&bytes.Buffer{},
		&bytes.Buffer{},
		nil,
	)
	if err == nil || !strings.Contains(err.Error(), `init provider "other"`) {
		t.Fatalf("init error = %v, want unsupported provider error", err)
	}
}

func TestProviderDryRunDoesNotWriteOrInvoke(t *testing.T) {
	dir := t.TempDir()
	var stdout bytes.Buffer
	called := false

	if err := initProjectWithRunner(
		context.Background(),
		cli.InitOptions{
			ConfigPath: filepath.Join(dir, "Bachfile"),
			Provider:   "opencode",
			DryRun:     true,
		},
		&stdout,
		&bytes.Buffer{},
		func(context.Context, string, []string, io.Writer, io.Writer) error {
			called = true
			return nil
		},
	); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("provider was invoked for dry-run")
	}
	promptPath := filepath.Join(dir, ".bach", "init", "opencode-prompt.md")
	if _, err := os.Stat(promptPath); !os.IsNotExist(err) {
		t.Fatalf("prompt stat error = %v, want missing", err)
	}
	if !strings.Contains(stdout.String(), "would run opencode run <prompt>") {
		t.Fatalf("stdout = %q, want provider command", stdout.String())
	}
}

func TestProviderInvokesCommandWithGeneratedPrompt(t *testing.T) {
	dir := t.TempDir()
	var gotWorkdir string
	var gotArgs []string

	if err := initProjectWithRunner(
		context.Background(),
		cli.InitOptions{ConfigPath: filepath.Join(dir, "Bachfile"), Provider: "opencode"},
		&bytes.Buffer{},
		&bytes.Buffer{},
		func(ctx context.Context, workdir string, args []string, stdout io.Writer, stderr io.Writer) error {
			gotWorkdir = workdir
			gotArgs = append([]string(nil), args...)
			outputDir := filepath.Join(workdir, ".bach", "init", "outputs")
			bachfilePath := filepath.Join(outputDir, "Bachfile")
			bachfileContents := []byte(starterBachfile("example"))
			if err := os.WriteFile(bachfilePath, bachfileContents, 0o644); err != nil {
				return err
			}
			agentsPath := filepath.Join(outputDir, "AGENTS.md")
			return os.WriteFile(agentsPath, []byte(starterAgentsFile()), 0o644)
		},
	); err != nil {
		t.Fatal(err)
	}

	if gotWorkdir != dir {
		t.Fatalf("workdir = %q, want %q", gotWorkdir, dir)
	}
	if len(gotArgs) != 3 || gotArgs[0] != "opencode" || gotArgs[1] != "run" {
		t.Fatalf("provider args = %#v", gotArgs)
	}
	prompt := gotArgs[2]
	if !strings.Contains(prompt, "Create initial `shell` targets") ||
		!strings.Contains(prompt, filepath.Join(dir, ".bach", "init", "outputs", "Bachfile")) ||
		!strings.Contains(prompt, filepath.Join(dir, "Bachfile")) {
		t.Fatalf("unexpected prompt:\n%s", prompt)
	}
	if _, err := os.Stat(filepath.Join(dir, ".bach", "init", "opencode-prompt.md")); err != nil {
		t.Fatalf("prompt file stat error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "Bachfile")); err != nil {
		t.Fatalf("final Bachfile stat error: %v", err)
	}
}

func TestProviderRequiresGeneratedFiles(t *testing.T) {
	dir := t.TempDir()
	err := initProjectWithRunner(
		context.Background(),
		cli.InitOptions{ConfigPath: filepath.Join(dir, "Bachfile"), Provider: "opencode"},
		&bytes.Buffer{},
		&bytes.Buffer{},
		func(context.Context, string, []string, io.Writer, io.Writer) error { return nil },
	)
	if err == nil || !strings.Contains(err.Error(), "staged files are missing") {
		t.Fatalf("init error = %v, want missing files error", err)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(contents)
}
