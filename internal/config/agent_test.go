package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDecodesOpenCodeProviderWithoutCommand(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Bachfile")
	contents := `project "example" {}

provider "opencode" {
  type = "opencode"
}

prompt "implementer" {
  path = "prompt.md"
}

agent "example" {
  mode     = "implement"
  provider = provider.opencode
  prompt   = prompt.implementer
  plan     = "plan.md"
}
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(dir, "prompt.md"),
		[]byte("implement\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plan.md"), []byte("plan\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	project, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	provider := project.Providers["provider/opencode"]
	if provider == nil || provider.Type != "opencode" || len(provider.Command) != 0 {
		t.Fatalf("provider = %#v", provider)
	}
}
func TestLoadRejectsOpenCodeProviderCommand(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Bachfile")
	contents := `project "example" {}

provider "opencode" {
  type    = "opencode"
  command = ["opencode", "run"]
}
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "command is not supported") {
		t.Fatalf("Load() error = %v, want unsupported command", err)
	}
}
func TestLoadRejectsOpenCodeProviderForMergeMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Bachfile")
	contents := `project "example" {}

provider "opencode" {
  type = "opencode"
}

agent "merge_subject" {
  mode     = "merge"
  provider = provider.opencode
  subject  = agent.example
}

agent "example" {
  mode     = "implement"
  provider = provider.opencode
  plan     = "plan.md"
}
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plan.md"), []byte("plan\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil || !strings.Contains(
		err.Error(),
		"opencode provider is supported only for implement mode",
	) {
		t.Fatalf("Load() error = %v, want implement-only opencode error", err)
	}
}
func TestLoadMergeSubjectRecordsExactPolicyTarget(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "Bachfile"), `project "example" {}

provider "local" {
  type    = "agent"
  command = ["true"]
}

shell "test" {
  command = ["true"]
}

policy "accept" {
  required_targets = [shell.test]
}

agent "subject" {
  mode     = "implement"
  provider = provider.local
  plan     = "plan.md"
  policy   = policy.accept
}

agent "merge_subject" {
  mode     = "merge"
  provider = provider.local
  subject  = agent.subject
}
`)
	writeTestFile(t, filepath.Join(dir, "plan.md"), "plan\n")

	project, err := Load(filepath.Join(dir, "Bachfile"))
	if err != nil {
		t.Fatal(err)
	}
	merge := project.Targets["agent/merge_subject"]
	if merge == nil {
		t.Fatal("merge target missing")
	}
	if got := merge.AgentSubject.PolicyTarget; got != "policy/accept@agent.subject" {
		t.Fatalf("policy target = %q, want policy/accept@agent.subject", got)
	}
}
