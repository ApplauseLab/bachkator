package graph

import (
	"testing"

	"github.com/applauselab/bachkator/internal/model"
)

func pathProvenanceEqual(got, want []PathProvenance) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i].Path != want[i].Path || got[i].Status != want[i].Status {
			return false
		}
	}
	return true
}

func TestProvenanceMapsOutputToProducer(t *testing.T) {
	project := provenanceProject(map[string]*model.RunTarget{
		"shell/docs-generate": provenanceTarget(
			"shell/docs-generate",
			[]string{"docs/reference"},
			[]string{"docs/reference.md"},
		),
	})

	got := Provenance(project, []string{"docs/reference.md"})
	if len(got) != 1 {
		t.Fatalf("records = %#v, want one record", got)
	}
	if !got[0].Generated || got[0].Source || len(got[0].Producers) != 1 {
		t.Fatalf("record = %#v, want generated producer", got[0])
	}
	if got[0].Producers[0].Target != "shell/docs-generate" {
		t.Fatalf("producer = %q", got[0].Producers[0].Target)
	}
	if got[0].Producers[0].RegenerateCommand != "bach run shell/docs-generate" {
		t.Fatalf("regenerate command = %q", got[0].Producers[0].RegenerateCommand)
	}
}

func TestProvenanceWithHandlersUsesDescribeOperation(t *testing.T) {
	project := provenanceProject(map[string]*model.RunTarget{
		"shell/docs-generate": provenanceTarget(
			"shell/docs-generate",
			nil,
			[]string{"docs/reference.md"},
		),
	})

	got := ProvenanceWithHandlers(
		project,
		[]string{"docs/reference.md"},
		fakeTargetHandlers{handler: fakeTargetHandler{operation: "custom operation"}},
	)
	if len(got) != 1 || len(got[0].Producers) != 1 {
		t.Fatalf("records = %#v, want one producer", got)
	}
	if got[0].Producers[0].Operation != "custom operation" {
		t.Fatalf("operation = %q", got[0].Producers[0].Operation)
	}
}

func TestProvenanceMapsInputToConsumer(t *testing.T) {
	project := provenanceProject(map[string]*model.RunTarget{
		"shell/build": provenanceTarget("shell/build", []string{"internal/runner"}, nil),
	})

	got := Provenance(project, []string{"internal/runner/plan.go"})
	if len(got) != 1 || !got[0].Source || got[0].Generated {
		t.Fatalf("records = %#v, want source consumer", got)
	}
	if got[0].Consumers[0].Target != "shell/build" {
		t.Fatalf("consumer = %q", got[0].Consumers[0].Target)
	}
}

func TestProvenanceShowsProducerAndConsumers(t *testing.T) {
	project := provenanceProject(map[string]*model.RunTarget{
		"shell/build": provenanceTarget(
			"shell/build",
			[]string{"src/app.txt"},
			[]string{"dist/app.txt"},
		),
		"shell/test": provenanceTarget("shell/test", []string{"dist"}, nil),
	})

	got := Provenance(project, []string{"dist/app.txt"})
	if len(got) != 1 || !got[0].Generated || got[0].Source {
		t.Fatalf("records = %#v, want generated non-source", got)
	}
	if len(got[0].Producers) != 1 || len(got[0].Consumers) != 1 {
		t.Fatalf("record = %#v, want producer and consumer", got[0])
	}
}

func TestProvenanceMatchesDirectoriesAndMissingOutputs(t *testing.T) {
	project := provenanceProject(map[string]*model.RunTarget{
		"shell/generate": provenanceTarget("shell/generate", nil, []string{"dist"}),
	})

	got := Provenance(project, []string{"dist/missing.txt"})
	if len(got) != 1 || !got[0].Generated {
		t.Fatalf("records = %#v, want generated directory match", got)
	}
}

func TestProvenanceUnknownPathSucceeds(t *testing.T) {
	project := provenanceProject(map[string]*model.RunTarget{
		"shell/build": provenanceTarget("shell/build", []string{"src"}, []string{"dist"}),
	})

	got := Provenance(project, []string{"README.md"})
	want := []PathProvenance{{Path: "README.md", Status: "unknown"}}
	if !pathProvenanceEqual(got, want) {
		t.Fatalf("records = %#v, want %#v", got, want)
	}
}

func TestProvenanceOutsideAbsolutePathDoesNotMatchProjectRootInput(t *testing.T) {
	project := provenanceProject(map[string]*model.RunTarget{
		"shell/test": provenanceTarget("shell/test", []string{"."}, nil),
	})
	project.Root = "/workspace/project"

	got := Provenance(project, []string{"/tmp/outside.txt"})
	if len(got) != 1 || got[0].Source || len(got[0].Consumers) != 0 {
		t.Fatalf("records = %#v, want outside path with no consumers", got)
	}
}

func provenanceProject(targets map[string]*model.RunTarget) *model.RunProject {
	return &model.RunProject{
		Root:      ".",
		Inputs:    map[string]*model.Input{},
		Resources: map[string]*model.Resource{},
		Targets:   targets,
	}
}

func provenanceTarget(name string, inputs []string, outputs []string) *model.RunTarget {
	return &model.RunTarget{
		Name: name,
		SpecValue: model.TargetSpec{
			Name: name,
			Cache: model.TargetCache{
				Inputs:  inputs,
				Outputs: outputs,
			},
			Body: model.ShellSpec{Command: []string{"true"}},
		},
	}
}
