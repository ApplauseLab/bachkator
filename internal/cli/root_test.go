package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestRootHelpShowsSubcommands(t *testing.T) {
	var stdout bytes.Buffer
	if err := Execute(
		context.Background(),
		[]string{"--help"},
		&stdout,
		&bytes.Buffer{},
		"test",
	); err != nil {
		t.Fatal(err)
	}

	got := stdout.String()
	for _, want := range []string{"Available Commands:", "run", "list", "runs", "reference"} {
		if !strings.Contains(got, want) {
			t.Fatalf("help missing %q:\n%s", want, got)
		}
	}
}

func TestFlagStyleCommandsAreRemoved(t *testing.T) {
	for _, args := range [][]string{{"-list"}, {"-runs"}} {
		if err := Execute(
			context.Background(),
			args,
			&bytes.Buffer{},
			&bytes.Buffer{},
			"test",
		); err == nil {
			t.Fatalf("Execute(%#v) succeeded, want removed flag command error", args)
		}
	}
}

func TestDirectTargetExecutionIsRemoved(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Bachfile"), `project "example" {}

shell "test" {
  command = ["true"]
}
`)

	args := []string{"-f", filepath.Join(dir, "Bachfile"), "shell/test"}
	if err := Execute(
		context.Background(),
		args,
		&bytes.Buffer{},
		&bytes.Buffer{},
		"test",
	); err == nil {
		t.Fatal("direct target execution succeeded, want unknown command error")
	}
}
