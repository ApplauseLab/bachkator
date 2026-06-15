package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDecodesPromptBlocks(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "prompts", "implementer.md"), "implementer prompt\n")
	writeTestFile(t, filepath.Join(dir, "Bachfile"), `project "example" {}

prompt "implementer" {
  path        = "prompts/implementer.md"
  description = "Default implementer prompt"
  version     = "v1"
}

shell "test" {
  command = ["true"]
}
`)

	project, err := Load(filepath.Join(dir, "Bachfile"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	prompt := project.Prompts["implementer"]
	if prompt == nil || prompt.Path != "prompts/implementer.md" ||
		prompt.Description != "Default implementer prompt" || prompt.Version != "v1" {
		t.Fatalf("prompt = %#v", prompt)
	}
}
func TestLoadRejectsMissingPromptPath(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "Bachfile"), `project "example" {}

prompt "implementer" {
  path = "prompts/missing.md"
}

shell "test" {
  command = ["true"]
}
`)

	_, err := Load(filepath.Join(dir, "Bachfile"))
	if err == nil || !strings.Contains(
		err.Error(),
		`prompt "implementer" path "prompts/missing.md"`,
	) {
		t.Fatalf("Load() error = %v, want missing prompt path", err)
	}
}
func TestLoadRejectsAbsolutePromptPath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	promptPath := filepath.Join(dir, "prompt.md")
	writeTestFile(t, promptPath, "prompt\n")
	writeTestFile(t, filepath.Join(dir, "Bachfile"), `project "example" {}

prompt "absolute" {
  path = "`+promptPath+`"
}
`)

	_, err := Load(filepath.Join(dir, "Bachfile"))
	if err == nil ||
		!strings.Contains(err.Error(), "prompt path must stay under the project root") {
		t.Fatalf("Load() error = %v, want prompt path containment error", err)
	}
}
