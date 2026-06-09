package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestGraphJSONIncludesDependenciesPipelinesLocksProfilesAndRisks(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Bachfile"), `project "example" {}

profile "staging" {
  env {
    NAMESPACE = "staging"
  }
}

shell "build" {
  command = ["true"]
}

shell "migrate" {
  depends_on            = [shell.build]
  lock                  = "postgres"
  remote                = true
  destructive           = true
  requires_confirmation = true
  command               = ["true"]
}

shell "smoke" {
  depends_on = [shell.migrate]
  command    = ["true"]
}

pipeline "deploy" {
  steps = [shell.migrate, shell.smoke]
}
`)

	var stdout bytes.Buffer
	args := []string{
		"-f",
		filepath.Join(dir, "Bachfile"),
		"-profile",
		"staging",
		"graph",
		"--format",
		"json",
	}
	if err := Execute(context.Background(), args, &stdout, &bytes.Buffer{}, "test"); err != nil {
		t.Fatal(err)
	}

	var doc graphDocument
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("invalid json graph: %v\n%s", err, stdout.String())
	}
	if len(doc.Profiles) != 1 || doc.Profiles[0] != "staging" {
		t.Fatalf("profiles = %#v, want selected staging profile", doc.Profiles)
	}
	migrate := graphNodeByName(doc.Nodes, "shell/migrate")
	if migrate == nil {
		t.Fatal("missing shell/migrate node")
	}
	if migrate.Lock != "postgres" || !migrate.Remote || !migrate.Destructive ||
		!migrate.RequiresConfirmation {
		t.Fatalf("migrate node = %#v, want lock and risk flags", migrate)
	}
	deploy := graphNodeByName(doc.Nodes, "pipeline/deploy")
	if deploy == nil || !deploy.Remote || !deploy.Destructive || !deploy.RequiresConfirmation {
		t.Fatalf("deploy node = %#v, want inherited pipeline risk flags", deploy)
	}
	for _, want := range []graphEdge{
		{From: "shell/migrate", To: "shell/build", Type: "depends_on"},
		{From: "shell/smoke", To: "shell/migrate", Type: "depends_on"},
		{From: "pipeline/deploy", To: "shell/migrate", Type: "pipeline_step", Order: 1},
		{From: "pipeline/deploy", To: "shell/smoke", Type: "pipeline_step", Order: 2},
	} {
		if !hasGraphEdge(doc.Edges, want) {
			t.Fatalf("missing edge %#v in %#v", want, doc.Edges)
		}
	}
}

func TestGraphMermaidPrintsVisualGraph(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Bachfile"), `project "example" {}

shell "build" {
  command = ["true"]
}

shell "deploy" {
  depends_on = [shell.build]
  lock       = "container-builder"
  remote     = true
  command    = ["true"]
}
`)

	var stdout bytes.Buffer
	args := []string{"-f", filepath.Join(dir, "Bachfile"), "graph"}
	if err := Execute(context.Background(), args, &stdout, &bytes.Buffer{}, "test"); err != nil {
		t.Fatal(err)
	}

	got := stdout.String()
	for _, want := range []string{"flowchart TD", "shell/deploy", "lock=container-builder", "risks=remote", "depends_on"} {
		if !strings.Contains(got, want) {
			t.Fatalf("stdout missing %q:\n%s", want, got)
		}
	}
}

func graphNodeByName(nodes []graphNode, name string) *graphNode {
	for index := range nodes {
		if nodes[index].Name == name {
			return &nodes[index]
		}
	}
	return nil
}

func hasGraphEdge(edges []graphEdge, want graphEdge) bool {
	for _, edge := range edges {
		if edge == want {
			return true
		}
	}
	return false
}
