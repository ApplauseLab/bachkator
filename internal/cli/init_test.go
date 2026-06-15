package cli

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
)

func TestInitCommandPassesOptions(t *testing.T) {
	var got InitOptions
	deps := testDependencies()
	deps.InitProject = func(
		ctx context.Context,
		opts InitOptions,
		stdout io.Writer,
		stderr io.Writer,
	) error {
		got = opts
		return nil
	}

	if err := ExecuteWithDependencies(
		context.Background(),
		[]string{"init", "--file", "custom.bach", "--dry-run", "--provider", "opencode"},
		&bytes.Buffer{},
		&bytes.Buffer{},
		"test",
		deps,
	); err != nil {
		t.Fatal(err)
	}

	if got.ConfigPath != "custom.bach" || got.Provider != "opencode" || !got.DryRun {
		t.Fatalf("InitOptions = %#v", got)
	}
}

func TestInitCommandRejectsPositionalArguments(t *testing.T) {
	deps := testDependencies()
	called := false
	deps.InitProject = func(
		ctx context.Context,
		opts InitOptions,
		stdout io.Writer,
		stderr io.Writer,
	) error {
		called = true
		return nil
	}

	err := ExecuteWithDependencies(
		context.Background(),
		[]string{"init", "extra"},
		&bytes.Buffer{},
		&bytes.Buffer{},
		"test",
		deps,
	)
	if err == nil || !strings.Contains(err.Error(), "init does not accept positional arguments") {
		t.Fatalf("Execute error = %v, want positional argument error", err)
	}
	if called {
		t.Fatal("InitProject was called for invalid args")
	}
}

func TestProviderFlagAcceptsValue(t *testing.T) {
	var got InitOptions
	deps := testDependencies()
	deps.InitProject = func(
		ctx context.Context,
		opts InitOptions,
		stdout io.Writer,
		stderr io.Writer,
	) error {
		got = opts
		return nil
	}

	if err := ExecuteWithDependencies(
		context.Background(),
		[]string{"init", "--provider", "opencode"},
		&bytes.Buffer{},
		&bytes.Buffer{},
		"test",
		deps,
	); err != nil {
		t.Fatal(err)
	}
	if got.Provider != "opencode" {
		t.Fatalf("provider = %q, want opencode", got.Provider)
	}
}
