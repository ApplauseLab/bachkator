package graph

import (
	"slices"
	"testing"

	"github.com/applauselab/bachkator/internal/model"
)

func affectedTargetsEqual(got, want []AffectedTarget) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i].Name != want[i].Name || !slices.Equal(got[i].Matches, want[i].Matches) {
			return false
		}
	}
	return true
}

func TestAffectedTargetsMatchResolvedInputs(t *testing.T) {
	project := &model.RunProject{
		Inputs: map[string]*model.Input{
			"file/api": {Kind: "file", Name: "api", Src: "packages/api/src"},
		},
		Resources: map[string]*model.Resource{},
		Targets: map[string]*model.RunTarget{
			"shell/test-api": runTarget("shell/test-api", []string{"file/api", "package.json"}),
			"shell/lint":     runTarget("shell/lint", []string{"cmd/bach"}),
		},
	}

	got := AffectedTargets(project, []string{"packages/api/src/server.go", "README.md"})
	want := []AffectedTarget{{Name: "shell/test-api", Matches: []string{"packages/api/src"}}}
	if !affectedTargetsEqual(got, want) {
		t.Fatalf("affected targets = %#v, want %#v", got, want)
	}
}

func TestAffectedTargetsMatchPluginInputs(t *testing.T) {
	project := &model.RunProject{
		Inputs: map[string]*model.Input{
			"plugin/ts_imports/api_tests": {
				Kind: "plugin/ts_imports",
				Name: "api_tests",
				Src:  "packages/api/src/main.ts",
			},
		},
		Resources: map[string]*model.Resource{},
		Targets: map[string]*model.RunTarget{
			"shell/test-api": runTarget("shell/test-api", []string{"plugin/ts_imports/api_tests"}),
		},
	}

	got := AffectedTargets(project, []string{"packages/api/src/main.ts"})
	want := []AffectedTarget{
		{Name: "shell/test-api", Matches: []string{"packages/api/src/main.ts"}},
	}
	if !affectedTargetsEqual(got, want) {
		t.Fatalf("affected targets = %#v, want %#v", got, want)
	}
}

func TestAffectedTargetsMatchProjectRootInput(t *testing.T) {
	project := &model.RunProject{
		Inputs:    map[string]*model.Input{},
		Resources: map[string]*model.Resource{},
		Targets: map[string]*model.RunTarget{
			"shell/test": runTarget("shell/test", []string{"."}),
		},
	}

	got := AffectedTargets(project, []string{"internal/build/affected.go"})
	want := []AffectedTarget{{Name: "shell/test", Matches: []string{"."}}}
	if !affectedTargetsEqual(got, want) {
		t.Fatalf("affected targets = %#v, want %#v", got, want)
	}
}

func runTarget(name string, inputs []string) *model.RunTarget {
	return &model.RunTarget{
		Name: name,
		SpecValue: model.TargetSpec{
			Name: name,
			Cache: model.TargetCache{
				Inputs: inputs,
			},
			Body: model.ShellSpec{Command: []string{"true"}},
		},
	}
}
