package planbatch

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/applauselab/bachkator/internal/backend"
	"github.com/applauselab/bachkator/internal/model"
	"github.com/applauselab/bachkator/internal/plan"
	"github.com/applauselab/bachkator/internal/planexecute"
)

func TestBatchExecutesIndependentPlansInParallel(t *testing.T) {
	project := testProject(t)
	client := newFakeLedgerClient()
	client.set("ext", plan.StatusImplemented, "sha256:ext")

	writePlan(t, project.Root, "plans/a.md", "a", "Plan A", nil)
	writePlan(t, project.Root, "plans/b.md", "b", "Plan B", nil)

	svc := Service{
		Implement: func(ctx context.Context, opts planexecute.Options) (planexecute.Result, error) {
			id := planIDFromPath(opts.Path)
			return planexecute.Result{
				Result: planexecute.ResultImplemented,
				Plan:   plan.Document{ID: id, Path: opts.Path, Hash: "sha256:" + id},
				Target: "agent/plan." + id,
				RunID:  "run-" + id,
			}, nil
		},
	}

	result, err := svc.Execute(context.Background(), project, client, Options{
		Paths:       []string{"plans/a.md", "plans/b.md"},
		Parallelism: 2,
		StopOn:      StopOnFailure,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Plans) != 2 {
		t.Fatalf("expected 2 results, got %d", len(result.Plans))
	}
	for _, r := range result.Plans {
		if r.State != StateImplemented {
			t.Fatalf("plan %s state = %s", r.Plan.ID, r.State)
		}
	}
}

func TestBatchSkipsAlreadyImplementedPlans(t *testing.T) {
	project := testProject(t)
	client := newFakeLedgerClient()

	writePlan(t, project.Root, "plans/a.md", "a", "Plan A", nil)
	hash := planHash(t, project.Root, "plans/a.md")
	client.set("a", plan.StatusImplemented, hash)

	svc := Service{
		Implement: func(ctx context.Context, opts planexecute.Options) (planexecute.Result, error) {
			t.Fatal("should not execute skipped plan")
			return planexecute.Result{}, nil
		},
	}

	result, err := svc.Execute(
		context.Background(),
		project,
		client,
		Options{Paths: []string{"plans/a.md"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Plans[0].State != StateAlreadyImplemented {
		t.Fatalf("state = %s", result.Plans[0].State)
	}
}

func TestBatchBlocksWhenExternalDependencyMissing(t *testing.T) {
	project := testProject(t)
	client := newFakeLedgerClient()

	writePlan(t, project.Root, "plans/a.md", "a", "Plan A", []string{"ext"})

	svc := Service{
		Implement: func(ctx context.Context, opts planexecute.Options) (planexecute.Result, error) {
			t.Fatal("should not execute blocked plan")
			return planexecute.Result{}, nil
		},
	}

	result, err := svc.Execute(
		context.Background(),
		project,
		client,
		Options{Paths: []string{"plans/a.md"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Plans[0].State != StateBlocked {
		t.Fatalf("state = %s", result.Plans[0].State)
	}
}

func TestBatchStopsOnFailure(t *testing.T) {
	project := testProject(t)
	client := newFakeLedgerClient()

	writePlan(t, project.Root, "plans/a.md", "a", "Plan A", nil)
	writePlan(t, project.Root, "plans/b.md", "b", "Plan B", []string{"a"})

	svc := Service{
		Implement: func(ctx context.Context, opts planexecute.Options) (planexecute.Result, error) {
			id := planIDFromPath(opts.Path)
			if id == "a" {
				return planexecute.Result{
					Result: planexecute.ResultFailed,
					Plan:   plan.Document{ID: id, Path: opts.Path, Hash: "sha256:" + id},
					Target: "agent/plan." + id,
				}, errors.New("boom")
			}
			return planexecute.Result{
				Result: planexecute.ResultImplemented,
				Plan:   plan.Document{ID: id, Path: opts.Path, Hash: "sha256:" + id},
				Target: "agent/plan." + id,
				RunID:  "run-" + id,
			}, nil
		},
	}

	result, err := svc.Execute(context.Background(), project, client, Options{
		Paths:  []string{"plans/a.md", "plans/b.md"},
		StopOn: StopOnFailure,
	})
	if err != nil {
		t.Fatal(err)
	}
	byID := mapByID(result.Plans)
	if byID["a"].State != StateFailed {
		t.Fatalf("a state = %s", byID["a"].State)
	}
	if byID["b"].State != StateBlocked {
		t.Fatalf("b state = %s, want blocked (dependency a failed)", byID["b"].State)
	}
}

func TestBatchContinuesOnFailureWhenStopOnNever(t *testing.T) {
	project := testProject(t)
	client := newFakeLedgerClient()

	writePlan(t, project.Root, "plans/a.md", "a", "Plan A", nil)
	writePlan(t, project.Root, "plans/b.md", "b", "Plan B", nil)

	callOrder := []string{}
	var mu sync.Mutex
	svc := Service{
		Implement: func(ctx context.Context, opts planexecute.Options) (planexecute.Result, error) {
			id := planIDFromPath(opts.Path)
			mu.Lock()
			callOrder = append(callOrder, id)
			mu.Unlock()
			if id == "a" {
				return planexecute.Result{Result: planexecute.ResultFailed}, errors.New("boom")
			}
			return planexecute.Result{
				Result: planexecute.ResultImplemented,
				RunID:  "run-" + id,
			}, nil
		},
	}

	result, err := svc.Execute(context.Background(), project, client, Options{
		Paths:       []string{"plans/a.md", "plans/b.md"},
		Parallelism: 1,
		StopOn:      StopOnNever,
	})
	if err != nil {
		t.Fatal(err)
	}
	byID := mapByID(result.Plans)
	if byID["a"].State != StateFailed {
		t.Fatalf("a state = %s", byID["a"].State)
	}
	if byID["b"].State != StateImplemented {
		t.Fatalf("b state = %s, want implemented", byID["b"].State)
	}
}

func TestBatchSkipsDependentPlanAfterStopOnFailure(t *testing.T) {
	project := testProject(t)
	client := newFakeLedgerClient()

	writePlan(t, project.Root, "plans/a.md", "a", "Plan A", nil)
	writePlan(t, project.Root, "plans/b.md", "b", "Plan B", nil)
	writePlan(t, project.Root, "plans/c.md", "c", "Plan C", []string{"a"})

	callCount := 0
	svc := Service{
		Implement: func(ctx context.Context, opts planexecute.Options) (planexecute.Result, error) {
			callCount++
			id := planIDFromPath(opts.Path)
			if id == "a" {
				return planexecute.Result{Result: planexecute.ResultFailed}, errors.New("boom")
			}
			return planexecute.Result{
				Result: planexecute.ResultImplemented,
				RunID:  "run-" + id,
			}, nil
		},
	}

	result, err := svc.Execute(context.Background(), project, client, Options{
		Paths:       []string{"plans/a.md", "plans/b.md", "plans/c.md"},
		Parallelism: 1,
		StopOn:      StopOnFailure,
	})
	if err != nil {
		t.Fatal(err)
	}
	byID := mapByID(result.Plans)
	if callCount != 2 {
		t.Fatalf("callCount = %d, want 2 (a and b, then stop)", callCount)
	}
	if byID["a"].State != StateFailed {
		t.Fatalf("a state = %s", byID["a"].State)
	}
	if byID["b"].State != StateImplemented {
		t.Fatalf("b state = %s", byID["b"].State)
	}
	if byID["c"].State != StateBlocked {
		t.Fatalf("c state = %s, want blocked (dependency a failed)", byID["c"].State)
	}
}

func TestReviewGroupsByState(t *testing.T) {
	result := Result{
		Plans: []PlanResult{
			{Plan: plan.Document{ID: "clean"}, State: StateImplemented},
			{
				Plan:        plan.Document{ID: "warn"},
				State:       StateImplemented,
				Diagnostics: []plan.Diagnostic{{Severity: "warning"}},
			},
			{Plan: plan.Document{ID: "fail"}, State: StateFailed},
			{Plan: plan.Document{ID: "block"}, State: StateBlocked},
			{Plan: plan.Document{ID: "skip"}, State: StateSkipped},
		},
	}
	queue := Review(result)
	if len(queue.Implemented) != 1 || queue.Implemented[0].PlanID != "clean" {
		t.Fatalf("implemented = %#v", queue.Implemented)
	}
	if len(queue.NeedsReview) != 1 || queue.NeedsReview[0].PlanID != "warn" {
		t.Fatalf("needs_review = %#v", queue.NeedsReview)
	}
	if len(queue.Failed) != 1 || queue.Failed[0].PlanID != "fail" {
		t.Fatalf("failed = %#v", queue.Failed)
	}
	if len(queue.Blocked) != 1 || queue.Blocked[0].PlanID != "block" {
		t.Fatalf("blocked = %#v", queue.Blocked)
	}
	if len(queue.Skipped) != 1 || queue.Skipped[0].PlanID != "skip" {
		t.Fatalf("skipped = %#v", queue.Skipped)
	}
}

func testProject(t *testing.T) *model.RunProject {
	t.Helper()
	return &model.RunProject{Root: t.TempDir(), Targets: map[string]*model.RunTarget{}}
}

func writePlan(t *testing.T, root string, path string, id string, title string, deps []string) {
	t.Helper()
	full := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	var front string
	if len(deps) > 0 {
		front = "---\nid: " + id + "\ntitle: " + title + "\ndepends_on: [" + strings.Join(
			deps,
			", ",
		) + "]\n---\n\n"
	} else {
		front = "---\nid: " + id + "\ntitle: " + title + "\n---\n\n"
	}
	content := front + "# " + title + "\n\nBody.\n"
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func planHash(t *testing.T, root string, path string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, path))
	if err != nil {
		t.Fatal(err)
	}
	doc, _ := plan.Parse(path, data)
	return doc.Hash
}

func planIDFromPath(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, ".md")
}

func mapByID(plans []PlanResult) map[string]PlanResult {
	out := make(map[string]PlanResult, len(plans))
	for _, p := range plans {
		out[p.Plan.ID] = p
	}
	return out
}

type fakeLedgerClient struct {
	ledgers map[string]struct {
		status string
		hash   string
	}
}

func newFakeLedgerClient() *fakeLedgerClient {
	return &fakeLedgerClient{ledgers: map[string]struct {
		status string
		hash   string
	}{}}
}

func (f *fakeLedgerClient) set(id string, status string, hash string) {
	f.ledgers[id] = struct {
		status string
		hash   string
	}{status: status, hash: hash}
}

func (f *fakeLedgerClient) Get(
	ctx context.Context,
	planID string,
) (backend.PlanLedger, bool, error) {
	_ = ctx
	l, ok := f.ledgers[planID]
	if !ok {
		return backend.PlanLedger{}, false, nil
	}
	return backend.PlanLedger{
		SchemaVersion: plan.LedgerSchemaVersion,
		LedgerID:      "ledger-" + planID,
		PlanID:        planID,
		Status:        l.status,
		Hash:          l.hash,
		RecordedAt:    time.Now().UTC(),
	}, true, nil
}
