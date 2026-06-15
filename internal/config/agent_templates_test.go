package config

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/applauselab/bachkator/internal/model"
)

func TestLoadDecodesAgentTemplateAndAppliesToAgent(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "prompts", "random.md"), "random prompt\n")
	writeTestFile(t, filepath.Join(dir, "plans", "random.md"), "random plan\n")
	writeTestFile(t, filepath.Join(dir, "Bachfile"), `project "example" {}

provider "fixture" {
  type    = "agent"
  command = ["true"]
}

prompt "random" {
  path = "prompts/random.md"
}

policy "accept" {}

agent_template "feature" {
  mode     = "implement"
  provider = provider.fixture
  role     = "implementer"
  prompt   = prompt.random
  policy   = policy.accept

  workspace {
    mode = "clone"
    path = ".bach/agents/template-feature"
  }

  git {
    branch = "bach/template-feature"
    commit = "optional"
  }
}

agent "templated" {
  template = agent_template.feature
  plan     = "plans/random.md"
}
`)

	project, err := Load(filepath.Join(dir, "Bachfile"))
	if err != nil {
		t.Fatal(err)
	}
	template := project.AgentTemplates["agent_template/feature"]
	if template == nil {
		t.Fatal("agent template missing")
	}
	if template.ProviderConfig == nil || template.ProviderConfig.Name != "fixture" {
		t.Fatalf("template provider = %#v", template.ProviderConfig)
	}
	agent := project.Targets["agent/templated"]
	if agent == nil {
		t.Fatal("templated agent missing")
	}
	if agent.Template != "agent_template/feature" || agent.Provider != "provider/fixture" ||
		agent.Prompt != "prompt/random" || agent.Policy != "policy/accept" {
		t.Fatalf(
			"agent refs = template:%q provider:%q prompt:%q policy:%q",
			agent.Template,
			agent.Provider,
			agent.Prompt,
			agent.Policy,
		)
	}
	if agent.Role != "implementer" || agent.Workspace[0].Path != ".bach/agents/template-feature" ||
		agent.Git[0].Commit != "optional" {
		t.Fatalf("agent inherited role/workspace/git = %#v", agent)
	}
	spec, ok := agent.Spec().Body.(model.AgentSpec)
	if !ok {
		t.Fatalf("agent spec = %T, want AgentSpec", agent.Spec().Body)
	}
	if spec.Template != "agent_template/feature" || spec.Provider.Name != "fixture" ||
		spec.Prompt.Path != "prompts/random.md" {
		t.Fatalf("agent spec = %#v", spec)
	}
	runtimeProject := RuntimeProject(project)
	if runtimeProject.AgentTemplates["agent_template/feature"].Workspace.Path != ".bach/agents/template-feature" {
		t.Fatalf("runtime template = %#v", runtimeProject.AgentTemplates["agent_template/feature"])
	}
}

func TestLoadAgentTemplateExplicitAgentFieldsOverrideDefaults(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "prompts", "base.md"), "base prompt\n")
	writeTestFile(t, filepath.Join(dir, "prompts", "override.md"), "override prompt\n")
	writeTestFile(t, filepath.Join(dir, "plans", "random.md"), "random plan\n")
	writeTestFile(t, filepath.Join(dir, "Bachfile"), `project "example" {}

provider "fixture" {
  type    = "agent"
  command = ["true"]
}

prompt "base" {
  path = "prompts/base.md"
}

prompt "override" {
  path = "prompts/override.md"
}

agent_template "base" {
  provider = provider.fixture
  role     = "base-role"
  prompt   = prompt.base

  workspace {
    path = ".bach/agents/base"
  }

  git {
    branch = "bach/base"
  }
}

agent "templated" {
  template = agent_template.base
  role     = "override-role"
  prompt   = prompt.override
  plan     = "plans/random.md"

  workspace {
    path = ".bach/agents/override"
  }

  git {
    branch = "bach/override"
  }
}
`)

	project, err := Load(filepath.Join(dir, "Bachfile"))
	if err != nil {
		t.Fatal(err)
	}
	agent := project.Targets["agent/templated"]
	if agent.Role != "override-role" || agent.Prompt != "prompt/override" ||
		agent.Workspace[0].Path != ".bach/agents/override" ||
		agent.Git[0].Branch != "bach/override" {
		t.Fatalf("agent override fields = %#v", agent)
	}
}

func TestLoadAgentTemplateAllowsPlaceholdersOnlyInTemplateWorkspaceAndGit(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "Bachfile"), `project "example" {}

provider "fixture" {
  type    = "agent"
  command = ["true"]
}

agent_template "feature" {
  provider = provider.fixture

  workspace {
    path = ".bach/agents/${work_item.id}/${workstream.id}"
  }

  git {
    branch = "bach/${work_item.slug}-${plan.id}-${factory.name}-${workflow.name}"
  }
}
`)

	project, err := Load(filepath.Join(dir, "Bachfile"))
	if err != nil {
		t.Fatal(err)
	}
	template := project.AgentTemplates["agent_template/feature"]
	if !strings.Contains(template.Workspace[0].Path, "${work_item.id}") ||
		!strings.Contains(template.Git[0].Branch, "${workflow.name}") {
		t.Fatalf(
			"template placeholders = workspace:%q branch:%q",
			template.Workspace[0].Path,
			template.Git[0].Branch,
		)
	}
}

func TestLoadAcceptsReviewAndMergeAgentTemplatesWithoutPolicy(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "Bachfile"), `project "example" {}

provider "fixture" {
  type    = "agent"
  command = ["true"]
}

agent_template "reviewer" {
  mode     = "review"
  provider = provider.fixture
}

agent_template "merger" {
  mode     = "merge"
  provider = provider.fixture
}
`)

	project, err := Load(filepath.Join(dir, "Bachfile"))
	if err != nil {
		t.Fatal(err)
	}
	if project.AgentTemplates["agent_template/reviewer"].Mode != "review" ||
		project.AgentTemplates["agent_template/merger"].Mode != "merge" {
		t.Fatalf("agent templates = %#v", project.AgentTemplates)
	}
}

func TestLoadRejectsInvalidAgentTemplates(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		files   map[string]string
		wantErr string
	}{
		{
			name: "missing provider",
			body: `agent_template "feature" {
  mode = "implement"
}`,
			wantErr: `agent_template "feature" provider is required`,
		},
		{
			name: "invalid mode",
			body: `agent_template "feature" {
  mode     = "audit"
  provider = provider.fixture
}`,
			wantErr: `agent_template "feature" mode must be implement, review, or merge`,
		},
		{
			name: "missing prompt reference",
			body: `agent_template "feature" {
  provider = provider.fixture
  prompt   = "missing"
}`,
			wantErr: `references unknown prompt`,
		},
		{
			name: "missing policy reference",
			body: `agent_template "feature" {
  provider = provider.fixture
  policy   = "missing"
}`,
			wantErr: `references unknown policy`,
		},
		{
			name: "policy on review template",
			body: `policy "accept" {}

agent_template "reviewer" {
  mode     = "review"
  provider = provider.fixture
  policy   = policy.accept
}`,
			wantErr: `agent_template "reviewer" policy is supported only for implement mode`,
		},
		{
			name: "placeholder in role",
			body: `agent_template "feature" {
  provider = provider.fixture
  role     = "${work_item.id}"
}`,
			wantErr: `agent_template "feature" role must not contain placeholders`,
		},
		{
			name: "unsupported placeholder",
			body: `agent_template "feature" {
  provider = provider.fixture

  workspace {
    path = ".bach/agents/$${unknown.placeholder}"
  }
}`,
			wantErr: `contains unsupported placeholder`,
		},
		{
			name: "unknown field",
			body: `agent_template "feature" {
  provider = provider.fixture
  plan     = "plans/random.md"
}`,
			wantErr: `Unsupported argument`,
		},
		{
			name: "unknown agent template reference",
			body: `agent "templated" {
  template = "missing"
  plan     = "plans/random.md"
}`,
			files:   map[string]string{"plans/random.md": "random plan\n"},
			wantErr: `references unknown agent_template`,
		},
		{
			name: "runnable agent inherits unresolved placeholder",
			body: `agent_template "feature" {
  provider = provider.fixture

  workspace {
    path = ".bach/agents/${work_item.id}"
  }
}

agent "templated" {
  template = agent_template.feature
  plan     = "plans/random.md"
}`,
			files:   map[string]string{"plans/random.md": "random plan\n"},
			wantErr: `workspace.path must not contain agent template placeholders`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, contents := range tt.files {
				writeTestFile(t, filepath.Join(dir, name), contents)
			}
			writeTestFile(t, filepath.Join(dir, "Bachfile"), `project "example" {}

provider "fixture" {
  type    = "agent"
  command = ["true"]
}

`+tt.body)

			_, err := Load(filepath.Join(dir, "Bachfile"))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Load() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}
