package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDecodesVariables(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Bachfile")
	contents := `project "example" {}

var "github_repo" {
  default = "applause/bachkator"
}

shell "release" {
  command = ["gh", "release", "create", "v0.1.0", "--repo", var.github_repo]
}
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	project, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	command := project.Targets["shell/release"].Command
	if got := command[len(command)-1]; got != "applause/bachkator" {
		t.Fatalf("repo = %q, want applause/bachkator", got)
	}
}
func TestLoadVariableOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Bachfile")
	contents := `project "example" {}

var "github_repo" {
  default = "applause/bachkator"
}

shell "release" {
  command = ["gh", "release", "create", "v0.1.0", "--repo", var.github_repo]
}
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	project, err := LoadWithOptions(
		path,
		LoadOptions{Variables: map[string]string{"github_repo": "example/repo"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	command := project.Targets["shell/release"].Command
	if got := command[len(command)-1]; got != "example/repo" {
		t.Fatalf("repo = %q, want example/repo", got)
	}
}
func TestLoadVariableEnvFallback(t *testing.T) {
	t.Setenv("BACH_VAR_github_repo", "env/repo")
	dir := t.TempDir()
	path := filepath.Join(dir, "Bachfile")
	contents := `project "example" {}

var "github_repo" {}

shell "release" {
  command = ["gh", "release", "create", "v0.1.0", "--repo", var.github_repo]
}
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	project, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	command := project.Targets["shell/release"].Command
	if got := command[len(command)-1]; got != "env/repo" {
		t.Fatalf("repo = %q, want env/repo", got)
	}
}
func TestLoadDecodesProjectEnvAndInterpolatedVariables(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Bachfile")
	contents := `project "example" {}

var bla {}

var "foo" {
  default = "foo"
}

var foobar {
  default = "${var.foo}bar"
}

env {
  ENV_2 = "${ENV_1} b ${var.bla} ${var.foobar}"
  ENV_1 = "a"
}

shell "show" {
  command = ["printf", ENV_2]
}
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	project, err := LoadWithOptions(path, LoadOptions{Variables: map[string]string{"bla": "x"}})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := project.Variables["foobar"], "foobar"; got != want {
		t.Fatalf("foobar = %q, want %q", got, want)
	}
	wantEnv := []string{"ENV_1=a", "ENV_2=a b x foobar"}
	if len(project.Env) != len(wantEnv) {
		t.Fatalf("project env = %v, want %v", project.Env, wantEnv)
	}
	for index, want := range wantEnv {
		if project.Env[index] != want {
			t.Fatalf("project env = %v, want %v", project.Env, wantEnv)
		}
	}
	if got := project.Targets["shell/show"].Command[1]; got != "a b x foobar" {
		t.Fatalf("command env ref = %q, want interpolated ENV_2", got)
	}
}
func TestLoadDecodesComputedVariables(t *testing.T) {
	dir := initGitRepo(t)
	writeFile(t, dir, "package.json", `{"name":"example"}`)
	writeFile(t, dir, "Bachfile", `project "example" {}

var "image_tag" {
  default = "${git_short_sha()}${git_dirty_suffix()}"
}

var "deps_tag" {
  default = "deps-${file_hash("package.json")}"
}

env {
  IMAGE_TAG = var.image_tag
}

shell "release" {
  env {
    DEPS_TAG = var.deps_tag
  }
  command = ["printf", var.image_tag, IMAGE_TAG, var.deps_tag]
}

image "app" {
  image = "example/app"
  tags  = [var.image_tag]
}
`)
	runGitOutput(t, dir, "add", "Bachfile", "package.json")
	runGitOutput(t, dir, "commit", "-m", "initial")
	writeFile(t, dir, "dirty.txt", "dirty\n")
	commit := runGitOutput(t, dir, "rev-parse", "HEAD")
	shortSHA := commit[:12]

	project, err := Load(filepath.Join(dir, "Bachfile"))
	if err != nil {
		t.Fatal(err)
	}

	if got, want := project.Variables["image_tag"], shortSHA+"-dirty"; got != want {
		t.Fatalf("image_tag = %q, want %q", got, want)
	}
	if got := project.Variables["deps_tag"]; len(got) != len("deps-")+12 ||
		got[:len("deps-")] != "deps-" {
		t.Fatalf("deps_tag = %q, want deps- plus 12-char hash", got)
	}
	command := project.Targets["shell/release"].Command
	if command[1] != shortSHA+"-dirty" || command[2] != shortSHA+"-dirty" {
		t.Fatalf("command = %v, want computed image tag in args", command)
	}
	if got := project.Targets["shell/release"].Env[0]; got != "DEPS_TAG="+project.Variables["deps_tag"] {
		t.Fatalf("target env = %v, want deps tag", project.Targets["shell/release"].Env)
	}
	if got, want := project.Targets["image/app"].Tags[0], shortSHA+"-dirty"; got != want {
		t.Fatalf("image tag = %q, want %q", got, want)
	}
}
func TestLoadDecodesTargetEnvBlock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Bachfile")
	contents := `project "example" {}

env {
  BASE = "base"
}

shell "show" {
  env {
    A = "${BASE}-a"
    B = "${A}-b"
  }
  command = ["sh", "-c", "printf $B"]
}
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	project, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"A=base-a", "B=base-a-b"}
	got := project.Targets["shell/show"].Env
	if len(got) != len(want) {
		t.Fatalf("target env = %v, want %v", got, want)
	}
	for index, value := range want {
		if got[index] != value {
			t.Fatalf("target env = %v, want %v", got, want)
		}
	}
	shell, ok := project.Targets["shell/show"].Spec().Body.(ShellSpec)
	if !ok {
		t.Fatalf("body = %T, want ShellSpec", project.Targets["shell/show"].Spec().Body)
	}
	if got := strings.Join(shell.Command, " "); got != "sh -c printf $B" {
		t.Fatalf("operation = %q, want decoded command", got)
	}
}
func TestLoadAppliesSelectedProfilesBetweenProjectAndTargetEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Bachfile")
	contents := `project "example" {}

env {
  NAMESPACE = "base"
  HOST      = "base.example.com"
}

profile "staging" {
  env {
    NAMESPACE = "staging"
    HOST      = "${NAMESPACE}.example.com"
  }
}

profile "kristiyan" {
  env {
    NAMESPACE = "staging-kristiyan"
  }
}

shell "show" {
  env {
    URL = "https://${HOST}/${NAMESPACE}"
  }
  shell = "printf '$URL'"
}
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	project, err := LoadWithOptions(path, LoadOptions{Profiles: []string{"staging", "kristiyan"}})
	if err != nil {
		t.Fatal(err)
	}

	if got, want := project.SelectedProfiles, []string{
		"staging",
		"kristiyan",
	}; len(got) != len(want) || got[0] != want[0] ||
		got[1] != want[1] {
		t.Fatalf("selected profiles = %v, want %v", got, want)
	}
	if got, want := project.ProfileEnv, []string{
		"HOST=staging.example.com",
		"NAMESPACE=staging-kristiyan",
	}; len(got) != len(want) || got[0] != want[0] ||
		got[1] != want[1] {
		t.Fatalf("profile env = %v, want %v", got, want)
	}
	if got, want := project.Targets["shell/show"].Env, []string{
		"URL=https://staging.example.com/staging-kristiyan",
	}; len(got) != len(want) ||
		got[0] != want[0] {
		t.Fatalf("target env = %v, want %v", got, want)
	}
	shell, ok := project.Targets["shell/show"].Spec().Body.(ShellSpec)
	if !ok {
		t.Fatalf("body = %T, want ShellSpec", project.Targets["shell/show"].Spec().Body)
	}
	if got := shell.Shell; got != "printf '$URL'" {
		t.Fatalf("operation = %q, want decoded shell", got)
	}
}
func TestLoadRejectsUnknownSelectedProfile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Bachfile")
	contents := `project "example" {}

shell "show" {
  command = ["true"]
}
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadWithOptions(path, LoadOptions{Profiles: []string{"missing"}}); err == nil {
		t.Fatal("expected unknown profile error")
	}
}
