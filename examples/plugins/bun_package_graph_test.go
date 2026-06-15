package plugins

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/applauselab/bachkator/internal/config"
	"github.com/applauselab/bachkator/internal/graph"
)

type pluginOutput struct {
	Inputs map[string][]string `json:"inputs"`
}

func TestBunPackageGraphEmitsDependencyClosure(t *testing.T) {
	out := runBunPackageGraph(t, map[string][]string{
		"api": {"packages/api"},
	})

	want := []string{"packages/api", "packages/core", "packages/test-utils"}
	if !slices.Equal(out.Inputs["api"], want) {
		t.Fatalf("api inputs = %#v, want %#v", out.Inputs["api"], want)
	}
}

func TestBunPackageGraphResolvesPackageExportsAsSources(t *testing.T) {
	out := runBunPackageGraph(t, map[string][]string{
		"core_feature": {"@app/core/feature"},
	})

	want := []string{"packages/core"}
	if !slices.Equal(out.Inputs["core_feature"], want) {
		t.Fatalf("core_feature inputs = %#v, want %#v", out.Inputs["core_feature"], want)
	}
}

func TestBunPackageGraphPluginInputsDriveAffectedTargets(t *testing.T) {
	fixture := fixtureRoot(t)
	path := filepath.Join(t.TempDir(), "Bachfile")
	contents := `project "example" {
  root = "` + fixture + `"
}

plugin "bun_packages" {
  command = ["bun", "` + pluginPath(t) + `"]
  sources = {
    web = ["packages/web"]
  }
}

shell "test-web" {
  command = ["true"]
  inputs  = [plugin.bun_packages.web]
}
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	project, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}

	got := graph.AffectedTargets(
		config.RuntimeProject(project),
		[]string{"packages/api/src/index.ts"},
	)
	want := []graph.AffectedTarget{{Name: "shell/test-web", Matches: []string{"packages/api"}}}
	if len(got) != len(want) || got[0].Name != want[0].Name ||
		!slices.Equal(got[0].Matches, want[0].Matches) {
		t.Fatalf("affected targets = %#v, want %#v", got, want)
	}
}

func runBunPackageGraph(t *testing.T, sources map[string][]string) pluginOutput {
	t.Helper()
	sourcesJSON, err := json.Marshal(sources)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bun", pluginPath(t))
	cmd.Env = append(os.Environ(),
		"BACH_PROJECT_ROOT="+fixtureRoot(t),
		"BACH_PLUGIN_SOURCES="+string(sourcesJSON),
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bun package graph failed: %v\n%s", err, output)
	}
	var out pluginOutput
	if err := json.Unmarshal(output, &out); err != nil {
		t.Fatalf("invalid plugin output: %v\n%s", err, output)
	}
	return out
}

func fixtureRoot(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "plugins", "fixtures", "bun-workspace")
}

func pluginPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "plugins", "bun-package-graph.ts")
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(filepath.Join(dir, "..", ".."))
}
