package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDecodesFactoryApprovalProvider(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "Bachfile"), `project "example" {}

factory "sldc" {
  workflow "ship" {}

  approvals {
    provider "github" {
      command       = ["bach-approval-fixture"]
      poll_interval = "30s"

      config = {
        repo         = "owner/repo"
        label_prefix = "approved/"
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
	providers := factory.ApprovalProviders()
	if len(providers) != 1 {
		t.Fatalf("approval providers = %d, want 1", len(providers))
	}
	p := providers[0]
	if p.Name != "github" {
		t.Fatalf("provider name = %q, want github", p.Name)
	}
	if len(p.Command) != 1 || p.Command[0] != "bach-approval-fixture" {
		t.Fatalf("provider command = %v, want [bach-approval-fixture]", p.Command)
	}
	if d := p.PollIntervalDuration(); d.String() != "30s" {
		t.Fatalf("poll interval = %v, want 30s", d)
	}
	if p.Config["label_prefix"] != "approved/" {
		t.Fatalf("config = %v, want label_prefix approved/", p.Config)
	}
}

func TestLoadRejectsInvalidApprovalProviders(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{
			name: "missing provider command",
			body: `factory "sldc" {
  workflow "ship" {}
  approvals {
    provider "github" {}
  }
}`,
			wantErr: `approval provider "github" command is required`,
		},
		{
			name: "invalid poll interval",
			body: `factory "sldc" {
  workflow "ship" {}
  approvals {
    provider "github" {
      command       = ["x"]
      poll_interval = "not-a-duration"
    }
  }
}`,
			wantErr: `poll_interval is not a valid duration`,
		},
		{
			name: "duplicate approval provider",
			body: `factory "sldc" {
  workflow "ship" {}
  approvals {
    provider "github" {
      command = ["x"]
    }
    provider "github" {
      command = ["x"]
    }
  }
}`,
			wantErr: `duplicate approval provider "github"`,
		},
		{
			name: "duplicate approvals blocks",
			body: `factory "sldc" {
  workflow "ship" {}
  approvals {}
  approvals {}
}`,
			wantErr: `at most one approvals block`,
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
