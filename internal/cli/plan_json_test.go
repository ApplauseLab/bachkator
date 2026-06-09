package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDryRunJSONPrintsMachineReadablePlan(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "input.txt"), "ok\n")
	writeFile(t, filepath.Join(dir, "Bachfile"), `project "example" {}

profile "staging" {
  env {
    HOST = "staging.example.com"
  }
}

shell "prepare" {
  shell = "printf prepare"
}

shell "deploy" {
  depends_on = [shell.prepare]
  shell = "printf deploy-${HOST}"
  inputs = ["input.txt"]
  outputs = ["missing.out"]
  lock = "deploy"
  remote = true
  destructive = true
  requires_confirmation = true
}
`)

	var stdout bytes.Buffer
	args := []string{
		"-f",
		filepath.Join(dir, "Bachfile"),
		"-profile",
		"staging",
		"-dry-run",
		"-json",
		"run",
		"shell/deploy",
	}
	if err := Execute(context.Background(), args, &stdout, &bytes.Buffer{}, "test"); err != nil {
		t.Fatal(err)
	}

	var plan struct {
		Target           string   `json:"target"`
		SelectedProfiles []string `json:"selected_profiles"`
		TargetOrder      []string `json:"target_order"`
		DependencyEdges  []struct {
			From string `json:"from"`
			To   string `json:"to"`
		} `json:"dependency_edges"`
		EffectiveRisk struct {
			Remote               bool     `json:"remote"`
			Destructive          bool     `json:"destructive"`
			RequiresConfirmation bool     `json:"requires_confirmation"`
			Labels               []string `json:"labels"`
		} `json:"effective_risk"`
		Targets []struct {
			Name      string `json:"name"`
			Operation string `json:"operation"`
			Lock      string `json:"lock"`
			Cache     struct {
				Cacheable   bool   `json:"cacheable"`
				Expectation string `json:"expectation"`
				Fresh       bool   `json:"fresh"`
				Fingerprint string `json:"fingerprint"`
			} `json:"cache"`
		} `json:"targets"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &plan); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if plan.Target != "shell/deploy" {
		t.Fatalf("target = %q, want shell/deploy", plan.Target)
	}
	if len(plan.SelectedProfiles) != 1 || plan.SelectedProfiles[0] != "staging" {
		t.Fatalf("selected profiles = %#v, want staging", plan.SelectedProfiles)
	}
	if len(plan.TargetOrder) != 2 || plan.TargetOrder[0] != "shell/prepare" ||
		plan.TargetOrder[1] != "shell/deploy" {
		t.Fatalf("target order = %#v", plan.TargetOrder)
	}
	if len(plan.DependencyEdges) != 1 || plan.DependencyEdges[0].From != "shell/prepare" ||
		plan.DependencyEdges[0].To != "shell/deploy" {
		t.Fatalf("dependency edges = %#v", plan.DependencyEdges)
	}
	if !plan.EffectiveRisk.Remote || !plan.EffectiveRisk.Destructive ||
		!plan.EffectiveRisk.RequiresConfirmation {
		t.Fatalf("effective risk = %#v", plan.EffectiveRisk)
	}
	deploy := plan.Targets[1]
	if deploy.Operation != "printf deploy-staging.example.com" {
		t.Fatalf("deploy operation = %q", deploy.Operation)
	}
	if deploy.Lock != "deploy" {
		t.Fatalf("deploy lock = %q", deploy.Lock)
	}
	if !deploy.Cache.Cacheable || deploy.Cache.Expectation != "run" || deploy.Cache.Fresh ||
		deploy.Cache.Fingerprint == "" {
		t.Fatalf("deploy cache = %#v", deploy.Cache)
	}
}

func TestJSONRequiresDryRun(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Bachfile"), `project "example" {}

shell "test" {
  command = ["true"]
}
`)

	args := []string{"-f", filepath.Join(dir, "Bachfile"), "-json", "run", "shell/test"}
	if err := Execute(
		context.Background(),
		args,
		&bytes.Buffer{},
		&bytes.Buffer{},
		"test",
	); err == nil {
		t.Fatal("expected -json without -dry-run to fail")
	}
}

func TestDryRunJSONExplainsStaleCacheReason(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "input.txt"), "one\n")
	writeFile(t, filepath.Join(dir, "Bachfile"), `project "example" {}

shell "build" {
  shell = "mkdir -p out && cp input.txt out/app.txt"
  inputs = ["input.txt"]
  outputs = ["out/app.txt"]
}
`)

	args := []string{"-f", filepath.Join(dir, "Bachfile"), "run", "shell/build"}
	if err := Execute(
		context.Background(),
		args,
		&bytes.Buffer{},
		&bytes.Buffer{},
		"test",
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "input.txt"), []byte("two\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	args = []string{"-f", filepath.Join(dir, "Bachfile"), "-dry-run", "-json", "run", "shell/build"}
	if err := Execute(context.Background(), args, &stdout, &bytes.Buffer{}, "test"); err != nil {
		t.Fatal(err)
	}

	var plan struct {
		Targets []struct {
			Cache struct {
				Expectation string   `json:"expectation"`
				Fresh       bool     `json:"fresh"`
				Reasons     []string `json:"reasons"`
			} `json:"cache"`
		} `json:"targets"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &plan); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if len(plan.Targets) != 1 {
		t.Fatalf("target count = %d, want 1", len(plan.Targets))
	}
	cache := plan.Targets[0].Cache
	if cache.Expectation != "run" || cache.Fresh {
		t.Fatalf("cache = %#v, want stale run", cache)
	}
	if !contains(cache.Reasons, "changed input") {
		t.Fatalf("cache reasons = %#v, want changed input", cache.Reasons)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
