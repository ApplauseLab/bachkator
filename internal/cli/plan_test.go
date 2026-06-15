package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/applauselab/bachkator/internal/plan"
	"github.com/applauselab/bachkator/internal/planbatch"
	"github.com/applauselab/bachkator/internal/planexecute"
)

func TestPlanStatusCommandPrintsHumanAndJSON(t *testing.T) {
	project := &Project{Root: t.TempDir(), StatePath: ".bach/state.db"}
	deps := Dependencies{
		LoadProject: func(path string, opts LoadOptions) (*Project, error) { return project, nil },
		PlanStatus: func(ctx context.Context, project *Project, paths []string) (PlanStatusResult, error) {
			return PlanStatusResult{
				Records: []plan.StatusRecord{
					{
						Document: plan.Document{
							Path:      paths[0],
							ID:        "plan-a",
							Title:     "Plan A",
							Hash:      "sha256:1234567890",
							DependsOn: []string{"plan-zero"},
						},
						Status: plan.StatusPlanned,
					},
				},
				Waves: [][]string{{"plan-zero"}, {"plan-a"}},
			}, nil
		},
	}
	var stdout bytes.Buffer
	if err := ExecuteWithDependencies(
		context.Background(),
		[]string{"plan", "status", "plans/a.md"},
		&stdout,
		&bytes.Buffer{},
		"test",
		deps,
	); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "plan-a") ||
		!strings.Contains(stdout.String(), "Planned waves") {
		t.Fatalf("stdout = %s", stdout.String())
	}

	stdout.Reset()
	if err := ExecuteWithDependencies(
		context.Background(),
		[]string{"--json", "plan", "status", "plans/a.md"},
		&stdout,
		&bytes.Buffer{},
		"test",
		deps,
	); err != nil {
		t.Fatal(err)
	}
	var decoded planStatusJSON
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	if decoded.SchemaVersion != "bach.plan_status.v1" || decoded.Plans[0].ID != "plan-a" {
		t.Fatalf("decoded = %#v", decoded)
	}
}

func TestPlanImplementSingleFileStillUsesSinglePlanPath(t *testing.T) {
	project := &Project{Root: t.TempDir(), StatePath: ".bach/state.db"}
	called := false
	deps := Dependencies{
		LoadProject: func(path string, opts LoadOptions) (*Project, error) { return project, nil },
		PlanImplement: func(ctx context.Context, project *Project, opts PlanImplementOptions) (planexecute.Result, error) {
			called = true
			if opts.Path != "plans/a.md" {
				t.Fatalf("path = %s", opts.Path)
			}
			return planexecute.Result{
				Result: planexecute.ResultImplemented,
				Plan:   plan.Document{ID: "plan-a", Path: "plans/a.md", Hash: "sha256:abc"},
				Target: "agent/plan.plan-a",
				RunID:  "run-1",
			}, nil
		},
	}
	var stdout bytes.Buffer
	if err := ExecuteWithDependencies(
		context.Background(),
		[]string{"plan", "implement", "plans/a.md"},
		&stdout,
		&bytes.Buffer{},
		"test",
		deps,
	); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("PlanImplement was not called")
	}
	if !strings.Contains(stdout.String(), "implemented") {
		t.Fatalf("stdout = %s", stdout.String())
	}
}

func TestPlanImplementMultipleFilesUsesBatch(t *testing.T) {
	project := &Project{Root: t.TempDir(), StatePath: ".bach/state.db"}
	called := false
	deps := Dependencies{
		LoadProject: func(path string, opts LoadOptions) (*Project, error) { return project, nil },
		PlanBatch: func(ctx context.Context, project *Project, opts PlanBatchOptions) (planbatch.Result, error) {
			called = true
			if len(opts.Paths) != 2 {
				t.Fatalf("paths = %v", opts.Paths)
			}
			if opts.StopOn != planbatch.StopOnFailure {
				t.Fatalf("stop-on = %s", opts.StopOn)
			}
			return planbatch.Result{
				Plans: []planbatch.PlanResult{
					{
						Plan:  plan.Document{ID: "a", Path: "plans/a.md", Hash: "sha256:a"},
						State: planbatch.StateImplemented,
						RunID: "run-a",
					},
					{
						Plan:  plan.Document{ID: "b", Path: "plans/b.md", Hash: "sha256:b"},
						State: planbatch.StateImplemented,
						RunID: "run-b",
					},
				},
				Waves: [][]string{{"a", "b"}},
			}, nil
		},
	}
	var stdout bytes.Buffer
	if err := ExecuteWithDependencies(
		context.Background(),
		[]string{"plan", "implement", "plans/a.md", "plans/b.md", "--parallelism", "2"},
		&stdout,
		&bytes.Buffer{},
		"test",
		deps,
	); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("PlanBatch was not called")
	}
	if !strings.Contains(stdout.String(), "implemented=2") {
		t.Fatalf("stdout = %s", stdout.String())
	}
}

func TestPlanBatchJSONOutput(t *testing.T) {
	project := &Project{Root: t.TempDir(), StatePath: ".bach/state.db"}
	deps := Dependencies{
		LoadProject: func(path string, opts LoadOptions) (*Project, error) { return project, nil },
		PlanBatch: func(ctx context.Context, project *Project, opts PlanBatchOptions) (planbatch.Result, error) {
			return planbatch.Result{
				Plans: []planbatch.PlanResult{
					{
						Plan:  plan.Document{ID: "a", Path: "plans/a.md", Hash: "sha256:a"},
						State: planbatch.StateImplemented,
						RunID: "run-a",
					},
				},
				Waves:     [][]string{{"a"}},
				StartedAt: time.Now(),
				EndedAt:   time.Now(),
			}, nil
		},
	}
	var stdout bytes.Buffer
	if err := ExecuteWithDependencies(
		context.Background(),
		[]string{"--json", "plan", "implement", "plans/a.md", "plans/b.md"},
		&stdout,
		&bytes.Buffer{},
		"test",
		deps,
	); err != nil {
		t.Fatal(err)
	}
	var decoded planBatchJSON
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	if decoded.SchemaVersion != "bach.plan_batch.v1" {
		t.Fatalf("schema_version = %s", decoded.SchemaVersion)
	}
}

func TestPlanReviewCommandGroupsByState(t *testing.T) {
	project := &Project{Root: t.TempDir(), StatePath: ".bach/state.db"}
	deps := Dependencies{
		LoadProject: func(path string, opts LoadOptions) (*Project, error) { return project, nil },
		PlanReview: func(ctx context.Context, project *Project, opts PlanReviewOptions) (PlanReviewResult, error) {
			return PlanReviewResult{
				Queue: planbatch.ReviewQueue{
					Implemented: []planbatch.ReviewItem{
						{PlanID: "clean", State: planbatch.StateImplemented},
					},
					Failed: []planbatch.ReviewItem{
						{PlanID: "fail", State: planbatch.StateFailed},
					},
				},
			}, nil
		},
	}
	var stdout bytes.Buffer
	if err := ExecuteWithDependencies(
		context.Background(),
		[]string{"plan", "review", "plans/a.md"},
		&stdout,
		&bytes.Buffer{},
		"test",
		deps,
	); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "clean") || !strings.Contains(stdout.String(), "fail") {
		t.Fatalf("stdout = %s", stdout.String())
	}

	stdout.Reset()
	if err := ExecuteWithDependencies(
		context.Background(),
		[]string{"--json", "plan", "review", "plans/a.md"},
		&stdout,
		&bytes.Buffer{},
		"test",
		deps,
	); err != nil {
		t.Fatal(err)
	}
	var decoded planReviewJSON
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	if decoded.SchemaVersion != "bach.plan_review.v1" || len(decoded.Failed) != 1 {
		t.Fatalf("decoded = %#v", decoded)
	}
}

func TestPlanBatchRejectsInvalidStopOn(t *testing.T) {
	project := &Project{Root: t.TempDir(), StatePath: ".bach/state.db"}
	deps := Dependencies{
		LoadProject: func(path string, opts LoadOptions) (*Project, error) { return project, nil },
	}
	var stdout bytes.Buffer
	err := ExecuteWithDependencies(
		context.Background(),
		[]string{"plan", "implement", "plans/a.md", "plans/b.md", "--stop-on", "bad"},
		&stdout,
		&bytes.Buffer{},
		"test",
		deps,
	)
	if err == nil {
		t.Fatal("expected invalid --stop-on to fail")
	}
}
