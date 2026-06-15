package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestRunProvenanceHumanIncludesRegenerateCommand(t *testing.T) {
	deps := Dependencies{Provenance: func(*Project, []string) ([]PathProvenance, error) {
		return []PathProvenance{{
			Path:      "docs/reference.md",
			Generated: true,
			Producers: []ProvenanceTarget{{
				Target:            "shell/docs-generate",
				Operation:         "go run ./cmd/bach-docs-gen",
				RegenerateCommand: "bach run shell/docs-generate",
			}},
			Status: "unknown",
		}}, nil
	}}
	var stdout bytes.Buffer

	err := runProvenance(&Project{}, deps, false, []string{"docs/reference.md"}, &stdout)
	if err != nil {
		t.Fatal(err)
	}
	got := stdout.String()
	for _, want := range []string{
		"docs/reference.md",
		"generated: true",
		"shell/docs-generate",
		"regenerate: bach run shell/docs-generate",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
}

func TestRunProvenanceJSONIsStrict(t *testing.T) {
	deps := Dependencies{Provenance: func(*Project, []string) ([]PathProvenance, error) {
		return []PathProvenance{{
			Path:      "src/app.txt",
			Source:    true,
			Consumers: []ProvenanceTarget{{Target: "shell/build", Inputs: []string{"src"}}},
			Status:    "unknown",
		}}, nil
	}}
	var stdout bytes.Buffer

	err := runProvenance(&Project{}, deps, true, []string{"src/app.txt"}, &stdout)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Paths []struct {
			Path      string `json:"path"`
			Generated bool   `json:"generated"`
			Source    bool   `json:"source"`
			Consumers []struct {
				Target string `json:"target"`
			} `json:"consumers"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, stdout.String())
	}
	if len(decoded.Paths) != 1 ||
		decoded.Paths[0].Path != "src/app.txt" ||
		!decoded.Paths[0].Source {
		t.Fatalf("decoded = %#v", decoded)
	}
}

func TestProvenanceRequiresPath(t *testing.T) {
	err := runProvenance(&Project{}, Dependencies{}, false, nil, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "requires at least one path") {
		t.Fatalf("err = %v", err)
	}
}

func TestRootHelpShowsProvenance(t *testing.T) {
	var stdout bytes.Buffer
	if err := Execute(
		context.Background(),
		[]string{"--help"},
		&stdout,
		&bytes.Buffer{},
		"test",
	); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "provenance") {
		t.Fatalf("help missing provenance:\n%s", stdout.String())
	}
}
