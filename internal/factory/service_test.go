package factory

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/applauselab/bachkator/internal/model"
)

func TestServiceSubmitWritesIntakeAndEnqueuesWorkItem(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	queue := &captureQueue{}
	ids := []string{"work-item", "evidence", "attempt", "event", "dedupe"}
	now := time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)
	service := Service{
		Root:  root,
		Queue: queue,
		NewID: func() (string, error) {
			id := ids[0]
			ids = ids[1:]
			return id, nil
		},
		Now: func() time.Time { return now },
	}
	result, err := service.Submit(ctx, SubmitOptions{
		Factory:           "sldc",
		Workflow:          "ship",
		Title:             "Ship billing webhook",
		Body:              "Implement it",
		Labels:            []string{"billing"},
		DedupeKey:         "billing-webhook",
		SubmittedPlanPath: "plans/billing.md",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Created || result.Item.ID != "work-item" {
		t.Fatalf("result = %#v", result)
	}
	if queue.item.BodyHash == "" || queue.item.IntakeEvidenceHash == "" {
		t.Fatalf("queued hashes missing: %#v", queue.item)
	}
	if queue.attempt.ID != "attempt" || queue.event.ID != "event" ||
		queue.dedupeEvent.ID != "dedupe" {
		t.Fatalf("queued records = %#v %#v %#v", queue.attempt, queue.event, queue.dedupeEvent)
	}
	data, err := os.ReadFile(
		filepath.Join(root, ".bach", "artifacts", "factory", "work-item", "intake.json"),
	)
	if err != nil {
		t.Fatal(err)
	}
	var intake intakeSnapshot
	if err := json.Unmarshal(data, &intake); err != nil {
		t.Fatal(err)
	}
	if intake.WorkItemID != "work-item" || intake.Factory != "sldc" || intake.Workflow != "ship" {
		t.Fatalf("intake = %#v", intake)
	}
}

func TestServiceSubmitRemovesSpeculativeIntakeWhenDeduped(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	now := time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)
	queue := &dedupeQueue{existing: WorkItem{
		ID:           "existing",
		Factory:      "sldc",
		Workflow:     "ship",
		Lifecycle:    model.LifecyclePending,
		CurrentPhase: WorkItemPhasePlan,
		Title:        "Existing",
		Priority:     model.PriorityNormal,
		SourceType:   SourceManual,
		CreatedAt:    now,
		UpdatedAt:    now,
	}}
	ids := []string{"candidate", "evidence", "attempt", "event", "dedupe"}
	service := Service{
		Root:  root,
		Queue: queue,
		NewID: func() (string, error) {
			id := ids[0]
			ids = ids[1:]
			return id, nil
		},
		Now: func() time.Time { return now },
	}
	result, err := service.Submit(ctx, SubmitOptions{
		Factory:   "sldc",
		Workflow:  "ship",
		Title:     "Duplicate",
		Body:      "body",
		DedupeKey: "same",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Created || result.Item.ID != "existing" {
		t.Fatalf("result = %#v", result)
	}
	path := filepath.Join(root, ".bach", "artifacts", "factory", "candidate", "intake.json")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("candidate intake stat err = %v, want not exist", err)
	}
}

type captureQueue struct {
	item        WorkItem
	attempt     WorkItemAttempt
	event       WorkItemEvent
	dedupeEvent WorkItemEvent
}

type dedupeQueue struct {
	existing WorkItem
}

func (q *dedupeQueue) Enqueue(
	_ context.Context,
	_ WorkItem,
	_ WorkItemAttempt,
	_ WorkItemEvent,
	_ WorkItemEvent,
) (WorkItem, bool, error) {
	return q.existing, false, nil
}

func (q *dedupeQueue) UpdatePending(
	_ context.Context,
	_ WorkItem,
	_ WorkItemEvent,
) (WorkItem, bool, error) {
	return q.existing, true, nil
}

func (q *dedupeQueue) Get(_ context.Context, _, _ string) (WorkItem, bool, error) {
	return WorkItem{}, false, nil
}

func (q *dedupeQueue) List(_ context.Context, _ WorkItemQuery) ([]WorkItem, error) {
	return nil, nil
}

func (q *dedupeQueue) Cancel(
	_ context.Context,
	_, _, _ string,
	_ time.Time,
	_ WorkItemEvent,
) (WorkItem, bool, error) {
	return WorkItem{}, false, nil
}

func (q *dedupeQueue) RecordApproval(
	_ context.Context,
	_ Approval,
	_ WorkItemEvent,
) (Approval, bool, error) {
	return Approval{}, false, nil
}

func (q *dedupeQueue) ListApprovals(_ context.Context, _ string) ([]Approval, error) {
	return nil, nil
}

func (q *captureQueue) Enqueue(
	_ context.Context,
	item WorkItem,
	attempt WorkItemAttempt,
	event WorkItemEvent,
	dedupeEvent WorkItemEvent,
) (WorkItem, bool, error) {
	q.item = item
	q.attempt = attempt
	q.event = event
	q.dedupeEvent = dedupeEvent
	item.Attempts = []WorkItemAttempt{attempt}
	item.Events = []WorkItemEvent{event}
	return item, true, nil
}

func (q *captureQueue) UpdatePending(
	_ context.Context,
	item WorkItem,
	_ WorkItemEvent,
) (WorkItem, bool, error) {
	q.item = item
	return item, true, nil
}

func (q *captureQueue) Get(_ context.Context, _, _ string) (WorkItem, bool, error) {
	return WorkItem{}, false, nil
}

func (q *captureQueue) List(_ context.Context, _ WorkItemQuery) ([]WorkItem, error) {
	return nil, nil
}

func (q *captureQueue) Cancel(
	_ context.Context,
	_, _, _ string,
	_ time.Time,
	_ WorkItemEvent,
) (WorkItem, bool, error) {
	return WorkItem{}, false, nil
}

func (q *captureQueue) RecordApproval(
	_ context.Context,
	_ Approval,
	_ WorkItemEvent,
) (Approval, bool, error) {
	return Approval{}, false, nil
}

func (q *captureQueue) ListApprovals(_ context.Context, _ string) ([]Approval, error) {
	return nil, nil
}
