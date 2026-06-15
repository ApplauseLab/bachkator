package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestProfileFlagMayRepeatAndAffectsDryRun(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Bachfile"), `project "example" {}

env {
  NAMESPACE = "base"
}

profile "staging" {
  env {
    NAMESPACE = "staging"
    HOST      = "staging.example.com"
  }
}

profile "personal" {
  env {
    NAMESPACE = "staging-kristiyan"
  }
}

shell "render" {
  shell = "printf '%s' '${HOST}/${NAMESPACE}'"
}
`)

	var stdout bytes.Buffer
	args := []string{
		"-f",
		filepath.Join(dir, "Bachfile"),
		"--profile",
		"staging",
		"--profile",
		"personal",
		"run",
		"--dry-run",
		"shell/render",
	}
	if err := Execute(context.Background(), args, &stdout, &bytes.Buffer{}, "test"); err != nil {
		t.Fatal(err)
	}

	if got := stdout.String(); !strings.Contains(got, "staging.example.com/staging-kristiyan") {
		t.Fatalf("stdout = %q, want profile-expanded command", got)
	}
}

func TestProfileFlagRejectsUnknownProfile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Bachfile"), `project "example" {}

shell "render" {
  command = ["true"]
}
`)

	args := []string{
		"-f",
		filepath.Join(dir, "Bachfile"),
		"--profile",
		"missing",
		"run",
		"--dry-run",
		"shell/render",
	}
	if err := Execute(
		context.Background(),
		args,
		&bytes.Buffer{},
		&bytes.Buffer{},
		"test",
	); err == nil {
		t.Fatal("expected unknown profile error")
	}
}
