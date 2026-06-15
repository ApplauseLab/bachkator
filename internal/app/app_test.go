package app

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestAppExecutesCLIWithProductionWiring(t *testing.T) {
	var stdout bytes.Buffer
	application := New("test")

	if err := application.Execute(
		context.Background(),
		[]string{"--version"},
		&stdout,
		&bytes.Buffer{},
	); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if got := strings.TrimSpace(stdout.String()); got != "bach test" {
		t.Fatalf("version output = %q, want %q", got, "bach test")
	}
}
