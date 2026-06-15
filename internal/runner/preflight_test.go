package runner

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCollectToolRequirementsIncludesPipelineSteps(t *testing.T) {
	project := &Project{Targets: map[string]*Target{
		"shell/render": shellTarget("shell/render", "", withTool(ToolRequirement{Name: "kubectl"})),
		"shell/apply": shellTarget(
			"shell/apply",
			"",
			withTool(ToolRequirement{Name: "aws", Fix: "Run aws sso login."}),
		),
		"pipeline/deploy": pipelineTarget(
			"pipeline/deploy",
			[]string{"shell/render", "shell/apply"},
		),
	}}

	plan, err := BuildPlan(project, "pipeline/deploy")
	if err != nil {
		t.Fatal(err)
	}
	requirements := plan.Tools
	if len(requirements) != 2 {
		t.Fatalf("requirements = %#v, want 2", requirements)
	}
	if requirements[0].Tool.Name != "kubectl" ||
		strings.Join(requirements[0].Targets, ",") != "shell/render" {
		t.Fatalf("first requirement = %#v", requirements[0])
	}
	if requirements[1].Tool.Name != "aws" ||
		strings.Join(requirements[1].Targets, ",") != "shell/apply" {
		t.Fatalf("second requirement = %#v", requirements[1])
	}
}

func TestCollectPreflightChecksIncludesPipelineSteps(t *testing.T) {
	project := &Project{Targets: map[string]*Target{
		"shell/render": shellTarget(
			"shell/render",
			"",
			withPreflight(
				PreflightCheck{Name: "registry session", Command: []string{"sh", "-c", "true"}},
			),
		),
		"shell/apply": shellTarget(
			"shell/apply",
			"",
			withPreflight(
				PreflightCheck{
					Kind:    "cloud-session",
					Command: []string{"sh", "-c", "true"},
					Fix:     "Refresh cloud session.",
				},
			),
		),
		"pipeline/deploy": pipelineTarget(
			"pipeline/deploy",
			[]string{"shell/render", "shell/apply"},
		),
	}}

	plan, err := BuildPlan(project, "pipeline/deploy")
	if err != nil {
		t.Fatal(err)
	}
	preflights := plan.Preflights
	if len(preflights) != 2 {
		t.Fatalf("preflights = %#v, want 2", preflights)
	}
	if preflights[0].Preflight.Name != "registry session" ||
		strings.Join(preflights[0].Targets, ",") != "shell/render" {
		t.Fatalf("first preflight = %#v", preflights[0])
	}
	if preflights[1].Preflight.Kind != "cloud-session" ||
		strings.Join(preflights[1].Targets, ",") != "shell/apply" {
		t.Fatalf("second preflight = %#v", preflights[1])
	}
}

func TestRunnerFailsBeforeCommandWhenRequiredToolMissing(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"shell/deploy": shellTarget(
				"shell/deploy",
				"printf ran > ran.txt",
				withTool(
					ToolRequirement{Name: "bach-missing-tool", Fix: "Install bach-missing-tool."},
				),
			),
		},
	}

	var out bytes.Buffer
	err := (&Runner{Stdout: &out, Stderr: &out}).Run(context.Background(), project, "shell/deploy")
	if err == nil || !strings.Contains(err.Error(), "required tool checks failed") ||
		!strings.Contains(err.Error(), "bach-missing-tool") ||
		!strings.Contains(err.Error(), "Install bach-missing-tool.") {
		t.Fatalf("error = %v, want required tool failure", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "ran.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("preflight failure still ran target, stat error = %v", statErr)
	}
	if !strings.Contains(
		out.String(),
		"targets: success=0 cached=0 failed=1 preflight-failed=0 quality-failed=0 dry-run=0 running=0",
	) {
		t.Fatalf("summary missing failed target count:\n%s", out.String())
	}
	logBytes, readErr := os.ReadFile(
		filepath.Join(dir, ".bach", "runs", latestRunID(t, dir), "shell__deploy.log"),
	)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !strings.Contains(string(logBytes), "required tool check failed") {
		t.Fatalf("log missing tool-check reason:\n%s", string(logBytes))
	}
}

func TestRunnerDryRunReportsRequiredToolsWithoutCheckingHost(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"shell/deploy": shellTarget(
				"shell/deploy",
				"printf ran > ran.txt",
				withTool(
					ToolRequirement{
						Name:    "bach-missing-tool",
						Command: []string{"bach-missing-tool", "--version"},
						Version: "v1",
						Fix:     "Install it.",
					},
				),
			),
		},
	}

	var out bytes.Buffer
	if err := (&Runner{DryRun: true, Stdout: &out, Stderr: &out}).Run(
		context.Background(),
		project,
		"shell/deploy",
	); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(
		got,
		"required tool bach-missing-tool (v1) via bach-missing-tool --version - Install it. for shell/deploy",
	) {
		t.Fatalf("dry-run output missing required tool:\n%s", got)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "ran.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("dry-run wrote output, stat error = %v", statErr)
	}
}

func TestRunnerFailsBeforeCommandWhenPreflightFails(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"shell/deploy": shellTarget(
				"shell/deploy",
				"printf ran > ran.txt",
				withPreflight(
					PreflightCheck{
						Name:    "cloud session",
						Command: []string{"sh", "-c", "printf expired; exit 4"},
						Fix:     "Run cloud login.",
					},
				),
			),
		},
	}

	var out bytes.Buffer
	err := (&Runner{Stdout: &out, Stderr: &out}).Run(context.Background(), project, "shell/deploy")
	if err == nil || !strings.Contains(err.Error(), "credential/session preflights failed") ||
		!strings.Contains(err.Error(), "cloud session") ||
		!strings.Contains(err.Error(), "Run cloud login.") {
		t.Fatalf("error = %v, want preflight failure", err)
	}
	if strings.Contains(err.Error(), "expired") {
		t.Fatalf("preflight error leaked probe output: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "ran.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("preflight failure still ran target, stat error = %v", statErr)
	}
	if !strings.Contains(out.String(), "run ") ||
		!strings.Contains(out.String(), " preflight-failed target=shell/deploy ") {
		t.Fatalf("summary missing preflight-failed run status:\n%s", out.String())
	}
	if !strings.Contains(
		out.String(),
		"targets: success=0 cached=0 failed=0 preflight-failed=1 quality-failed=0 dry-run=0 running=0",
	) {
		t.Fatalf("summary missing preflight-failed target count:\n%s", out.String())
	}
	logBytes, readErr := os.ReadFile(
		filepath.Join(dir, ".bach", "runs", latestRunID(t, dir), "shell__deploy.log"),
	)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !strings.Contains(string(logBytes), "credential/session preflight failed") ||
		strings.Contains(string(logBytes), "expired") ||
		strings.Contains(string(logBytes), "ran.txt") {
		t.Fatalf("log missing preflight reason or leaked command output:\n%s", string(logBytes))
	}
}

func TestRunnerDryRunReportsPreflightsWithoutCheckingHost(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"shell/deploy": shellTarget(
				"shell/deploy",
				"printf ran > ran.txt",
				withPreflight(
					PreflightCheck{
						Kind:    "session",
						Command: []string{"bach-missing-session-check"},
						Fix:     "Refresh it.",
					},
				),
			),
		},
	}

	var out bytes.Buffer
	if err := (&Runner{DryRun: true, Stdout: &out, Stderr: &out}).Run(
		context.Background(),
		project,
		"shell/deploy",
	); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "preflight session via bach-missing-session-check for shell/deploy") {
		t.Fatalf("dry-run output missing preflight:\n%s", got)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "ran.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("dry-run wrote output, stat error = %v", statErr)
	}
}

func TestRunnerFailsWhenRequiredToolProbeFails(t *testing.T) {
	dir := t.TempDir()
	project := &Project{
		Root:      dir,
		StatePath: filepath.Join(dir, ".bach", "state.db"),
		Targets: map[string]*Target{
			"shell/deploy": shellTarget(
				"shell/deploy",
				"printf ran > ran.txt",
				withTool(
					ToolRequirement{
						Name:    "sh",
						Command: []string{"sh", "-c", "printf bad-probe; exit 9"},
					},
				),
			),
		},
	}

	var out bytes.Buffer
	err := (&Runner{Stdout: &out, Stderr: &out}).Run(context.Background(), project, "shell/deploy")
	if err == nil || !strings.Contains(err.Error(), "probe failed: bad-probe") {
		t.Fatalf("error = %v, want probe failure output", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "ran.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("preflight failure still ran target, stat error = %v", statErr)
	}
}

func latestRunID(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(dir, ".bach", "runs"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("run dirs = %d, want 1", len(entries))
	}
	return entries[0].Name()
}
