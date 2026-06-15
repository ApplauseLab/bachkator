package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDecodesFactoryProviderTrigger(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "Bachfile"), `project "example" {}

factory "sldc" {
  workflow "ship" {}

  triggers {
    manual {}

    provider "github_issues" {
      command = ["bach-trigger-fixture"]

      config = {
        repo = "owner/repo"
      }

      route {
        label    = "factory:ship"
        workflow = "ship"
      }
    }
  }
}
`)

	project, err := Load(filepath.Join(dir, "Bachfile"))
	if err != nil {
		t.Fatal(err)
	}
	factory := project.Factories["sldc"]
	if factory == nil {
		t.Fatal("factory missing")
	}
	providers := factory.ProviderTriggers()
	if len(providers) != 1 {
		t.Fatalf("providers = %d, want 1", len(providers))
	}
	p := providers[0]
	if p.Name != "github_issues" {
		t.Fatalf("provider name = %q, want github_issues", p.Name)
	}
	if len(p.Command) != 1 || p.Command[0] != "bach-trigger-fixture" {
		t.Fatalf("provider command = %v, want [bach-trigger-fixture]", p.Command)
	}
	if d := p.PollIntervalDuration(); d != defaultPollInterval {
		t.Fatalf("poll interval = %v, want %v", d, defaultPollInterval)
	}
	workflow, err := p.RouteWorkflow([]string{"factory:ship"}, "ship")
	if err != nil {
		t.Fatal(err)
	}
	if workflow != "ship" {
		t.Fatalf("workflow = %q, want ship", workflow)
	}
}

func TestLoadRejectsInvalidProviderTriggers(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{
			name: "missing provider command",
			body: `factory "sldc" {
  workflow "ship" {}
  triggers {
    provider "github" {}
  }
}`,
			wantErr: `provider trigger "github" command is required`,
		},
		{
			name: "invalid poll interval",
			body: `factory "sldc" {
  workflow "ship" {}
  triggers {
    provider "github" {
      command       = ["x"]
      poll_interval = "not-a-duration"
    }
  }
}`,
			wantErr: `poll_interval is not a valid duration`,
		},
		{
			name: "multi-workflow route required",
			body: `factory "sldc" {
  workflow "ship" {}
  workflow "hotfix" {}
  triggers {
    provider "github" {
      command = ["x"]
    }
  }
}`,
			wantErr: `requires route rules`,
		},
		{
			name: "unknown route workflow",
			body: `factory "sldc" {
  workflow "ship" {}
  triggers {
    provider "github" {
      command = ["x"]
      route {
        label    = "x"
        workflow = "missing"
      }
    }
  }
}`,
			wantErr: `route.workflow "missing" is not a declared workflow`,
		},
		{
			name: "duplicate provider trigger",
			body: `factory "sldc" {
  workflow "ship" {}
  triggers {
    provider "github" {
      command = ["x"]
    }
    provider "github" {
      command = ["x"]
    }
  }
}`,
			wantErr: `duplicate provider trigger "github"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeTestFile(t, filepath.Join(dir, "Bachfile"), "project \"example\" {}\n\n"+tt.body)
			_, err := Load(filepath.Join(dir, "Bachfile"))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Load() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestLoadDecodesFactoryManualQueue(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "Bachfile"), `project "example" {}

factory "sldc" {
  workflow "ship" {}

  triggers {
    manual {}
  }
}
`)

	project, err := Load(filepath.Join(dir, "Bachfile"))
	if err != nil {
		t.Fatal(err)
	}
	factory := project.Factories["sldc"]
	if factory == nil {
		t.Fatal("factory missing")
	}
	if !factory.ManualEnabled() {
		t.Fatal("manual trigger disabled, want enabled")
	}
	workflow, err := factory.ResolveWorkflow("")
	if err != nil {
		t.Fatal(err)
	}
	if workflow != "ship" {
		t.Fatalf("workflow = %q, want ship", workflow)
	}
	if len(project.Targets) != 0 {
		t.Fatalf("factory created targets: %#v", project.Targets)
	}
}

func TestLoadFactoryWorkflowRouting(t *testing.T) {
	factory := &Factory{
		Name:      "sldc",
		Workflows: []*FactoryWorkflow{{Name: "ship"}, {Name: "hotfix"}},
	}
	if _, err := factory.ResolveWorkflow(""); err == nil ||
		!strings.Contains(err.Error(), "--workflow is required") {
		t.Fatalf("ResolveWorkflow() error = %v, want required workflow", err)
	}
	workflow, err := factory.ResolveWorkflow("hotfix")
	if err != nil {
		t.Fatal(err)
	}
	if workflow != "hotfix" {
		t.Fatalf("workflow = %q, want hotfix", workflow)
	}
	_, err = factory.ResolveWorkflow("missing")
	if err == nil || !strings.Contains(err.Error(), `workflow "missing"`) {
		t.Fatalf("ResolveWorkflow() error = %v, want unknown workflow", err)
	}
}

func TestLoadRejectsInvalidFactories(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{
			name:    "missing workflows",
			body:    `factory "sldc" {}`,
			wantErr: `factory "sldc" must declare at least one workflow`,
		},
		{
			name: "duplicate factory",
			body: `factory "sldc" {
  workflow "ship" {}
}

factory "sldc" {
  workflow "ship" {}
}`,
			wantErr: `duplicate factory "sldc"`,
		},
		{
			name: "duplicate workflow",
			body: `factory "sldc" {
  workflow "ship" {}
  workflow "ship" {}
}`,
			wantErr: `duplicate workflow "ship"`,
		},
		{
			name: "duplicate triggers",
			body: `factory "sldc" {
  workflow "ship" {}
  triggers {}
  triggers {}
}`,
			wantErr: `at most one triggers block`,
		},
		{
			name: "duplicate manual",
			body: `factory "sldc" {
  workflow "ship" {}
  triggers {
    manual {}
    manual {}
  }
}`,
			wantErr: `at most one manual trigger`,
		},
		{
			name: "unknown field",
			body: `factory "sldc" {
  workflow "ship" {}
  repo = "."
}`,
			wantErr: `Unsupported argument`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeTestFile(t, filepath.Join(dir, "Bachfile"), "project \"example\" {}\n\n"+tt.body)
			_, err := Load(filepath.Join(dir, "Bachfile"))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Load() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}
