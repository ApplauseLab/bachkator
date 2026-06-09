package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestArtifactsCommandListsIndexedArtifacts(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Bachfile"), `project "example" {}

shell "render" {
  shell   = "mkdir -p dist && printf manifest > \"$BACH_RUN_DIRECTORY/deploy.yaml\" && printf out > dist/app.txt"
  outputs = ["dist/app.txt"]
}
`)

	var runOut bytes.Buffer
	args := []string{"-f", filepath.Join(dir, "Bachfile"), "run", "shell/render"}
	if err := Execute(context.Background(), args, &runOut, &bytes.Buffer{}, "test"); err != nil {
		t.Fatal(err)
	}

	var artifactsOut bytes.Buffer
	args = []string{"-f", filepath.Join(dir, "Bachfile"), "artifacts", "--target", "shell/render"}
	if err := Execute(
		context.Background(),
		args,
		&artifactsOut,
		&bytes.Buffer{},
		"test",
	); err != nil {
		t.Fatal(err)
	}
	got := artifactsOut.String()
	for _, want := range []string{"shell/render", "manifest", "deploy.yaml", "artifact", "dist/app.txt", "log"} {
		if !strings.Contains(got, want) {
			t.Fatalf("artifacts output missing %q: %s", want, got)
		}
	}

	var runsOut bytes.Buffer
	args = []string{"-f", filepath.Join(dir, "Bachfile"), "runs", "--artifact", "dist/app.txt"}
	if err := Execute(context.Background(), args, &runsOut, &bytes.Buffer{}, "test"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(runsOut.String(), "shell/render") ||
		!strings.Contains(runsOut.String(), "success") {
		t.Fatalf("runs output did not include artifact-matched run: %s", runsOut.String())
	}
}
